package servicediscovery

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	_ "k8s.io/klog/v2/ktesting/init"
)

func Test_hostLeadingLabelMatchesSubdomain(t *testing.T) {
	tests := map[string]struct {
		host, subdomain string
		want            bool
	}{
		"dot shape, matches":          {"eh1c6a8z1wf8hi.inventory.integration-cyberark.cloud", "eh1c6a8z1wf8hi", true},
		"dot shape, different tenant": {"eh1c6a8z1wf8hi.inventory.integration-cyberark.cloud", "someone-else", false},
		"hyphen shape, matches":       {"disco4asaf-discoverycontext.integration-cyberark.cloud", "disco4asaf", true},
		"hyphen shape, different tenant": {
			"disco4asaf-discoverycontext.integration-cyberark.cloud", "someone-else", false,
		},
		"empty subdomain never flags anything": {"anything.at.all", "", true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, hostLeadingLabelMatchesSubdomain(tt.host, tt.subdomain))
		})
	}
}

func Test_hostOnAllowedRootDomain(t *testing.T) {
	tests := map[string]struct {
		host string
		want bool
	}{
		"exact match":              {"cyberark.cloud", true},
		"subdomain":                {"id.cyberark.cloud", true},
		"uppercase":                {"ID.CyberArk.Cloud", true},
		"trailing dot":             {"id.cyberark.cloud.", true},
		"unrelated domain":         {"attacker.example", false},
		"looks like a suffix only": {"notcyberark.cloud", false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, hostOnAllowedRootDomain(tt.host))
		})
	}
}

// failOnDial is an http.RoundTripper that fails the test if it is ever used.
// It pins the property the base-URL guard exists to provide: a disallowed
// ARK_DISCOVERY_API must be rejected without a request being issued.
type failOnDial struct{ t *testing.T }

func (f failOnDial) RoundTrip(req *http.Request) (*http.Response, error) {
	f.t.Errorf("no request should be made for a disallowed base URL, got one to %q", req.URL.Redacted())
	return nil, fmt.Errorf("unexpected request")
}

func Test_RejectsDisallowedBaseURL(t *testing.T) {
	tests := map[string]string{
		"plain HTTP":            "http://platform-discovery.cyberark.cloud/",
		"disallowed domain":     "https://attacker.example/",
		"host with no scheme":   "platform-discovery.cyberark.cloud",
		"loopback when not set": "https://127.0.0.1:1234/",
	}
	for name, baseURL := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("ARK_DISCOVERY_API", baseURL)

			// Rejected at construction, so no client is built and no
			// request is ever attempted.
			client, err := New(&http.Client{Transport: failOnDial{t}}, MockDiscoverySubdomain)
			require.Error(t, err)
			require.ErrorContains(t, err, "refusing to bootstrap trust")
			assert.Nil(t, client)
		})
	}
}

// Test_DiscoverServices_RevalidatesBaseURL covers the defence-in-depth repeat
// of the check inside DiscoverServices, for a Client not built via New().
func Test_DiscoverServices_RevalidatesBaseURL(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
	ctx := klog.NewContext(t.Context(), logger)

	client := &Client{
		client:    &http.Client{Transport: failOnDial{t}},
		baseURL:   "http://platform-discovery.cyberark.cloud/",
		subdomain: MockDiscoverySubdomain,
	}

	services, _, err := client.DiscoverServices(ctx)
	require.Error(t, err)
	require.ErrorContains(t, err, "refusing to bootstrap trust")
	assert.Nil(t, services)
}

