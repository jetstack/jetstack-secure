package agent

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/d4l3k/messagediff"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/kylelemons/godebug/diff"
	"github.com/stretchr/testify/assert"
)

func TestGetConfiguration(t *testing.T) {
	t.Run("minimal successful configuration", func(t *testing.T) {
		got, cl, err := getConfiguration(discardLogs(t),
			Config{Server: "http://api.venafi.eu", Period: 1 * time.Hour},
			AgentCmdFlags{},
		)
		assert.NoError(t, err)
		assert.Equal(t, Config{
			Server: "http://api.venafi.eu",
			Period: 1 * time.Hour,
		}, got)
		assert.IsType(t, &client.UnauthenticatedClient{}, cl)
	})

	t.Run("period must be given", func(t *testing.T) {
		_, _, err := getConfiguration(discardLogs(t),
			Config{Server: "http://api.venafi.eu"},
			AgentCmdFlags{})
		assert.EqualError(t, err, "period must be set as a flag or in config")
	})

	t.Run("server must be given", func(t *testing.T) {
		got, _, err := getConfiguration(discardLogs(t),
			Config{Period: 1 * time.Hour},
			AgentCmdFlags{})
		assert.EqualError(t, err, `failed to parse server URL: parse "://": missing protocol scheme`)
		assert.Equal(t, Config{}, got)
	})

	t.Run("auth defaults to 'unauthenticated'", func(t *testing.T) {
		got, cl, err := getConfiguration(discardLogs(t),
			fillRequired(Config{}),
			AgentCmdFlags{})
		assert.NoError(t, err)
		assert.Equal(t, fillRequired(Config{}), got)
		assert.IsType(t, &client.UnauthenticatedClient{}, cl)
	})

	t.Run("old jetstack-secure auth", func(t *testing.T) {
		t.Run("--credential-path alone means jetstack-secure auth", func(t *testing.T) {
			// `client_id`, `client_secret`, and `auth_server_domain` are
			// usually injected at build time, but we can't do that in tests, so
			// we need to provide them in the credentials file.
			path := withFile(t, `{"user_id":"fpp2624799349@affectionate-hertz6.platform.jetstack.io","user_secret":"foo","client_id": "k3TrDbfLhCgnpAbOiiT2kIE1AbovKzjo","client_secret": "f39w_3KT9Vp0VhzcPzvh-uVbudzqCFmHER3Huj0dvHgJwVrjxsoOQPIw_1SDiCfa","auth_server_domain":"auth.jetstack.io"}`)
			got, cl, err := getConfiguration(discardLogs(t),
				fillRequired(Config{}),
				AgentCmdFlags{CredentialsPath: path})
			assert.NoError(t, err)
			assert.Equal(t, fillRequired(Config{}), got)
			assert.IsType(t, &client.OAuthClient{}, cl)
		})
		t.Run("--credential-path but file is missing", func(t *testing.T) {
			got, _, err := getConfiguration(discardLogs(t),
				fillRequired(Config{}),
				AgentCmdFlags{CredentialsPath: "credentials.json"})
			assert.EqualError(t, err, "failed to load credentials from file credentials.json: open credentials.json: no such file or directory")
			assert.Equal(t, Config{}, got)
		})
	})

	t.Run("vcp auth: private key jwt service account", func(t *testing.T) {
		// When --client-id is used, --venafi-cloud is implied.
		t.Run("--private-key-path is required when --client-id is used", func(t *testing.T) {
			got, _, err := getConfiguration(discardLogs(t),
				fillRequired(Config{}),
				AgentCmdFlags{
					ClientID:       "test-client-id",
					PrivateKeyPath: "",
				})
			assert.EqualError(t, err, "failed to create client: cannot create VenafiCloudClient: 1 error occurred:\n\t* private_key_file cannot be empty\n\n")
			assert.Equal(t, Config{}, got)
		})
		t.Run("valid --client-id and --private-key-path", func(t *testing.T) {
			path := withFile(t, "-----BEGIN PRIVATE KEY-----\nMHcCAQEEIFptpPXOvEWDrYkiMhyEH1+FB1GwtwX2tyXH4KtBO6g7oAoGCCqGSM49\nAwEHoUQDQgAE/BsIwagYc4YUjSSFyqcStj2qliAkdVGlMoJbMuXupzQ9Qs4TX5Pl\ndFjz6J/j6Gu4fLPqXmM61Hj6kiuRHx5eHQ==\n-----END PRIVATE KEY-----\n")
			got, cl, err := getConfiguration(discardLogs(t),
				fillRequired(Config{}),
				AgentCmdFlags{
					ClientID:       "5bc7d07c-45da-11ef-a878-523f1e1d7de1",
					PrivateKeyPath: path,
				})
			assert.NoError(t, err)
			assert.Equal(t, fillRequired(Config{}), got)
			assert.IsType(t, &client.VenafiCloudClient{}, cl)
		})

		// --credentials-path + --venafi-cloud can be used instead of
		// --client-id and --private-key-path. Unfortunately, --credentials-path
		// can't contain the private key material, just a path to it, so you
		// still need to have the private key file somewhere one the filesystem.
		t.Run("valid --venafi-cloud + --credential-path + private key stored to disk", func(t *testing.T) {
			privKeyPath := withFile(t, "-----BEGIN PRIVATE KEY-----\nMHcCAQEEIFptpPXOvEWDrYkiMhyEH1+FB1GwtwX2tyXH4KtBO6g7oAoGCCqGSM49\nAwEHoUQDQgAE/BsIwagYc4YUjSSFyqcStj2qliAkdVGlMoJbMuXupzQ9Qs4TX5Pl\ndFjz6J/j6Gu4fLPqXmM61Hj6kiuRHx5eHQ==\n-----END PRIVATE KEY-----\n")
			credsPath := withFile(t, fmt.Sprintf(`{"client_id": "5bc7d07c-45da-11ef-a878-523f1e1d7de1","private_key_file": "%s"}`, privKeyPath))
			got, _, err := getConfiguration(discardLogs(t),
				fillRequired(Config{}),
				AgentCmdFlags{
					CredentialsPath: credsPath,
					VenafiCloudMode: true,
				})
			assert.NoError(t, err)
			assert.Equal(t, fillRequired(Config{}), got)
		})

		t.Run("--private-key-file can be passed with --credential-path", func(t *testing.T) {
			privKeyPath := withFile(t, "-----BEGIN PRIVATE KEY-----\nMHcCAQEEIFptpPXOvEWDrYkiMhyEH1+FB1GwtwX2tyXH4KtBO6g7oAoGCCqGSM49\nAwEHoUQDQgAE/BsIwagYc4YUjSSFyqcStj2qliAkdVGlMoJbMuXupzQ9Qs4TX5Pl\ndFjz6J/j6Gu4fLPqXmM61Hj6kiuRHx5eHQ==\n-----END PRIVATE KEY-----\n")
			credsPath := withFile(t, `{"client_id": "5bc7d07c-45da-11ef-a878-523f1e1d7de1"}`)
			got, _, err := getConfiguration(discardLogs(t),
				fillRequired(Config{}),
				AgentCmdFlags{
					CredentialsPath: credsPath,
					PrivateKeyPath:  privKeyPath,
					VenafiCloudMode: true,
				})
			assert.EqualError(t, err, "failed to parse credentials file: 1 error occurred:\n\t* private_key_file cannot be empty\n\n")
			assert.Equal(t, Config{}, got)
		})

		t.Run("config.venafi-cloud", func(t *testing.T) {
			privKeyPath := withFile(t, "-----BEGIN PRIVATE KEY-----\nMHcCAQEEIFptpPXOvEWDrYkiMhyEH1+FB1GwtwX2tyXH4KtBO6g7oAoGCCqGSM49\nAwEHoUQDQgAE/BsIwagYc4YUjSSFyqcStj2qliAkdVGlMoJbMuXupzQ9Qs4TX5Pl\ndFjz6J/j6Gu4fLPqXmM61Hj6kiuRHx5eHQ==\n-----END PRIVATE KEY-----\n")
			credsPath := withFile(t, `{"client_id": "5bc7d07c-45da-11ef-a878-523f1e1d7de1"}`)
			got, _, err := getConfiguration(discardLogs(t),
				fillRequired(Config{
					VenafiCloud: &VenafiCloudConfig{
						UploaderID: "test-agent",
						UploadPath: "/testing/path",
					},
				}),
				AgentCmdFlags{
					CredentialsPath: credsPath,
					PrivateKeyPath:  privKeyPath,
					VenafiCloudMode: true,
				})
			assert.EqualError(t, err, "failed to parse credentials file: 1 error occurred:\n\t* private_key_file cannot be empty\n\n")
			assert.Equal(t, Config{}, got)
		})
	})

	t.Run("vcp auth: workload identity federation", func(t *testing.T) {
		os.Setenv("KUBECONFIG", withFile(t, fakeKubeconfig))

		t.Run("valid --venafi-connection", func(t *testing.T) {
			got, cl, err := getConfiguration(discardLogs(t),
				fillRequired(Config{}),
				AgentCmdFlags{VenConnName: "venafi-components", InstallNS: "venafi"})
			assert.NoError(t, err)
			assert.Equal(t, fillRequired(Config{}), got)
			assert.IsType(t, &client.VenConnClient{}, cl)
		})

		t.Run("namespace can't be read from disk", func(t *testing.T) {
			got, _, err := getConfiguration(discardLogs(t),
				fillRequired(Config{}),
				AgentCmdFlags{VenConnName: "venafi-components"})
			assert.EqualError(t, err, "could not guess which namespace the agent is running in: not running in cluster, please use --install-namespace to specify the namespace in which the agent is running")
			assert.Equal(t, Config{}, got)
		})

		t.Run("warning about venafi-cloud.uploader_id and venafi-cloud.upload_path being skipped", func(t *testing.T) {
			log, out := withLogs(t)
			cfg := fillRequired(Config{VenafiCloud: &VenafiCloudConfig{
				UploaderID: "test-agent",
				UploadPath: "/testing/path",
			}})
			got, _, err := getConfiguration(log,
				cfg,
				AgentCmdFlags{VenConnName: "venafi-components", InstallNS: "venafi"})
			assert.NoError(t, err)
			assert.Equal(t, cfg, got)
			assert.Contains(t, out.String(), "ignoring venafi-cloud.uploader_id")
			assert.Contains(t, out.String(), "ignoring venafi-cloud.upload_path")
		})
	})
}

