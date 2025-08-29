package agent

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/testutil"
)

func Test_ValidateAndCombineConfig(t *testing.T) {
	// For common things like validating `server` and `data-gatherers`, we don't
	// need to test every auth mode. We just test them using the Jetstack Secure
	// OAuth mode.
	fakeCredsPath := withFile(t, `{"user_id":"foo","user_secret":"bar","client_id": "baz","client_secret": "foobar","auth_server_domain":"bazbar"}`)

	t.Run("In Venafi Connection mode, --install-namespace must be provided if POD_NAMESPACE is not set", func(t *testing.T) {
		_, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				organization_id: foo
				cluster_id: bar
				period: 5m
			`)),
			withCmdLineFlags("--venafi-connection", "venafi-components"))
		assert.EqualError(t, err, "1 error occurred:\n\t* could not guess which namespace the agent is running in: POD_NAMESPACE env var not set, meaning that you are probably not running in cluster. Please use --install-namespace or POD_NAMESPACE to specify the namespace in which the agent is running.\n\n")
	})

	t.Run("period must be given with either --period/-p or period field in config", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		_, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				organization_id: foo
				cluster_id: bar
			`)),
			withCmdLineFlags("--credentials-file", fakeCredsPath))
		assert.EqualError(t, err, "1 error occurred:\n\t* period must be set using --period or -p, or using the 'period' field in the config file\n\n")

	})

	t.Run("period can be provided using --period or -p", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")

		given := withConfig(testutil.Undent(`
			server: https://api.venafi.eu
			organization_id: foo
			cluster_id: bar
		`))

		got, _, err := ValidateAndCombineConfig(discardLogs(), given, withCmdLineFlags("--period", "5m", "--credentials-file", fakeCredsPath))

		require.NoError(t, err)
		assert.Equal(t, 5*time.Minute, got.Period)

		got, _, err = ValidateAndCombineConfig(discardLogs(), given, withCmdLineFlags("-p", "3m", "--credentials-file", fakeCredsPath))
		require.NoError(t, err)
		assert.Equal(t, 3*time.Minute, got.Period)
	})

	t.Run("period can be provided using the period field in config file", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		got, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				period: 7m
				organization_id: foo
				cluster_id: bar
			`)),
			withCmdLineFlags("--credentials-file", fakeCredsPath))
		require.NoError(t, err)
		assert.Equal(t, 7*time.Minute, got.Period)
	})

	t.Run("--period flag takes precedence over period field in config, shows warning", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		log, gotLogs := recordLogs(t)
		got, _, err := ValidateAndCombineConfig(log,
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				period: 1111m
				organization_id: foo
				cluster_id: bar
			`)),
			withCmdLineFlags("--period", "99m", "--credentials-file", fakeCredsPath))
		require.NoError(t, err)
		assert.Equal(t, testutil.Undent(`
			INFO Output mode selected mode="Jetstack Secure OAuth" reason="--credentials-file was specified without --venafi-cloud"
			INFO Both the 'period' field and --period are set. Using the value provided with --period.
		`), gotLogs.String())
		assert.Equal(t, 99*time.Minute, got.Period)
	})

	t.Run("jetstack-secure-oauth-auth: server field is not required", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		got, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				period: 1h
				organization_id: foo
				cluster_id: bar
			`)),
			withCmdLineFlags("--credentials-file", fakeCredsPath))
		require.NoError(t, err)
		assert.Equal(t, "https://preflight.jetstack.io", got.Server)
	})

	t.Run("venafi-cloud-keypair-auth: server field is not required", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		credsPath := withFile(t, `{"client_id": "foo","private_key_file": "`+withFile(t, fakePrivKeyPEM)+`"}`)
		got, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				period: 1h
				cluster_id: bar
				venafi-cloud:
				  upload_path: /foo/bar
			`)),
			withCmdLineFlags("--venafi-cloud", "--credentials-file", credsPath))
		require.NoError(t, err)
		assert.Equal(t, "https://api.venafi.cloud", got.Server)
	})

	t.Run("server URL must be valid", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		_, _, gotErr := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: "something not a URL"
				period: 1h
				organization_id: "my_org"
				cluster_id: "my_cluster"
				data-gatherers:
				  - kind: dummy
				    name: dummy
			`)),
			withCmdLineFlags("--credentials-file", fakeCredsPath))
		assert.EqualError(t, gotErr, testutil.Undent(`
			1 error occurred:
				* server "something not a URL" is not a valid URL

		`))
	})

	t.Run("--strict is passed down", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		got, _, gotErr := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				period: 1h
				organization_id: "my_org"
				cluster_id: "my_cluster"
			`)),
			withCmdLineFlags("--strict", "--credentials-file", fakeCredsPath))
		require.NoError(t, gotErr)
		assert.Equal(t, true, got.StrictMode)
	})

	t.Run("--disable-compression is deprecated and doesn't do anything", func(t *testing.T) {
		path := withFile(t, `{"user_id":"fpp2624799349@affectionate-hertz6.platform.jetstack.io","user_secret":"foo","client_id": "k3TrDbfLhCgnpAbOiiT2kIE1AbovKzjo","client_secret": "f39w_3KT9Vp0VhzcPzvh-uVbudzqCFmHER3Huj0dvHgJwVrjxsoOQPIw_1SDiCfa","auth_server_domain":"auth.jetstack.io"}`)
		log, b := recordLogs(t)
		_, _, err := ValidateAndCombineConfig(log,
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				period: 1h
				organization_id: foo
				cluster_id: bar
				`)),
			withCmdLineFlags("--disable-compression", "--credentials-file", path, "--install-namespace", "venafi"))
		require.NoError(t, err)

		// The log line printed by pflag is not captured by the log recorder.
		assert.Equal(t, testutil.Undent(`
			INFO Output mode selected mode="Jetstack Secure OAuth" reason="--credentials-file was specified without --venafi-cloud"
			INFO Using period from config period="1h0m0s"
		`), b.String())
	})

	t.Run("error when no output mode specified", func(t *testing.T) {
		_, cl, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				period: 1h
				organization_id: foo
				cluster_id: bar
			`)),
			withoutCmdLineFlags(),
		)
		assert.EqualError(t, err, testutil.Undent(`
			no output mode specified. To enable one of the output modes, you can:
			 - Use (--venafi-cloud with --credentials-file) or (--client-id with --private-key-path) to use the Venafi Cloud Key Pair Service Account mode.
			 - Use --venafi-connection for the Venafi Cloud VenafiConnection mode.
			 - Use --credentials-file alone if you want to use the Jetstack Secure OAuth mode.
			 - Use --api-token if you want to use the Jetstack Secure API Token mode.
			 - Use --machine-hub if you want to use the MachineHub mode.
			 - Use --output-path or output-path in the config file for Local File mode.`))
		assert.Nil(t, cl)
	})

	t.Run("jetstack-secure-oauth-auth: sample config", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		// `client_id`, `client_secret`, and `auth_server_domain` are usually
		// injected at build time, but we can't do that in tests, so we need to
		// provide them in the credentials file.
		credsPath := withFile(t, `{"user_id":"fpp2624799349@affectionate-hertz6.platform.jetstack.io","user_secret":"foo","client_id": "k3TrDbfLhCgnpAbOiiT2kIE1AbovKzjo","client_secret": "f39w_3KT9Vp0VhzcPzvh-uVbudzqCFmHER3Huj0dvHgJwVrjxsoOQPIw_1SDiCfa","auth_server_domain":"auth.jetstack.io"}`)
		got, cl, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				period: 5m
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
			`)),
			withCmdLineFlags("--credentials-file", credsPath),
		)
		expect := CombinedConfig{
			OutputMode: "Jetstack Secure OAuth",
			ClusterID:  "example-cluster",
			DataGatherers: []DataGatherer{{Kind: "dummy",
				Name:   "d1",
				Config: &dummyConfig{},
			}},
			Period:         5 * time.Minute,
			Server:         "http://example.com",
			OrganizationID: "example",
			EndpointPath:   "api/v1/data",
			BackoffMaxTime: 10 * time.Minute,
			InstallNS:      "venafi",
		}
		require.NoError(t, err)
		assert.Equal(t, expect, got)
		assert.IsType(t, &client.OAuthClient{}, cl)
	})

	t.Run("venafi-cloud-keypair-auth: extended config using --venafi-cloud and --credentials-file", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		privKeyPath := withFile(t, fakePrivKeyPEM)
		credsPath := withFile(t, `{"client_id": "5bc7d07c-45da-11ef-a878-523f1e1d7de1","private_key_file": "`+privKeyPath+`"}`)
		got, cl, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: "http://localhost:8080"
				cluster_id: "the cluster name"
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
			`)),
			withCmdLineFlags("--venafi-cloud", "--credentials-file", credsPath, "--backoff-max-time", "99m"),
		)
		expect := CombinedConfig{
			Server: "http://localhost:8080",
			Period: time.Hour,
			DataGatherers: []DataGatherer{
				{Name: "d1", Kind: "dummy", Config: &dummyConfig{AlwaysFail: false}},
			},
			InputPath:      "/home",
			OutputPath:     "/nothome",
			UploadPath:     "/testing/path",
			OutputMode:     VenafiCloudKeypair,
			ClusterID:      "the cluster name",
			BackoffMaxTime: 99 * time.Minute,
			InstallNS:      "venafi",
		}
		require.NoError(t, err)
		assert.Equal(t, expect, got)
		assert.IsType(t, &client.VenafiCloudClient{}, cl)
	})

	t.Run("venafi-cloud-keypair-auth: using --client-id and --private-key-path", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		privKeyPath := withFile(t, fakePrivKeyPEM)
		got, cl, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: "http://localhost:8080"
				period: 1h
				cluster_id: "the cluster name"
				venafi-cloud:
				  upload_path: "/foo/bar"
			`)),
			withCmdLineFlags("--client-id", "5bc7d07c-45da-11ef-a878-523f1e1d7de1", "--private-key-path", privKeyPath),
		)
		require.NoError(t, err)
		assert.Equal(t, VenafiCloudKeypair, got.OutputMode)
		assert.IsType(t, &client.VenafiCloudClient{}, cl)
	})

	t.Run("jetstack-secure-oauth-auth: fail if organization_id or cluster_id is missing and --venafi-cloud not enabled", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		credsPath := withFile(t, `{"user_id":"fpp2624799349@affectionate-hertz6.platform.jetstack.io","user_secret":"foo","client_id": "k3TrDbfLhCgnpAbOiiT2kIE1AbovKzjo","client_secret": "f39w_3KT9Vp0VhzcPzvh-uVbudzqCFmHER3Huj0dvHgJwVrjxsoOQPIw_1SDiCfa","auth_server_domain":"auth.jetstack.io"}`)
		_, _, err := ValidateAndCombineConfig(discardLogs(), withConfig(""), withCmdLineFlags("--credentials-file", credsPath))
		assert.EqualError(t, err, testutil.Undent(`
			3 errors occurred:
				* organization_id is required
				* cluster_id is required
				* period must be set using --period or -p, or using the 'period' field in the config file

		`))
	})

	t.Run("venafi-cloud-keypair-auth: authenticated if --client-id set", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		path := withFile(t, fakePrivKeyPEM)
		_, cl, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				cluster_id: foo
				venafi-cloud:
				  upload_path: /foo/bar
			`)),
			withCmdLineFlags("--venafi-cloud", "--period", "1m", "--client-id", "test-client-id", "--private-key-path", path))
		require.NoError(t, err)
		assert.IsType(t, &client.VenafiCloudClient{}, cl)
	})

	t.Run("venafi-cloud-keypair-auth: valid 1: --client-id and --private-key-path", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		path := withFile(t, fakePrivKeyPEM)
		_, cl, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				cluster_id: foo
				venafi-cloud:
				  upload_path: /foo/bar
			`)),
			withCmdLineFlags("--venafi-cloud", "--period", "1m", "--private-key-path", path, "--client-id", "test-client-id"))
		require.NoError(t, err)
		assert.IsType(t, &client.VenafiCloudClient{}, cl)
	})

	t.Run("venafi-cloud-keypair-auth: valid 2: --venafi-cloud and --credentials-file", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		credsPath := withFile(t, fmt.Sprintf(`{"client_id": "foo","private_key_file": "%s"}`, withFile(t, fakePrivKeyPEM)))
		_, cl, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				cluster_id: foo
				venafi-cloud:
				  upload_path: /foo/bar
			`)),
			withCmdLineFlags("--venafi-cloud", "--credentials-file", credsPath, "--period", "1m"))
		require.NoError(t, err)
		assert.IsType(t, &client.VenafiCloudClient{}, cl)
	})

	t.Run("venafi-cloud-keypair-auth: when --venafi-cloud is used, upload_path is required", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		credsPath := withFile(t, fmt.Sprintf(`{"client_id": "foo","private_key_file": "%s"}`, withFile(t, fakePrivKeyPEM)))
		_, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: "http://localhost:8080"
				period: 1h
				venafi-cloud:
				  uploader_id: test-agent
				cluster_id: "the cluster name"
			`)),
			withCmdLineFlags("--venafi-cloud", "--credentials-file", credsPath))
		require.EqualError(t, err, "1 error occurred:\n\t* the venafi-cloud.upload_path field is required when using the Venafi Cloud Key Pair Service Account mode\n\n")
	})

	t.Run("jetstack-secure-oauth-auth: --credential-file alone means jetstack-secure oauth auth", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		// `client_id`, `client_secret`, and `auth_server_domain` are usually
		// injected at build time, but we can't do that in tests, so we need to
		// provide them in the credentials file.
		path := withFile(t, `{"user_id":"fpp2624799349@affectionate-hertz6.platform.jetstack.io","user_secret":"foo","client_id": "k3TrDbfLhCgnpAbOiiT2kIE1AbovKzjo","client_secret": "f39w_3KT9Vp0VhzcPzvh-uVbudzqCFmHER3Huj0dvHgJwVrjxsoOQPIw_1SDiCfa","auth_server_domain":"auth.jetstack.io"}`)
		got, cl, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				period: 1h
				organization_id: foo
				cluster_id: bar
				`)),
			withCmdLineFlags("--credentials-file", path))
		require.NoError(t, err)
		assert.Equal(t, CombinedConfig{Server: "https://api.venafi.eu", Period: time.Hour, OrganizationID: "foo", ClusterID: "bar", OutputMode: JetstackSecureOAuth, BackoffMaxTime: 10 * time.Minute, InstallNS: "venafi"}, got)
		assert.IsType(t, &client.OAuthClient{}, cl)
	})

	t.Run("jetstack-secure-oauth-auth: --credential-file used but file is missing", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		got, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				period: 1h
				organization_id: foo
				cluster_id: bar
			`)),
			withCmdLineFlags("--credentials-file", "credentials.json"))
		assert.EqualError(t, err, testutil.Undent(`
			validating creds: failed loading config using the Jetstack Secure OAuth mode: 1 error occurred:
				* credentials file: failed to load credentials from file credentials.json: open credentials.json: no such file or directory

		`))
		assert.Equal(t, CombinedConfig{}, got)
	})

	t.Run("jetstack-secure-oauth-auth: shows helpful err messages", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		credsPath := withFile(t, `{"user_id":""}`)
		_, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				period: 1h
				organization_id: foo
				cluster_id: bar
			`)),
			withCmdLineFlags("--credentials-file", credsPath))
		assert.EqualError(t, err, testutil.Undent(`
			validating creds: failed loading config using the Jetstack Secure OAuth mode: 2 errors occurred:
				* credentials file: user_id cannot be empty
				* credentials file: user_secret cannot be empty

			`))
	})

	t.Run("venafi-cloud-keypair-auth: --client-id cannot be used alone, it needs --private-key-path", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		got, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				period: 1h
			`)),
			withCmdLineFlags("--client-id", "test-client-id"))
		assert.EqualError(t, err, "if --client-id is specified, --private-key-path must also be specified")
		assert.Equal(t, CombinedConfig{}, got)
	})

	t.Run("venafi-cloud-keypair-auth: --private-key-path cannot be used alone, it needs --client-id", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		got, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				period: 1h
			`)),
			withCmdLineFlags("--private-key-path", "foo"))
		assert.EqualError(t, err, "--private-key-path is specified, --client-id must also be specified")
		assert.Equal(t, CombinedConfig{}, got)
	})

	// When --client-id is used, --venafi-cloud is implied.
	t.Run("venafi-cloud-keypair-auth: valid --client-id and --private-key-path", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		path := withFile(t, fakePrivKeyPEM)
		got, cl, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				period: 1h
				cluster_id: the cluster name
				venafi-cloud:
				  upload_path: /foo/bar
			`)),
			withCmdLineFlags("--client-id", "5bc7d07c-45da-11ef-a878-523f1e1d7de1", "--private-key-path", path))
		require.NoError(t, err)
		assert.Equal(t, CombinedConfig{Server: "https://api.venafi.eu", Period: time.Hour, OutputMode: VenafiCloudKeypair, ClusterID: "the cluster name", UploadPath: "/foo/bar", BackoffMaxTime: 10 * time.Minute, InstallNS: "venafi"}, got)
		assert.IsType(t, &client.VenafiCloudClient{}, cl)
	})

	// --credentials-file + --venafi-cloud can be used instead of
	// --client-id and --private-key-path. Unfortunately, --credentials-file
	// can't contain the private key material, just a path to it, so you
	// still need to have the private key file somewhere one the filesystem.
	t.Run("venafi-cloud-keypair-auth: valid --venafi-cloud + --credential-file + private key stored to disk", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		privKeyPath := withFile(t, fakePrivKeyPEM)
		credsPath := withFile(t, fmt.Sprintf(`{"client_id": "5bc7d07c-45da-11ef-a878-523f1e1d7de1","private_key_file": "%s"}`, privKeyPath))
		got, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				period: 1h
				cluster_id: the cluster name
				venafi-cloud:
				  upload_path: /foo/bar
			`)),
			withCmdLineFlags("--venafi-cloud", "--credentials-file", credsPath))
		require.NoError(t, err)
		assert.Equal(t, CombinedConfig{Server: "https://api.venafi.eu", Period: time.Hour, OutputMode: VenafiCloudKeypair, ClusterID: "the cluster name", UploadPath: "/foo/bar", BackoffMaxTime: 10 * time.Minute, InstallNS: "venafi"}, got)
	})

	t.Run("venafi-cloud-keypair-auth: venafi-cloud.upload_path field is required", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		privKeyPath := withFile(t, fakePrivKeyPEM)
		credsPath := withFile(t, fmt.Sprintf(`{"client_id": "5bc7d07c-45da-11ef-a878-523f1e1d7de1","private_key_file": "%s"}`, privKeyPath))
		_, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				period: 1h
				cluster_id: the cluster name
				venafi-cloud:
				  upload_path: ""        # <-- Cannot be left empty
			`)),
			withCmdLineFlags("--venafi-cloud", "--credentials-file", credsPath))
		require.EqualError(t, err, testutil.Undent(`
			1 error occurred:
				* the venafi-cloud.upload_path field is required when using the Venafi Cloud Key Pair Service Account mode

		`))
	})

	t.Run("venafi-cloud-keypair-auth: --private-key-file can be passed with --credential-file", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		privKeyPath := withFile(t, fakePrivKeyPEM)
		credsPath := withFile(t, `{"client_id": "5bc7d07c-45da-11ef-a878-523f1e1d7de1"}`)
		got, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				period: 1h
				cluster_id: the cluster name
			`)),
			withCmdLineFlags("--venafi-cloud", "--credentials-file", credsPath, "--private-key-path", privKeyPath))
		require.EqualError(t, err, testutil.Undent(`
			1 error occurred:
				* the venafi-cloud.upload_path field is required when using the Venafi Cloud Key Pair Service Account mode

		`))
		assert.Equal(t, CombinedConfig{}, got)
	})

	t.Run("venafi-cloud-keypair-auth: config.venafi-cloud", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		privKeyPath := withFile(t, fakePrivKeyPEM)
		credsPath := withFile(t, `{"client_id": "5bc7d07c-45da-11ef-a878-523f1e1d7de1"}`)
		got, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
					server: https://api.venafi.eu
					period: 1h
					venafi-cloud:
					  uploader_id: test-agent
					  upload_path: /testing/path
			`)),
			withCmdLineFlags("--venafi-cloud", "--credentials-file", credsPath, "--private-key-path", privKeyPath))
		require.EqualError(t, err, testutil.Undent(`
			1 error occurred:
				* cluster_id is required in Venafi Cloud Key Pair Service Account mode

		`))
		assert.Equal(t, CombinedConfig{}, got)
	})

	t.Run("venafi-cloud-workload-identity-auth: valid --venafi-connection", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		t.Setenv("KUBECONFIG", withFile(t, fakeKubeconfig))
		got, cl, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: http://should-be-ignored
				period: 1h
				cluster_id: the cluster name
			`)),
			withCmdLineFlags("--venafi-connection", "venafi-components"))
		require.NoError(t, err)
		assert.Equal(t, CombinedConfig{
			Period:         1 * time.Hour,
			ClusterID:      "the cluster name",
			OutputMode:     VenafiCloudVenafiConnection,
			VenConnName:    "venafi-components",
			VenConnNS:      "venafi",
			InstallNS:      "venafi",
			BackoffMaxTime: 10 * time.Minute,
		}, got)
		assert.IsType(t, &client.VenConnClient{}, cl)
	})

	t.Run("venafi-cloud-workload-identity-auth: warning about server, venafi-cloud.uploader_id, and venafi-cloud.upload_path being skipped", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		t.Setenv("KUBECONFIG", withFile(t, fakeKubeconfig))
		log, gotLogs := recordLogs(t)
		got, gotCl, err := ValidateAndCombineConfig(log,
			withConfig(testutil.Undent(`
				server: https://api.venafi.eu
				period: 1h
				cluster_id: id
				venafi-cloud:
				  uploader_id: id
				  upload_path: /path
			`)),
			withCmdLineFlags("--venafi-connection", "venafi-components"),
		)
		require.NoError(t, err)
		assert.Equal(t, testutil.Undent(`
			INFO Output mode selected venConnName="venafi-components" mode="Venafi Cloud VenafiConnection" reason="--venafi-connection was specified"
			INFO ignoring the server field specified in the config file. In Venafi Cloud VenafiConnection mode, this field is not needed.
			INFO ignoring the venafi-cloud.upload_path field in the config file. In Venafi Cloud VenafiConnection mode, this field is not needed.
			INFO ignoring the venafi-cloud.uploader_id field in the config file. This field is not needed in Venafi Cloud VenafiConnection mode.
			INFO Using period from config period="1h0m0s"
		`), gotLogs.String())
		assert.Equal(t, VenafiCloudVenafiConnection, got.OutputMode)
		assert.IsType(t, &client.VenConnClient{}, gotCl)
	})

	t.Run("venafi-cloud-workload-identity-auth: server field can be left empty in venconn mode", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		t.Setenv("KUBECONFIG", withFile(t, fakeKubeconfig))
		got, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: ""
				period: 1h
				cluster_id: foo
			`)),
			withCmdLineFlags("--venafi-connection", "venafi-components"))
		require.NoError(t, err)
		assert.Equal(t, VenafiCloudVenafiConnection, got.OutputMode)
	})

	t.Run("--machine-hub selects MachineHub mode", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		t.Setenv("KUBECONFIG", withFile(t, fakeKubeconfig))
		t.Setenv("ARK_SUBDOMAIN", "tlspk")
		t.Setenv("ARK_USERNAME", "first_last@cyberark.cloud.123456")
		t.Setenv("ARK_SECRET", "test-secret")
		got, cl, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(""),
			withCmdLineFlags("--period", "1m", "--machine-hub"))
		require.NoError(t, err)
		assert.Equal(t, MachineHub, got.OutputMode)
		assert.IsType(t, &client.CyberArkClient{}, cl)
	})

	t.Run("--machine-hub without required environment variables", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")
		t.Setenv("KUBECONFIG", withFile(t, fakeKubeconfig))
		t.Setenv("ARK_SUBDOMAIN", "")
		t.Setenv("ARK_USERNAME", "")
		t.Setenv("ARK_SECRET", "")
		got, cl, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(""),
			withCmdLineFlags("--period", "1m", "--machine-hub"))
		assert.Equal(t, CombinedConfig{}, got)
		assert.Nil(t, cl)
		assert.EqualError(t, err, testutil.Undent(`
			validating creds: failed loading config using the MachineHub mode: 1 error occurred:
				* missing environment variables: ARK_SUBDOMAIN, ARK_USERNAME, ARK_SECRET

	   `))
	})

	t.Run("argument: --output-file selects local file mode", func(t *testing.T) {
		log, gotLog := recordLogs(t)
		got, outputClient, err := ValidateAndCombineConfig(log,
			withConfig(""),
			withCmdLineFlags("--period", "1m", "--output-path", "/foo/bar/baz"))
		require.NoError(t, err)
		assert.Equal(t, LocalFile, got.OutputMode)
		assert.Equal(t, testutil.Undent(`
			INFO Output mode selected mode="Local File" reason="--output-path was specified"
		`), gotLog.String())
		assert.IsType(t, &client.FileClient{}, outputClient)
	})

	t.Run("config: output-path selects local file mode", func(t *testing.T) {
		log, gotLog := recordLogs(t)
		got, outputClient, err := ValidateAndCombineConfig(log,
			withConfig(testutil.Undent(`
				output-path: /foo/bar/baz
			`)),
			withCmdLineFlags("--period=1h"))
		require.NoError(t, err)
		assert.Equal(t, LocalFile, got.OutputMode)
		assert.Equal(t, testutil.Undent(`
			INFO Output mode selected mode="Local File" reason="output-path was specified in the config file"
		`), gotLog.String())
		assert.IsType(t, &client.FileClient{}, outputClient)
	})

	// When --input-path is supplied, the data is being read from a local file
	// and the agent is probably running outside the cluster and has no access
	// to a cluster, so the environment variables which are required for
	// generating events attached to the Agent pod should not be required:
	// POD_NAME, POD_NAMESPACE, POD_UID, KUBECONFIG, etc.
	// This test deliberately does not set those environment variables.
	//
	// TODO(wallrj): Some other config settings like cluster_id, organization_id
	// should also not be required in this situation. We'll fix those in the
	// future.
	t.Run("--input-path requires no Kubernetes config", func(t *testing.T) {
		expectedInputPath := "/foo/bar/baz"
		got, _, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				cluster_id: should-not-be-required
				organization_id: should-not-be-required
			`)),
			withCmdLineFlags(
				"--one-shot",
				"--input-path", expectedInputPath,
				"--output-path", "/dev/null",
			),
		)
		require.NoError(t, err)
		assert.Equal(t, expectedInputPath, got.InputPath)
	})
}