func Test_DiscoverIdentityAPIURL(t *testing.T) {
	tests := map[string]struct {
		subdomain     string
		expectedURL   string
		expectedError error
	}{
		"successful request": {
			subdomain:     MockDiscoverySubdomain,
			expectedURL:   "https://ajp5871.id.integration-cyberark.cloud",
			expectedError: nil,
		},
		"subdomain not found": {
			subdomain:     "something-random",
			expectedURL:   "",
			expectedError: fmt.Errorf("got an HTTP 404 response from service discovery; maybe the subdomain %q is incorrect or does not exist?", "something-random"),
		},
		"no identity service in response": {
			subdomain:     "no-identity",
			expectedURL:   "",
			expectedError: fmt.Errorf("didn't find %s in service discovery response, which may indicate a suspended tenant; unable to detect CyberArk Identity API URL", IdentityServiceName),
		},
		"unexpected HTTP response": {
			subdomain:     "bad-request",
			expectedURL:   "",
			expectedError: fmt.Errorf("got unexpected status code 400 Bad Request from request to service discovery API"),
		},
		"response JSON too long": {
			subdomain:     "json-too-long",
			expectedURL:   "",
			expectedError: fmt.Errorf("rejecting JSON response from server as it was too large or was truncated"),
		},
		"response JSON invalid": {
			subdomain:     "json-invalid",
			expectedURL:   "",
			expectedError: fmt.Errorf("failed to parse JSON from otherwise successful request to service discovery endpoint: invalid character 'a' looking for beginning of value"),
		},
	}

	t.Run("identity API host outside the allowed CyberArk domains is rejected", func(t *testing.T) {
		logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
		ctx := klog.NewContext(t.Context(), logger)

		httpClient := MockDiscoveryServer(t, Services{
			Identity: ServiceEndpoint{
				API: "https://ajp5871.id.attacker.example",
			},
			DiscoveryContext: ServiceEndpoint{
				API: mockDiscoveryContextAPIURL,
			},
			SecretsManager: ServiceEndpoint{
				API: mockSecretsManagerAPIURL,
			},
		})

		client, err := New(httpClient, MockDiscoverySubdomain)
		require.NoError(t, err)
		services, _, err := client.DiscoverServices(ctx)
		require.Error(t, err)
		assert.Nil(t, services)
		// The error must say the host was rejected, not the unrelated
		// "suspended tenant" message reserved for a genuinely absent
		// identity_administration entry (see the no-identity-in-response
		// case in the tests map above, which still gets that message).
		assert.Contains(t, err.Error(), "not on an allowed CyberArk domain")
	})

	t.Run("secrets_manager and discovery_context hosts outside the allowed CyberArk domains are dropped, not fatal", func(t *testing.T) {
		logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
		ctx := klog.NewContext(t.Context(), logger)

		httpClient := MockDiscoveryServer(t, Services{
			Identity: ServiceEndpoint{
				API: mockIdentityAPIURL,
			},
			DiscoveryContext: ServiceEndpoint{
				API: "https://venafi-test.inventory.attacker.example",
			},
			SecretsManager: ServiceEndpoint{
				API: "https://venafi-test.secretsmgr.attacker.example",
			},
		})

		client, err := New(httpClient, MockDiscoverySubdomain)
		require.NoError(t, err)
		services, _, err := client.DiscoverServices(ctx)
		require.NoError(t, err)
		assert.Equal(t, mockIdentityAPIURL, services.Identity.API)
		assert.Equal(t, "", services.DiscoveryContext.API)
		assert.Equal(t, "", services.SecretsManager.API)
	})

	t.Run("plain-HTTP identity host is rejected even though the hostname is allowlisted", func(t *testing.T) {
		logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
		ctx := klog.NewContext(t.Context(), logger)

		httpClient := MockDiscoveryServer(t, Services{
			Identity: ServiceEndpoint{
				API: "http://ajp5871.id.integration-cyberark.cloud",
			},
			DiscoveryContext: ServiceEndpoint{
				API: mockDiscoveryContextAPIURL,
			},
			SecretsManager: ServiceEndpoint{
				API: mockSecretsManagerAPIURL,
			},
		})

		client, err := New(httpClient, MockDiscoverySubdomain)
		require.NoError(t, err)
		services, _, err := client.DiscoverServices(ctx)
		require.Error(t, err)
		assert.Nil(t, services)
	})

	t.Run("plain-HTTP secrets_manager and discovery_context hosts are dropped, not fatal", func(t *testing.T) {
		logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
		ctx := klog.NewContext(t.Context(), logger)

		httpClient := MockDiscoveryServer(t, Services{
			Identity: ServiceEndpoint{
				API: mockIdentityAPIURL,
			},
			DiscoveryContext: ServiceEndpoint{
				API: "http://venafi-test.inventory.integration-cyberark.cloud",
			},
			SecretsManager: ServiceEndpoint{
				API: "http://venafi-test.secretsmgr.integration-cyberark.cloud",
			},
		})

		client, err := New(httpClient, MockDiscoverySubdomain)
		require.NoError(t, err)
		services, _, err := client.DiscoverServices(ctx)
		require.NoError(t, err)
		assert.Equal(t, mockIdentityAPIURL, services.Identity.API)
		assert.Equal(t, "", services.DiscoveryContext.API)
		assert.Equal(t, "", services.SecretsManager.API)
	})

	t.Run("gov-cloud root domains are accepted", func(t *testing.T) {
		logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
		ctx := klog.NewContext(t.Context(), logger)

		httpClient := MockDiscoveryServer(t, Services{
			Identity: ServiceEndpoint{
				API: "https://ajp5871.id.cyberarkgov.cloud",
			},
			DiscoveryContext: ServiceEndpoint{
				API: "https://venafi-test.inventory.integration-cyberarkgov.cloud",
			},
			SecretsManager: ServiceEndpoint{
				API: "https://venafi-test.secretsmgr.dev-cyberarkgov.com",
			},
		})

		client, err := New(httpClient, MockDiscoverySubdomain)
		require.NoError(t, err)
		services, _, err := client.DiscoverServices(ctx)
		require.NoError(t, err)
		assert.Equal(t, "https://ajp5871.id.cyberarkgov.cloud", services.Identity.API)
		assert.Equal(t, "https://venafi-test.inventory.integration-cyberarkgov.cloud", services.DiscoveryContext.API)
		assert.Equal(t, "https://venafi-test.secretsmgr.dev-cyberarkgov.com", services.SecretsManager.API)
	})

	t.Run("uppercase host is still accepted (net/url doesn't lowercase the host)", func(t *testing.T) {
		logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
		ctx := klog.NewContext(t.Context(), logger)

		httpClient := MockDiscoveryServer(t, Services{
			Identity: ServiceEndpoint{
				API: "https://AJP5871.ID.Integration-CyberArk.Cloud",
			},
			DiscoveryContext: ServiceEndpoint{
				API: mockDiscoveryContextAPIURL,
			},
			SecretsManager: ServiceEndpoint{
				API: mockSecretsManagerAPIURL,
			},
		})

		client, err := New(httpClient, MockDiscoverySubdomain)
		require.NoError(t, err)
		services, _, err := client.DiscoverServices(ctx)
		require.NoError(t, err)
		assert.Equal(t, "https://AJP5871.ID.Integration-CyberArk.Cloud", services.Identity.API)
	})

	t.Run("a host on the allowed domain but a different tenant's subdomain is warned about, not dropped", func(t *testing.T) {
		// Deliberately not enforcement -- see sanitizeServiceAPI's doc
		// comment for why. This also matches every other test in this file:
		// none of the mock*APIURL constants' leading labels are
		// MockDiscoverySubdomain ("tlskp-test"), and none of those tests
		// fail, which already exercises this path -- this test just makes
		// the "not dropped" property explicit and named.
		logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
		ctx := klog.NewContext(t.Context(), logger)

		httpClient := MockDiscoveryServer(t, Services{
			Identity: ServiceEndpoint{
				API: "https://some-other-tenant.id.integration-cyberark.cloud",
			},
			DiscoveryContext: ServiceEndpoint{
				API: mockDiscoveryContextAPIURL,
			},
			SecretsManager: ServiceEndpoint{
				API: mockSecretsManagerAPIURL,
			},
		})

		client, err := New(httpClient, MockDiscoverySubdomain)
		require.NoError(t, err)
		services, _, err := client.DiscoverServices(ctx)
		require.NoError(t, err)
		assert.Equal(t, "https://some-other-tenant.id.integration-cyberark.cloud", services.Identity.API)
	})

	for name, testSpec := range tests {
		t.Run(name, func(t *testing.T) {
			logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
			ctx := klog.NewContext(t.Context(), logger)

			httpClient := MockDiscoveryServer(t, Services{
				Identity: ServiceEndpoint{
					API: mockIdentityAPIURL,
				},
				DiscoveryContext: ServiceEndpoint{
					API: mockDiscoveryContextAPIURL,
				},
				SecretsManager: ServiceEndpoint{
					API: mockSecretsManagerAPIURL,
				},
			})

			client, err := New(httpClient, testSpec.subdomain)
			require.NoError(t, err)

			services, _, err := client.DiscoverServices(ctx)
			if testSpec.expectedError != nil {
				assert.EqualError(t, err, testSpec.expectedError.Error())
				assert.Nil(t, services)
				return
			}
			require.NoError(t, err)
			if services.Identity.API != testSpec.expectedURL {
				t.Errorf("expected API URL=%s\nobserved API URL=%s", testSpec.expectedURL, services.Identity.API)
			}
			// The Conjur authn-jwt exchange is served by secrets_manager, not
			// by identity_administration. Parsing it into the wrong field means
			// every live token exchange 404s/401s, which the Conjur unit tests
			// cannot catch because they point their mock at whichever field the
			// code reads.
			assert.Equal(t, mockSecretsManagerAPIURL, services.SecretsManager.API)
			assert.NotEqual(t, services.Identity.API, services.SecretsManager.API)
		})
	}
}