// Fills in the `server` and `period` as they appear in each and every test
// case.
func fillRequired(c Config) Config {
	c.Server = "http://api.venafi.eu"
	c.Period = 1 * time.Hour
	return c
}

func TestValidConfigLoad(t *testing.T) {
	configFileContents := `
      server: "http://localhost:8080"
      period: 1h
      organization_id: "example"
      cluster_id: "example-cluster"
      data-gatherers:
      - name: d1
        kind: dummy
        config:
          always-fail: false
      input-path: "/home"
      output-path: "/nothome"
`

	loadedConfig, err := ParseConfig([]byte(configFileContents), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := Config{
		Server:         "http://localhost:8080",
		Period:         time.Hour,
		OrganizationID: "example",
		ClusterID:      "example-cluster",
		DataGatherers: []DataGatherer{
			{
				Name: "d1",
				Kind: "dummy",
				Config: &dummyConfig{
					AlwaysFail: false,
				},
			},
		},
		InputPath:  "/home",
		OutputPath: "/nothome",
	}

	if diff, equal := messagediff.PrettyDiff(expected, loadedConfig); !equal {
		t.Errorf("Diff %s", diff)
	}
}

func TestValidConfigWithEndpointLoad(t *testing.T) {
	configFileContents := `
      endpoint:
        host: example.com
        path: api/v1/data
      schedule: "* * * * *"
      organization_id: "example"
      cluster_id: "example-cluster"
      data-gatherers:
      - name: d1
        kind: dummy
        config:
          always-fail: false
`

	loadedConfig, err := ParseConfig([]byte(configFileContents), false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected := Config{
		Endpoint: Endpoint{
			Protocol: "http",
			Host:     "example.com",
			Path:     "api/v1/data",
		},
		Schedule:       "* * * * *",
		OrganizationID: "example",
		ClusterID:      "example-cluster",
		DataGatherers: []DataGatherer{
			{
				Name: "d1",
				Kind: "dummy",
				Config: &dummyConfig{
					AlwaysFail: false,
				},
			},
		},
	}

	if diff, equal := messagediff.PrettyDiff(expected, loadedConfig); !equal {
		t.Errorf("Diff %s", diff)
	}
}

func TestValidVenafiCloudConfigLoad(t *testing.T) {
	configFileContents := `
      server: "http://localhost:8080"
      period: 1h
      data-gatherers:
      - name: d1
        kind: dummy
        config:
          always-fail: false
      input-path: "/home"
      output-path: "/nothome"
      venafi-cloud: 
        uploader_id: test-agent
        upload_path: "/testing/path"
`

	loadedConfig, err := ParseConfig([]byte(configFileContents), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := Config{
		Server:         "http://localhost:8080",
		Period:         time.Hour,
		OrganizationID: "",
		ClusterID:      "",
		DataGatherers: []DataGatherer{
			{
				Name: "d1",
				Kind: "dummy",
				Config: &dummyConfig{
					AlwaysFail: false,
				},
			},
		},
		InputPath:  "/home",
		OutputPath: "/nothome",
		VenafiCloud: &VenafiCloudConfig{
			UploaderID: "test-agent",
			UploadPath: "/testing/path",
		},
	}

	if diff, equal := messagediff.PrettyDiff(expected, loadedConfig); !equal {
		t.Errorf("Diff %s", diff)
	}
}

func TestInvalidConfigError(t *testing.T) {
	configFileContents := `data-gatherers: "things"`

	_, parseError := ParseConfig([]byte(configFileContents), false)

	expectedError := fmt.Errorf("yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `things` into []agent.DataGatherer")

	if parseError.Error() != expectedError.Error() {
		t.Fatalf("got != want;\ngot=%s,\nwant=%s", parseError, expectedError)
	}
}

func TestMissingConfigError(t *testing.T) {
	t.Run("fail to parse config if organization_id or cluster_id are missing (venafi-cloud not enabled)", func(t *testing.T) {
		_, parseError := ParseConfig([]byte(""), false)

		if parseError == nil {
			t.Fatalf("expected error, got nil")
		}

		expectedErrorLines := []string{
			"2 errors occurred:",
			"\t* organization_id is required",
			"\t* cluster_id is required",
			"\n",
		}

		expectedError := strings.Join(expectedErrorLines, "\n")

		gotError := parseError.Error()

		if gotError != expectedError {
			t.Errorf("\ngot=\n%v\nwant=\n%s\ndiff=\n%s", gotError, expectedError, diff.Diff(gotError, expectedError))
		}
	})
	t.Run("successfully parse config if organization_id or cluster_id are missing (venafi-cloud is enabled)", func(t *testing.T) {
		_, parseError := ParseConfig([]byte(""), true)

		if parseError != nil {
			t.Fatalf("unxexpected error, no error should have occured when parsing configuration: %s", parseError)
		}
	})
}

func TestPartialMissingConfigError(t *testing.T) {
	_, parseError := ParseConfig([]byte(`
      endpoint:
        host: example.com
        path: /api/v1/data
      schedule: "* * * * *"
      organization_id: "example"
      cluster_id: "example-cluster"
      data-gatherers:
        - kind: dummy`), false)

	if parseError == nil {
		t.Fatalf("expected error, got nil")
	}

	expectedErrorLines := []string{
		"1 error occurred:",
		"\t* datagatherer 1/1 is missing a name",
		"\n",
	}

	expectedError := strings.Join(expectedErrorLines, "\n")

	gotError := parseError.Error()

	if gotError != expectedError {
		t.Errorf("\ngot=\n%v\nwant=\n%s\ndiff=\n%s", gotError, expectedError, diff.Diff(gotError, expectedError))
	}
}

func TestInvalidServerError(t *testing.T) {
	_, parseError := ParseConfig([]byte(`
      server: "something not a URL"
      organization_id: "my_org"
      cluster_id: "my_cluster"
      data-gatherers:
        - kind: dummy
          name: dummy`), false)

	if parseError == nil {
		t.Fatalf("expected error, got nil")
	}

	expectedErrorLines := []string{
		"1 error occurred:",
		"\t* server is not a valid URL",
		"\n",
	}

	expectedError := strings.Join(expectedErrorLines, "\n")

	gotError := parseError.Error()

	if gotError != expectedError {
		t.Errorf("\ngot=\n%v\nwant=\n%s\ndiff=\n%s", gotError, expectedError, diff.Diff(gotError, expectedError))
	}
}

func TestInvalidDataGathered(t *testing.T) {
	_, parseError := ParseConfig([]byte(`
      endpoint:
        host: example.com
        path: /api/v1/data
      schedule: "* * * * *"
      data-gatherers:
        - kind: "foo"`), false)

	if parseError == nil {
		t.Fatalf("expected error, got nil")
	}

	if got, want := parseError.Error(), `cannot parse data-gatherer configuration, kind "foo" is not supported`; got != want {
		t.Errorf("\ngot=\n%v\nwant=\n%s\ndiff=\n%s", got, want, diff.Diff(got, want))
	}
}

func withFile(t testing.TB, content string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "file")
	if err != nil {
		t.Fatalf("failed to create temporary file: %v", err)
	}
	defer f.Close()

	_, err = f.WriteString(content)
	if err != nil {
		t.Fatalf("failed to write to temporary file: %v", err)
	}

	return f.Name()
}

func withLogs(t testing.TB) (*log.Logger, *bytes.Buffer) {
	b := bytes.Buffer{}
	return log.New(&b, "", 0), &b
}

func discardLogs(t testing.TB) *log.Logger {
	return log.New(io.Discard, "", 0)
}

const fakeKubeconfig = `
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCVENDQWUyZ0F3SUJBZ0lJVGpXZTMvWXhJbXN3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TkRBM01UVXhOREUxTVRSYUZ3MHpOREEzTVRNeE5ESXdNVFJhTUJVeApFekFSQmdOVkJBTVRDbXQxWW1WeWJtVjBaWE13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLCkFvSUJBUUMweVhZSmIyT0JRb0NrYXYySWw1NjNRM0t3RFpGSmluNFRFSkJJbWt6MnpJVU56cHIvV09MY01jdjYKVG9IaTl1c1oyL005dktMcnhYRE1FcFNJaTR4c1psZ3BDN2Erb3hqNW80MVdqRy9rdzhmcVc2MTRUV2ZEekRkWQppRkNKOC9PdmpKdFY2elREZ04vUGtWRytKQWJIOTdnVkc5NXRzRHBIazN3Nk12WkdYK3lqdnhXblV1enlpdFIzCkNLNkhYcE82Y0xBVzJva1FWZHYrZEFUSDFrZVpZZHpMOFp0U0txcUo2QWlRTUtEMG1FbXZPWDNBRk4vUUNQdXkKTVdDUXVkQ1RaQ0t1a1gwRzllakd3NGE1RC9CZnVmYmtWd1g3Vmo3OGJjQ0NId3JJMFZNOHVzYnJzcEs5eGtsVwpodjRXOGVaQ21KZWlMajFLVUhSbTdRVlFYVHNoQWdNQkFBR2pXVEJYTUE0R0ExVWREd0VCL3dRRUF3SUNwREFQCkJnTlZIUk1CQWY4RUJUQURBUUgvTUIwR0ExVWREZ1FXQkJTckNJaE44czZpMmRIMEpwQWU3dFdPL2p2clJqQVYKQmdOVkhSRUVEakFNZ2dwcmRXSmxjbTVsZEdWek1BMEdDU3FHU0liM0RRRUJDd1VBQTRJQkFRQ0pQd2x1OFVhRgo5UnIvUG5QSDNtL0w2amhlcE5Kak5vNThFSWlEMWpjc1Y3R04zZUpha0h1b3g1MGRmR2gvMFFMZEwreUluamFtCkw0Y0R6RnVYeDhCL0ZXQlMwdnYvaG5WQ1JadER4bjB1OW92WC9iblNJdHpBOHNKMHA4cU1YeEFmbkxuZDI0TksKNFZXZmFXTThjbitQeUoybnJ3MHo2YmtYYnZZMGxEV2ZRakorOUJxU3IyeUZYZWM4eXljSzZ6aHlXeHJMV1p1OAoyQngrYjJML1JETDg2T3FXSkthRmljNGlWeDBoK2xDYlBIQmNwazhQOVFvSjZodThhdXdiWjZlMkwxbmZSdWFjCjB3Z1F5OEMzNVExMTdla0dOcjZKMUlrRlE5OGorYTNBTVQ2Z05KclZGZEJOOGlMcjlhMDZJQnRBb04wV2s0bysKL2F5akJBc3hONHo5Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    server: https://127.0.0.1:58453
  name: fake
contexts:
- context:
    cluster: fake
    user: fake
  name: fake
current-context: fake
kind: Config
preferences: {}
users:
- name: fake
  user:
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURLVENDQWhHZ0F3SUJBZ0lJV1JQVy9Nblo0VnN3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TkRBM01UVXhOREUxTVRSYUZ3MHlOVEEzTVRVeE5ESXdNVFZhTUR3eApIekFkQmdOVkJBb1RGbXQxWW1WaFpHMDZZMngxYzNSbGNpMWhaRzFwYm5NeEdUQVhCZ05WQkFNVEVHdDFZbVZ5CmJtVjBaWE10WVdSdGFXNHdnZ0VpTUEwR0NTcUdTSWIzRFFFQkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDcGpIRW4KY2w3QlVURlJLdTVUeU54TmxEdWxHYittalNLcHdsd2FGa0ZyYUZPMXU0MVRVOE9FalZhNDlheHp1SHZYNTZpWgpLMEJCbkJ5aFdYeGVKNE1CTzRWdXk2K09zYVBHWUgxcDZIcGpmUTBwVW5QODFndTgzMloyWmRaazhmZkJVb0pjCjI4b25Mbjd0UERVdjhHVk9WbndZRzE4RGFDWFFjVGR3VjFNYVFKZCtsNGpveHQ5S0J6aDhZUUhZanJMdnl4RncKd2dPbTNITk5GQ3J3Zno2Wis2bi95bHliaTA3amNHVi9nMTVHaVl6azJNWW5EbFBYUHVQYzY0MVp0NWdBcGFwSgpUbUdsaW95Ym85bUVtZmRFbnd0aDJDSTZTdkx6eXlveTJidlhEVktNRzhZTzE5N25kRUd6TE95T1lYT1RMYUNkCnhaWVVCdlNadkxSK1pzMGpBZ01CQUFHalZqQlVNQTRHQTFVZER3RUIvd1FFQXdJRm9EQVRCZ05WSFNVRUREQUsKQmdnckJnRUZCUWNEQWpBTUJnTlZIUk1CQWY4RUFqQUFNQjhHQTFVZEl3UVlNQmFBRktzSWlFM3l6cUxaMGZRbQprQjd1MVk3K08rdEdNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUExeXpDdE55Rmp6SHlNZ0FFTVpXalR4OWxWClk2MHRpeTFvYjUvL0thR0MvWmhSbW94NmZ0Sy94dFJDRlptRVYxZ1ZzaXNLc0g2L0YwTEZHRys4V0lrNzVoZXkKVGtoRXUvRVpBdEpRMUNoSmFWMTg4QzNvMmtmSkZOOFlVRlRyS0k3K1NNb0RCTmJJU0VPV3FsZFRiVDdWdkVzNQpsWTRKcS9rU2xnNnNZcWNCRDYzY2pFOHpKU3Y4aDUra3J0d2JVRW90Y0ptN0IvNnpMZksxNWQ5WXBEb0F1anl0CjlVcTVROEhaSGRqWlZ1OWgvNmYvbVMvZkRyek9weDhNOTdPblU1T0MvY2dTNGtUNVhkdVo3SVB3TDJVMkZsTlIKVUdvZ0RndmxDQkFaMDV4WXh4Z2xjNlNYK3JrcURUK3VhWHNtR2dBU21oUjR4OXFkRzA1R2JIdXhoZkJhCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    client-key-data: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcEFJQkFBS0NBUUVBcVl4eEozSmV3VkV4VVNydVU4amNUWlE3cFJtL3BvMGlxY0pjR2haQmEyaFR0YnVOClUxUERoSTFXdVBXc2M3aDcxK2VvbVN0QVFad2NvVmw4WGllREFUdUZic3V2anJHanhtQjlhZWg2WTMwTktWSnoKL05ZTHZOOW1kbVhXWlBIM3dWS0NYTnZLSnk1KzdUdzFML0JsVGxaOEdCdGZBMmdsMEhFM2NGZFRHa0NYZnBlSQo2TWJmU2djNGZHRUIySTZ5NzhzUmNNSURwdHh6VFJRcThIOCttZnVwLzhwY200dE80M0JsZjROZVJvbU01TmpHCkp3NVQxejdqM091TldiZVlBS1dxU1U1aHBZcU1tNlBaaEpuM1JKOExZZGdpT2tyeTg4c3FNdG03MXcxU2pCdkcKRHRmZTUzUkJzeXpzam1Gemt5MmduY1dXRkFiMG1ieTBmbWJOSXdJREFRQUJBb0lCQUY2dHkzNWdzcU0zYU5mUApwbmpwSUlTOTh6UzJGVHkzY1pUa3NUUHNHNm9UL3pMcndmYTNQdVpsV3ZrOFQ0bnJpbFM5eTN1RkdJUEszbjRICmo1aXdiY3FoWjFqQXE0OStpVnM5Qkt2QW81K3M5RTJQK3E5RkJCYjdsYWNtSlR3SGx2ZkEwSVYwUXdYd1EvYk0KZVZNRTVqMkJ0Qmh1S0hlcGovdy9UTnNTR0pqK2NlNmN2aXVVb2NXWGsxWDl2c1RDaUdtMVdnVkZGQVphVGpMTgpDcEU1dHFpdnpvbEZVbXZIbmVYNTZTOEdFWk01NFA5MFk1enJ3NHBGa0Vud1VMRlBLa1U0cUU0eWVPNVFsWUhCClQ0NklIOVNPcUU5T0pLL3JCSGVzQU45TWNrMTdKblF6Sy95bXh6eHhhcGdPMnk0bVBTcjJaaGk0SENMRHRQV2QKc0ZtRzc2RUNnWUVBeHhQTTJYVFV2bXV5ckZmUVgxblJTSW9jMGhxZFY0MnFaRFlkMzZWVWc1UUVMM0Y4S01aUwptSkNsWlJXYW9IY0NFVUdXakFTWEJaMW9hOHlOMVhSNURTV3ZJMmV5TjE1dnh3NFg1SjV5QzUvY0F4ZW00dUk3CnkzM0VWWktXZXpFQTVVeUFtNlF6ei9lR1R6QkZyNUlxYkJDUitTUldudHRXUHdJTUhkK0VoeEVDZ1lFQTJnY3QKT2h1U0xJeDZZbTFTRHVVT0pSdmtFZFlCazJPQWxRbk5kOVJoaWIxdVlVbjhPTkhYdHBsY2FHZEl3bFdkaEJlcwo4M1F4dXA4MEFydEFtM2FHMXZ6RlZ6Q05KeHA4ZGFxWlFsZk94YlJReUQ0cjdtT2Z5aENFY2VibHAxMkZKRTBQCmNhOFl2TkFuTTdkbnlTSFd0aUo2THFQWDVuMXlRSC9JY1NIaEdQTUNnWUVBa0ZDZFBzSy8rcTZ1SHR1bDFZbVIKK3FrTWpZNzNvdUd5dE9TNk1VZDBCZEtHV2pKRmxIVjRxTnFxMjZXV3ExNjZZL0lOQmNIS0RTcjM2TFduMkNhUQpIbVRFR3NGd1kwMFZjTktacFlUckhkd3NMUjIzUUdCS2dwRFFoRXc0eEdOWXgrRDJsbDJwcGNoRldDQ2hVODU4CjdFdnkxZzV1c01oR05IVHlmYkZzTEZFQ2dZRUF6QXJOVzhVenZuZFZqY25MY3Q4UXBzLzhXR2pVbnJBUFJPdWcKbTlWcDF2TXVXdVJYcElGV0JMQnYxOUZaT1czUWRTK0hEMndkb2c2ZUtUUS9HWDhLWUNhOU5JVGVoTXIzMFZMdwpEVE9KOG1KMiszK2JzNFVPcEpkaXJBb3Z3THI0QUdvUjJ3M0g4K1JGMjlOMzBMYlhieXJDOStVa0I3UTgrWG5kCkIydHljdHNDZ1lCZkxqUTNRUnpQN1Z5Y1VGNkFTYUNYVTJkcE5lckVUbGFpdldIb1FFWVo3NHEyMkFTeFcrMlEKWmtZTEM1RVNGMnZwUU5kZUZhZlRyRm9zR3pLQ1dwYXBUL2QwUC9qaG83TEF1TTJQZEcxSXFoNElRU3FUM3VqNwp4Sm9WUzhIbEg1Ri9sQzZzczZQSm1GWlpsanhFL1FVTDlucDNLYTVCRjFXdXZiZVp0Q2I5Mnc9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
`
