package keyfetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jetstack/preflight/internal/cyberark"
	"github.com/jetstack/preflight/internal/cyberark/identity"
	"github.com/jetstack/preflight/internal/cyberark/servicediscovery"
)

// testClientSetup sets up a complete test environment with mock identity and discovery servers
// and returns a configured client along with the test ClientConfig
func testClientSetup(t *testing.T, jwksServerURL string) (*Client, cyberark.ClientConfig) {
	t.Helper()

	// Create mock identity server
	identityURL, httpClient := identity.MockIdentityServer(t)

	// Set up services for mock discovery server
	services := servicediscovery.Services{
		Identity: servicediscovery.ServiceEndpoint{
			IsActive: true,
			Type:     "main",
			API:      identityURL,
		},
		DiscoveryContext: servicediscovery.ServiceEndpoint{
			IsActive: true,
			Type:     "main",
			API:      jwksServerURL,
		},
	}

	// Create mock discovery server
	_ = servicediscovery.MockDiscoveryServer(t, services)

	// Create discovery client
	discoveryClient := servicediscovery.New(httpClient, servicediscovery.MockDiscoverySubdomain)

	// Create test config with credentials that match the mock identity server
	cfg := cyberark.ClientConfig{
		Subdomain: servicediscovery.MockDiscoverySubdomain,
		Username:  "test@example.com", // matches successUser in mock identity server
		Secret:    "somepassword",     // matches successPassword in mock identity server
	}

	// Create the keyfetch client with the properly configured httpClient
	client, err := NewClient(t.Context(), discoveryClient, cfg, httpClient)
	require.NoError(t, err)

	return client, cfg
}

func mockJWKSServer(t *testing.T, statusCode int, jwksResponse string) *httptest.Server {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is the JWKS endpoint
		if r.URL.Path == "/discovery-context/jwks" {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Accept"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			_, err := w.Write([]byte(jwksResponse))
			require.NoError(t, err)
		}
	}))

	t.Cleanup(server.Close)

	return server
}