func Test_ValidateAndCombineConfig_VenafiCloudKeyPair(t *testing.T) {
	t.Run("server, uploader_id, and cluster name are correctly passed", func(t *testing.T) {
		t.Setenv("POD_NAMESPACE", "venafi")

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		log := ktesting.NewLogger(t, ktesting.NewConfig(ktesting.Verbosity(10)))
		ctx = klog.NewContext(ctx, log)

		srv, cert, setVenafiCloudAssert := testutil.FakeVenafiCloud(t)
		setVenafiCloudAssert(func(t testing.TB, gotReq *http.Request) {
			// Only care about /v1/tlspk/upload/clusterdata/:uploader_id?name=
			if gotReq.URL.Path == "/v1/oauth/token/serviceaccount" {
				return
			}

			assert.Equal(t, srv.URL, "https://"+gotReq.Host)
			assert.Equal(t, "test cluster name", gotReq.URL.Query().Get("name"))
			assert.Equal(t, "/v1/tlspk/upload/clusterdata/no", gotReq.URL.Path)
		})

		privKeyPath := withFile(t, fakePrivKeyPEM)
		got, cl, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: `+srv.URL+`
				period: 1h
				cluster_id: "test cluster name"
				venafi-cloud:
				  uploader_id: no
				  upload_path: /v1/tlspk/upload/clusterdata
			`)),
			withCmdLineFlags("--client-id", "5bc7d07c-45da-11ef-a878-523f1e1d7de1", "--private-key-path", privKeyPath),
		)
		require.NoError(t, err)
		testutil.TrustCA(t, cl, cert)
		assert.Equal(t, VenafiCloudKeypair, got.OutputMode)

		err = cl.PostDataReadingsWithOptions(ctx, nil, client.Options{ClusterName: "test cluster name"})
		require.NoError(t, err)
	})
}

// Slower test cases due to envtest. That's why they are separated from the
// other tests.
func Test_ValidateAndCombineConfig_VenafiConnection(t *testing.T) {
	_, cfg, kcl := testutil.WithEnvtest(t)
	t.Setenv("KUBECONFIG", testutil.WithKubeconfig(t, cfg))
	srv, cert, setVenafiCloudAssert := testutil.FakeVenafiCloud(t)
	for _, obj := range testutil.Parse(
		testutil.VenConnRBAC + testutil.Undent(`
			---
			apiVersion: jetstack.io/v1alpha1
			kind: VenafiConnection
			metadata:
			  name: venafi-components
			  namespace: venafi
			spec:
			  vcp:
			    url: "`+srv.URL+`"
			    accessToken:
			      - secret:
			          name: accesstoken
			          fields: [accesstoken]
			---
			apiVersion: v1
			kind: Secret
			metadata:
			  name: accesstoken
			  namespace: venafi
			stringData:
			  accesstoken: VALID_ACCESS_TOKEN
			---
			apiVersion: rbac.authorization.k8s.io/v1
			kind: Role
			metadata:
			  name: venafi-connection-accesstoken-reader
			  namespace: venafi
			rules:
			- apiGroups: [""]
			  resources: ["secrets"]
			  verbs: ["get"]
			  resourceNames: ["accesstoken"]
			---
			apiVersion: rbac.authorization.k8s.io/v1
			kind: RoleBinding
			metadata:
			  name: venafi-connection-accesstoken-reader
			  namespace: venafi
			roleRef:
			  apiGroup: rbac.authorization.k8s.io
			  kind: Role
			  name: venafi-connection-accesstoken-reader
			subjects:
			- kind: ServiceAccount
			  name: venafi-connection
			  namespace: venafi
		`)) {
		require.NoError(t, kcl.Create(t.Context(), obj))
	}

	t.Run("err when cluster_id field is empty", func(t *testing.T) {
		expected := srv.URL
		setVenafiCloudAssert(func(t testing.TB, gotReq *http.Request) {
			assert.Equal(t, expected, "https://"+gotReq.Host)
		})

		_, _, err := ValidateAndCombineConfig(discardLogs(),
			Config{Server: "http://should-be-ignored", Period: 1 * time.Hour},
			AgentCmdFlags{VenConnName: "venafi-components", InstallNS: "venafi"})
		assert.EqualError(t, err, "1 error occurred:\n\t* cluster_id is required in Venafi Cloud VenafiConnection mode\n\n")
	})

	t.Run("the server field is ignored when VenafiConnection is used", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		log := ktesting.NewLogger(t, ktesting.NewConfig(ktesting.Verbosity(10)))
		ctx = klog.NewContext(ctx, log)

		expected := srv.URL
		setVenafiCloudAssert(func(t testing.TB, gotReq *http.Request) {
			assert.Equal(t, expected, "https://"+gotReq.Host)
		})

		cfg, cl, err := ValidateAndCombineConfig(discardLogs(),
			withConfig(testutil.Undent(`
				server: http://this-url-should-be-ignored
				period: 1h
				cluster_id: test cluster name
			`)),
			withCmdLineFlags("--venafi-connection", "venafi-components", "--install-namespace", "venafi"))
		require.NoError(t, err)

		testutil.VenConnStartWatching(ctx, t, cl)
		testutil.TrustCA(t, cl, cert)

		// TODO(mael): the client should keep track of the cluster ID, we
		// shouldn't need to pass it as an option to
		// PostDataReadingsWithOptions.
		err = cl.PostDataReadingsWithOptions(ctx, nil, client.Options{ClusterName: cfg.ClusterID})
		require.NoError(t, err)
	})
}

func Test_ParseConfig(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		cfg, err := ParseConfig([]byte(testutil.Undent(`
			server: https://api.venafi.eu
			period: 1h
			organization_id: foo
			cluster_id: bar
		`)))
		require.NoError(t, err)
		assert.Equal(t,
			Config{Server: "https://api.venafi.eu", Period: 1 * time.Hour, OrganizationID: "foo", ClusterID: "bar"},
			cfg)
	})

	t.Run("unknown data gatherer kind", func(t *testing.T) {
		_, err := ParseConfig([]byte(testutil.Undent(`
			endpoint:
			  host: example.com
			  path: /api/v1/data
			schedule: "* * * * *"
			data-gatherers:
			  - kind: "foo"
		`)))
		assert.EqualError(t, err, `cannot parse data-gatherer configuration, kind "foo" is not supported`)
	})

	t.Run("validates incorrect schema", func(t *testing.T) {
		_, gotErr := ParseConfig([]byte(`data-gatherers: "things"`))
		assert.EqualError(t, gotErr, "yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `things` into []agent.DataGatherer")
	})

	t.Run("does not show an error when user provides an unknown field", func(t *testing.T) {
		_, gotErr := ParseConfig([]byte(`some-unknown-field: foo`))
		assert.NoError(t, gotErr)
	})

	// The only validation that ParseConfig does it to check if the `kind` is
	// known. The rest of the validation is done in ValidateDataGatherers and
	// ValidateAndCombineConfig.
	t.Run("validates that the kind is known", func(t *testing.T) {
		_, gotErr := ParseConfig([]byte(testutil.Undent(`
			data-gatherers:
			- kind: unknown`,
		)))
		assert.EqualError(t, gotErr, `cannot parse data-gatherer configuration, kind "unknown" is not supported`)
	})

	// ParseConfig only checks the data-gatherer kind. The rest of the
	// validation is done in ValidateDataGatherers and ValidateAndCombineConfig.
	t.Run("does not check for missing name", func(t *testing.T) {
		_, gotErr := ParseConfig([]byte(testutil.Undent(`
			endpoint:
			  host: example.com
			  path: /api/v1/data
			schedule: "* * * * *"
			organization_id: "example"
			cluster_id: "example-cluster"
			data-gatherers:
			  - kind: dummy
		`)))
		assert.NoError(t, gotErr)
	})
	t.Run("does not check correct server URL", func(t *testing.T) {
		_, gotErr := ParseConfig([]byte(testutil.Undent(`
			server: https://api.venafi.eu
		`)))
		assert.NoError(t, gotErr)
	})
}

func Test_ValidateDataGatherers(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		err := ValidateDataGatherers(withConfig(testutil.Undent(`
			data-gatherers:
			- kind: "k8s"
			  name: "k8s/secrets"
			- kind: "k8s-discovery"
			  name: "k8s-discovery"
			- kind: "k8s-dynamic"
			  name: "k8s/secrets"
			- kind: "local"
			  name: "local"
			- kind: "dummy"
			  name: "dummy"
		`)).DataGatherers)
		require.NoError(t, err)
	})

	t.Run("missing name", func(t *testing.T) {
		gotErr := ValidateDataGatherers(withConfig(testutil.Undent(`
			data-gatherers:
			  - kind: dummy
		`)).DataGatherers)
		assert.EqualError(t, gotErr, "1 error occurred:\n\t* datagatherer 1/1 is missing a name\n\n")
	})

	// For context, the custom UnmarshalYAML in ParseConfig already validates
	// the kind. That's why ValidateDataGatherers panics: because it would be a
	// programmer mistake.
	t.Run("missing kind triggers a panic", func(t *testing.T) {
		assert.PanicsWithError(t, `cannot parse data-gatherer configuration, kind "unknown" is not supported`, func() {
			_ = ValidateDataGatherers(withConfig(testutil.Undent(`
				data-gatherers:
				- kind: unknown
			`)).DataGatherers)
		})
	})
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

func recordLogs(t *testing.T) (logr.Logger, ktesting.Buffer) {
	log := ktesting.NewLogger(t, ktesting.NewConfig(ktesting.BufferLogs(true)))
	testingLogger, ok := log.GetSink().(ktesting.Underlier)
	require.True(t, ok)
	return log, testingLogger.GetBuffer()
}

func discardLogs() logr.Logger {
	return logr.Discard()
}

// Shortcut for ParseConfig.
func withConfig(s string) Config {
	cfg, err := ParseConfig([]byte(s))
	if err != nil {
		panic(err)
	}
	return cfg
}

func withCmdLineFlags(flags ...string) AgentCmdFlags {
	parsed := withoutCmdLineFlags()
	agentCmd := &cobra.Command{}
	InitAgentCmdFlags(agentCmd, &parsed)
	err := agentCmd.ParseFlags(flags)
	if err != nil {
		panic(err)
	}

	return parsed
}

func withoutCmdLineFlags() AgentCmdFlags {
	return AgentCmdFlags{}
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

const fakePrivKeyPEM = `-----BEGIN PRIVATE KEY-----
MHcCAQEEIFptpPXOvEWDrYkiMhyEH1+FB1GwtwX2tyXH4KtBO6g7oAoGCCqGSM49
AwEHoUQDQgAE/BsIwagYc4YUjSSFyqcStj2qliAkdVGlMoJbMuXupzQ9Qs4TX5Pl
dFjz6J/j6Gu4fLPqXmM61Hj6kiuRHx5eHQ==
-----END PRIVATE KEY-----
`
