package keyfetch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jetstack/preflight/internal/cyberark/servicediscovery"
)

// mockDiscoveryClient creates a discovery client that returns the given URL as the API endpoint
func mockDiscoveryClient(t *testing.T, apiURL string) *servicediscovery.Client {
	t.Helper()

	// Create a mock discovery server that returns the test server URL
	discoveryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := servicediscovery.DiscoveryResponse{
			TenantID: "test-tenant",
			Services: []servicediscovery.Service{
				{
					ServiceName: servicediscovery.DiscoveryContextServiceName,
					Endpoints: []servicediscovery.ServiceEndpoint{
						{
							IsActive: true,
							Type:     "main",
							API:      apiURL,
						},
					},
				},
				{
					ServiceName: servicediscovery.IdentityServiceName,
					Endpoints: []servicediscovery.ServiceEndpoint{
						{
							IsActive: true,
							Type:     "main",
							API:      "https://identity.example.com",
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(discoveryServer.Close)

	// Override the discovery API URL with our mock server
	t.Setenv("ARK_DISCOVERY_API", discoveryServer.URL)

	return servicediscovery.New(&http.Client{}, "test-subdomain")
}

func TestClient_FetchKey(t *testing.T) {
	// Sample JWKs response with a valid RSA key
	// This is a minimal example with the required fields
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
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Accept"))

			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(jwksResponse))
			require.NoError(t, err)
		}))
		defer server.Close()

		discoveryClient := mockDiscoveryClient(t, server.URL)
		client := NewClient(discoveryClient)
		key, err := client.FetchKey(context.Background())

		require.NoError(t, err)

		assert.Equal(t, "test-key-1", key.KeyID)
		assert.NotNil(t, key.Key)
		assert.NotNil(t, key.Key.N)
		assert.Greater(t, key.Key.E, 0)
	})

	t.Run("multiple keys", func(t *testing.T) {
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

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(multiKeyResponse))
			require.NoError(t, err)
		}))
		defer server.Close()

		discoveryClient := mockDiscoveryClient(t, server.URL)
		client := NewClient(discoveryClient)
		key, err := client.FetchKey(context.Background())

		require.NoError(t, err)

		assert.Equal(t, "key-1", key.KeyID)
	})

	t.Run("filters non-RSA keys", func(t *testing.T) {
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

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(mixedKeyResponse))
			require.NoError(t, err)
		}))
		defer server.Close()

		discoveryClient := mockDiscoveryClient(t, server.URL)
		client := NewClient(discoveryClient)
		key, err := client.FetchKey(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "rsa-key-1", key.KeyID)
	})

	t.Run("error on non-200 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal server error"))
		}))
		defer server.Close()

		discoveryClient := mockDiscoveryClient(t, server.URL)
		client := NewClient(discoveryClient)
		_, err := client.FetchKey(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status code 500")
	})

	t.Run("error on invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		discoveryClient := mockDiscoveryClient(t, server.URL)
		client := NewClient(discoveryClient)
		_, err := client.FetchKey(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JWKs response")
	})

	t.Run("error on no RSA keys", func(t *testing.T) {
		emptyResponse := `{
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

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(emptyResponse))
			require.NoError(t, err)
		}))
		defer server.Close()

		discoveryClient := mockDiscoveryClient(t, server.URL)
		client := NewClient(discoveryClient)
		_, err := client.FetchKey(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no valid RSA keys found")
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// This handler will never respond
			<-r.Context().Done()
		}))
		defer server.Close()

		discoveryClient := mockDiscoveryClient(t, server.URL)
		client := NewClient(discoveryClient)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.FetchKey(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}