func TestClient_FetchKey(t *testing.T) {
	// Sample JWKs response with a valid RSA key
	// This is a minimal example with the required fields, used in multiple tests
	jwksResponse := `{
"keys": [
			{
				"kty": "RSA",
				"use": "enc",
				"kid": "test-key-1",
				"alg": "RSA-OAEP-256",
				"n": "vDdioGpDuAEQDd4WRXyWa4sZ5EeS9OPsRrU_jU3PbZdDcANxfh_WSeSvSBKGfGXGC3fIzu0Ernk9VjXcs3LeFdRq2N4nNRZvCzsd_MjBtn7CWgjM_Sk9DXEGn3cHHilcJUJQ4i2YgX9bHu0odNgE6cSVIUEMIC2EGuGk_I7lwroinAAwXpNLLQkV_25kv_QQof2i5f7AocY6QTd0SAo8ZUqFBzanupkeFpl3-Bsz6_zdt_N0x9k5XHQn42Q2oTupTwvXFbE1x8XtCpiaP3_fsQ9dN7t4z6HtwlNUJB2tFfF6PgdKZ9LuJpYjFPYzJQ6Rv28fuc8YHcF7Jittjyzmew",
				"e": "AQAB"
			}
		]
	}`

	t.Run("successful fetch", func(t *testing.T) {

		server := mockJWKSServer(t, http.StatusOK, jwksResponse)

		client, _ := testClientSetup(t, server.URL)
		key, err := client.FetchKey(t.Context())

		require.NoError(t, err)

		assert.Equal(t, "test-key-1", key.KeyID)
		assert.NotNil(t, key.Key)
		assert.NotNil(t, key.Key.N)
		assert.Greater(t, key.Key.E, 0)
	})

	t.Run("multiple keys", func(t *testing.T) {
		// want to check that FetchKey returns the first valid RSA key, even if there are multiple keys in the response
		multiKeyResponse := `{
			"keys": [
				{
					"kty": "RSA",
					"kid": "key-1",
					"alg": "RSA-OAEP-256",
					"n": "vDdioGpDuAEQDd4WRXyWa4sZ5EeS9OPsRrU_jU3PbZdDcANxfh_WSeSvSBKGfGXGC3fIzu0Ernk9VjXcs3LeFdRq2N4nNRZvCzsd_MjBtn7CWgjM_Sk9DXEGn3cHHilcJUJQ4i2YgX9bHu0odNgE6cSVIUEMIC2EGuGk_I7lwroinAAwXpNLLQkV_25kv_QQof2i5f7AocY6QTd0SAo8ZUqFBzanupkeFpl3-Bsz6_zdt_N0x9k5XHQn42Q2oTupTwvXFbE1x8XtCpiaP3_fsQ9dN7t4z6HtwlNUJB2tFfF6PgdKZ9LuJpYjFPYzJQ6Rv28fuc8YHcF7Jittjyzmew",
					"e": "AQAB"
				},
				{
					"kty": "RSA",
					"kid": "key-2",
					"alg": "RSA-OAEP-256",
					"n": "4J0VE8FK1rSQUBGiLpk4MkPyFApCyCugOfkuH0hiHclxZay96JgyZylH97eqs-ZmWXtv42ynYctIj2ZleaoqVDfMOqZ1GsbccyNAYReDtUYgeUtJEajpfUo1vitoh6OEB6nB0Hau07ELLqcUoxH_zkH5Kwoi_BgxByJDQ1HOut6nyEPTXLTMrAYK_pqL_kzsU0OtrCgSBh6j-11ToqUfxsLupbadRC0t5zrq4-3mZKqxBUz4XB2g3b9d2lH7mOTl5J_E8jcD4tK9DePzjdbkRWonBEJetWl9f2mh_VD1sxJbie1kzM5cdQylXzV_AvhSr58w00qy6XR_QXI10UU16Q",
					"e": "AQAB"
				}
			]
		}`

		server := mockJWKSServer(t, http.StatusOK, multiKeyResponse)

		client, _ := testClientSetup(t, server.URL)
		key, err := client.FetchKey(t.Context())

		require.NoError(t, err)

		assert.Equal(t, "key-1", key.KeyID)
	})

	t.Run("filters non-RSA keys", func(t *testing.T) {
		// check that the client correctly filters out non-RSA keys and returns the first valid RSA key
		mixedKeyResponse := `{
			"keys": [
				{
					"kty": "EC",
					"kid": "ec-key-1",
					"alg": "ES256",
					"crv": "P-256",
					"x": "WKn-ZIGevcwGIyyrzFoZNBdaq9_TsqzGl96oc0CWuis",
					"y": "y77t-RvAHRKTsSGdIYUfweuOvwrvDD-Q3Hv5J0fSKbE"
				},
				{
					"kty": "RSA",
					"kid": "rsa-key-1",
					"alg": "RSA-OAEP-256",
					"n": "vDdioGpDuAEQDd4WRXyWa4sZ5EeS9OPsRrU_jU3PbZdDcANxfh_WSeSvSBKGfGXGC3fIzu0Ernk9VjXcs3LeFdRq2N4nNRZvCzsd_MjBtn7CWgjM_Sk9DXEGn3cHHilcJUJQ4i2YgX9bHu0odNgE6cSVIUEMIC2EGuGk_I7lwroinAAwXpNLLQkV_25kv_QQof2i5f7AocY6QTd0SAo8ZUqFBzanupkeFpl3-Bsz6_zdt_N0x9k5XHQn42Q2oTupTwvXFbE1x8XtCpiaP3_fsQ9dN7t4z6HtwlNUJB2tFfF6PgdKZ9LuJpYjFPYzJQ6Rv28fuc8YHcF7Jittjyzmew",
					"e": "AQAB"
				}
			]
		}`

		server := mockJWKSServer(t, http.StatusOK, mixedKeyResponse)

		client, _ := testClientSetup(t, server.URL)
		key, err := client.FetchKey(t.Context())

		require.NoError(t, err)
		assert.Equal(t, "rsa-key-1", key.KeyID)
	})

	t.Run("error on non-200 status", func(t *testing.T) {
		server := mockJWKSServer(t, http.StatusInternalServerError, "") // Response body won't be used since we return 500

		client, _ := testClientSetup(t, server.URL)
		_, err := client.FetchKey(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status code 500")
	})

	t.Run("error on invalid JSON", func(t *testing.T) {
		server := mockJWKSServer(t, http.StatusOK, "invalid json")

		client, _ := testClientSetup(t, server.URL)
		_, err := client.FetchKey(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JWKs response")
	})

	t.Run("error on no RSA keys", func(t *testing.T) {
		ecOnlyResponse := `{
			"keys": [
				{
					"kty": "EC",
					"kid": "ec-key-1",
					"alg": "ES256",
					"crv": "P-256",
					"x": "WKn-ZIGevcwGIyyrzFoZNBdaq9_TsqzGl96oc0CWuis",
					"y": "y77t-RvAHRKTsSGdIYUfweuOvwrvDD-Q3Hv5J0fSKbE"
				}
			]
		}`

		server := mockJWKSServer(t, http.StatusOK, ecOnlyResponse)

		client, _ := testClientSetup(t, server.URL)
		_, err := client.FetchKey(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no valid RSA keys found")
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// This handler will never respond
			<-r.Context().Done()
		}))
		defer server.Close()

		client, _ := testClientSetup(t, server.URL)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.FetchKey(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("authentication failure", func(t *testing.T) {
		server := mockJWKSServer(t, http.StatusOK, jwksResponse)

		// Create mock identity server
		identityURL, httpClient := identity.MockIdentityServer(t)

		// Set up services for mock discovery server
		services := servicediscovery.Services{
			Identity: servicediscovery.ServiceEndpoint{
				IsActive: true,
				Type:     "main",
				API:      identityURL,
			},
			DiscoveryContext: servicediscovery.ServiceEndpoint{
				IsActive: true,
				Type:     "main",
				API:      server.URL,
			},
		}

		// Create mock discovery server
		_ = servicediscovery.MockDiscoveryServer(t, services)

		// Create discovery client
		discoveryClient := servicediscovery.New(httpClient, servicediscovery.MockDiscoverySubdomain)

		// Create test config with WRONG credentials
		// Use the failureUser from the mock identity server
		cfg := cyberark.ClientConfig{
			Subdomain: servicediscovery.MockDiscoverySubdomain,
			Username:  "test-fail@example.com", // This user is configured to fail in the mock server // TODO: export these constants from the identity package to avoid hardcoding them here
			Secret:    "somepassword",
		}

		// Create the keyfetch client
		client, err := NewClient(t.Context(), discoveryClient, cfg, httpClient)
		require.NoError(t, err)

		_, err = client.FetchKey(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to authenticate")
	})

	t.Run("service discovery fails", func(t *testing.T) {
		// Create mock identity server (won't be used but needed for setup)
		identityURL, httpClient := identity.MockIdentityServer(t)

		// Set up services for mock discovery server
		services := servicediscovery.Services{
			Identity: servicediscovery.ServiceEndpoint{
				IsActive: true,
				Type:     "main",
				API:      identityURL,
			},
		}

		// Create mock discovery server
		_ = servicediscovery.MockDiscoveryServer(t, services)

		// Create discovery client with a subdomain that triggers failure
		discoveryClient := servicediscovery.New(httpClient, "bad-request")

		cfg := cyberark.ClientConfig{
			Subdomain: "bad-request",
			Username:  "test@example.com",
			Secret:    "somepassword",
		}

		_, err := NewClient(t.Context(), discoveryClient, cfg, httpClient)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get services from discovery client")
	})

	t.Run("ignores small RSA keys", func(t *testing.T) {
		// This is a 1024-bit RSA key (half the minimum size)
		// Generated with: openssl genrsa 1024 | openssl rsa -pubin -outform der | base64url
		smallKeyResponse := `{
			"keys": [
				{
					"kty": "RSA",
					"kid": "small-key-1",
					"alg": "RSA-OAEP-256",
					"n": "wKhJSKlx9aO_TmT4qAqN5EZ8FeXCXmh5F_hGHWL6c4lKvdKc_jBq1YI0H8pCIWZ6WhPKmBZ8JQ4Q2q0TjvdKLYQ8jqzMZxz4J_z4ySbN7yBn7N7xKqL5JN7KqVr7N8KQ",
					"e": "AQAB"
				},
				{
					"kty": "RSA",
					"kid": "valid-key",
					"alg": "RSA-OAEP-256",
					"n": "vDdioGpDuAEQDd4WRXyWa4sZ5EeS9OPsRrU_jU3PbZdDcANxfh_WSeSvSBKGfGXGC3fIzu0Ernk9VjXcs3LeFdRq2N4nNRZvCzsd_MjBtn7CWgjM_Sk9DXEGn3cHHilcJUJQ4i2YgX9bHu0odNgE6cSVIUEMIC2EGuGk_I7lwroinAAwXpNLLQkV_25kv_QQof2i5f7AocY6QTd0SAo8ZUqFBzanupkeFpl3-Bsz6_zdt_N0x9k5XHQn42Q2oTupTwvXFbE1x8XtCpiaP3_fsQ9dN7t4z6HtwlNUJB2tFfF6PgdKZ9LuJpYjFPYzJQ6Rv28fuc8YHcF7Jittjyzmew",
					"e": "AQAB"
				}
			]
		}`

		server := mockJWKSServer(t, http.StatusOK, smallKeyResponse)

		client, _ := testClientSetup(t, server.URL)
		key, err := client.FetchKey(t.Context())

		require.NoError(t, err)
		// Should skip the small key and return the valid one
		assert.Equal(t, "valid-key", key.KeyID)
	})

	t.Run("skips keys without kid", func(t *testing.T) {
		noKidResponse := `{
			"keys": [
				{
					"kty": "RSA",
					"alg": "RSA-OAEP-256",
					"n": "vDdioGpDuAEQDd4WRXyWa4sZ5EeS9OPsRrU_jU3PbZdDcANxfh_WSeSvSBKGfGXGC3fIzu0Ernk9VjXcs3LeFdRq2N4nNRZvCzsd_MjBtn7CWgjM_Sk9DXEGn3cHHilcJUJQ4i2YgX9bHu0odNgE6cSVIUEMIC2EGuGk_I7lwroinAAwXpNLLQkV_25kv_QQof2i5f7AocY6QTd0SAo8ZUqFBzanupkeFpl3-Bsz6_zdt_N0x9k5XHQn42Q2oTupTwvXFbE1x8XtCpiaP3_fsQ9dN7t4z6HtwlNUJB2tFfF6PgdKZ9LuJpYjFPYzJQ6Rv28fuc8YHcF7Jittjyzmew",
					"e": "AQAB"
				}
			]
		}`

		server := mockJWKSServer(t, http.StatusOK, noKidResponse)

		client, _ := testClientSetup(t, server.URL)
		_, err := client.FetchKey(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no valid RSA keys found")
	})

	t.Run("filters keys with wrong algorithm", func(t *testing.T) {
		wrongAlgResponse := `{
			"keys": [
				{
					"kty": "RSA",
					"kid": "wrong-alg-key",
					"alg": "RS256",
					"n": "vDdioGpDuAEQDd4WRXyWa4sZ5EeS9OPsRrU_jU3PbZdDcANxfh_WSeSvSBKGfGXGC3fIzu0Ernk9VjXcs3LeFdRq2N4nNRZvCzsd_MjBtn7CWgjM_Sk9DXEGn3cHHilcJUJQ4i2YgX9bHu0odNgE6cSVIUEMIC2EGuGk_I7lwroinAAwXpNLLQkV_25kv_QQof2i5f7AocY6QTd0SAo8ZUqFBzanupkeFpl3-Bsz6_zdt_N0x9k5XHQn42Q2oTupTwvXFbE1x8XtCpiaP3_fsQ9dN7t4z6HtwlNUJB2tFfF6PgdKZ9LuJpYjFPYzJQ6Rv28fuc8YHcF7Jittjyzmew",
					"e": "AQAB"
				},
				{
					"kty": "RSA",
					"kid": "correct-alg-key",
					"alg": "RSA-OAEP-256",
					"n": "4J0VE8FK1rSQUBGiLpk4MkPyFApCyCugOfkuH0hiHclxZay96JgyZylH97eqs-ZmWXtv42ynYctIj2ZleaoqVDfMOqZ1GsbccyNAYReDtUYgeUtJEajpfUo1vitoh6OEB6nB0Hau07ELLqcUoxH_zkH5Kwoi_BgxByJDQ1HOut6nyEPTXLTMrAYK_pqL_kzsU0OtrCgSBh6j-11ToqUfxsLupbadRC0t5zrq4-3mZKqxBUz4XB2g3b9d2lH7mOTl5J_E8jcD4tK9DePzjdbkRWonBEJetWl9f2mh_VD1sxJbie1kzM5cdQylXzV_AvhSr58w00qy6XR_QXI10UU16Q",
					"e": "AQAB"
				}
			]
		}`

		server := mockJWKSServer(t, http.StatusOK, wrongAlgResponse)

		client, _ := testClientSetup(t, server.URL)
		key, err := client.FetchKey(t.Context())

		require.NoError(t, err)
		// Should skip the RS256 key and return the RSA-OAEP-256 key
		assert.Equal(t, "correct-alg-key", key.KeyID)
	})

	t.Run("skips keys without algorithm", func(t *testing.T) {
		noAlgResponse := `{
			"keys": [
				{
					"kty": "RSA",
					"kid": "no-alg-key",
					"n": "vDdioGpDuAEQDd4WRXyWa4sZ5EeS9OPsRrU_jU3PbZdDcANxfh_WSeSvSBKGfGXGC3fIzu0Ernk9VjXcs3LeFdRq2N4nNRZvCzsd_MjBtn7CWgjM_Sk9DXEGn3cHHilcJUJQ4i2YgX9bHu0odNgE6cSVIUEMIC2EGuGk_I7lwroinAAwXpNLLQkV_25kv_QQof2i5f7AocY6QTd0SAo8ZUqFBzanupkeFpl3-Bsz6_zdt_N0x9k5XHQn42Q2oTupTwvXFbE1x8XtCpiaP3_fsQ9dN7t4z6HtwlNUJB2tFfF6PgdKZ9LuJpYjFPYzJQ6Rv28fuc8YHcF7Jittjyzmew",
					"e": "AQAB"
				}
			]
		}`

		server := mockJWKSServer(t, http.StatusOK, noAlgResponse)

		client, _ := testClientSetup(t, server.URL)
		_, err := client.FetchKey(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no valid RSA keys found")
	})

	t.Run("handles empty key set", func(t *testing.T) {
		emptyKeysResponse := `{
			"keys": []
		}`

		server := mockJWKSServer(t, http.StatusOK, emptyKeysResponse)

		client, _ := testClientSetup(t, server.URL)
		_, err := client.FetchKey(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no valid RSA keys found")
	})
}
