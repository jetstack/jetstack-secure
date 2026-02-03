package oidc

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"

	"github.com/jetstack/preflight/api"
)

func makeRESTClient(t *testing.T, ts *httptest.Server) rest.Interface {
	t.Helper()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	cfg := &rest.Config{
		Host: u.Host,
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfigAndClient(cfg, ts.Client())
	if err != nil {
		t.Fatalf("new discovery client: %v", err)
	}

	return discoveryClient.RESTClient()
}

func TestFetch_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issuer":"https://example"}`))
		case "/openid/v1/jwks":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"keys":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	rc := makeRESTClient(t, ts)
	g := &DataGathererOIDC{cl: rc}

	anyRes, count, err := g.Fetch()
	require.NoError(t, err)
	require.Equal(t, 1, count)

	res, ok := anyRes.(*api.OIDCDiscoveryData)
	require.True(t, ok, "unexpected result type")

	require.NotNil(t, res.OIDCConfig)
	require.Equal(t, "https://example", res.OIDCConfig["issuer"].(string))
	require.Empty(t, res.OIDCConfigError)

	require.NotNil(t, res.JWKS)
	_, ok = res.JWKS["keys"].([]any)
	require.True(t, ok, "unexpected result type")
	require.Empty(t, res.JWKSError)
}

func TestFetch_Errors(t *testing.T) {
	tests := []struct {
		name                        string
		openidConfigurationResponse func(w http.ResponseWriter, r *http.Request)
		jwksResponse                func(w http.ResponseWriter, r *http.Request)
		expOIDCConfigError          string
		expJWKSError                string
	}{
		{
			name: "5xx errors",
			openidConfigurationResponse: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "boom", http.StatusInternalServerError)
			},
			jwksResponse: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "boom", http.StatusInternalServerError)
			},
			expOIDCConfigError: `failed to get /.well-known/openid-configuration: Error from server (InternalError): an error on the server ("boom") has prevented the request from succeeding`,
			expJWKSError:       `failed to get /openid/v1/jwks: Error from server (InternalError): an error on the server ("boom") has prevented the request from succeeding`,
		},
		{
			name: "malformed JSON",
			openidConfigurationResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`}{`))
			},
			jwksResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`}`))
				_, _ = w.Write(bytes.Repeat([]byte{'0'}, 5000))
			},
			expOIDCConfigError: `failed to unmarshal OIDC discovery document: invalid character '}' looking for beginning of value (raw: "}{")`,
			expJWKSError:       `failed to unmarshal JWKS response: invalid character '}' looking for beginning of value (raw: "}0000000000000000000000000000000000000000000000000000000000000000000000000000000")`,
		},
		{
			name: "Forbidden error (no body)",
			openidConfigurationResponse: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "forbidden", http.StatusForbidden)
			},
			jwksResponse: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "forbidden", http.StatusForbidden)
			},
			expOIDCConfigError: "failed to get /.well-known/openid-configuration: Error from server (Forbidden): forbidden",
			expJWKSError:       "failed to get /openid/v1/jwks: Error from server (Forbidden): forbidden",
		},
		{
			name: "Forbidden error (*metav1.Status body)",
			openidConfigurationResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{
					"kind":"Status",
					"apiVersion":"v1",
					"metadata":{},
					"status":"Failure",
					"message":"forbidden: User \"system:serviceaccount:default:test\" cannot get path \"/.well-known/openid-configuration\"",
					"reason":"Forbidden",
					"details":{},
					"code":403
				}`))
			},
			jwksResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{
					"kind":"Status",
					"apiVersion":"v1",
					"metadata":{},
					"status":"Failure",
					"message":"forbidden: User \"system:serviceaccount:default:test\" cannot get path \"/openid/v1/jwks\"",
					"reason":"Forbidden",
					"details":{},
					"code":403
				}`))
			},
			expOIDCConfigError: `failed to get /.well-known/openid-configuration: Error from server (Forbidden): forbidden: User "system:serviceaccount:default:test" cannot get path "/.well-known/openid-configuration"`,
			expJWKSError:       `failed to get /openid/v1/jwks: Error from server (Forbidden): forbidden: User "system:serviceaccount:default:test" cannot get path "/openid/v1/jwks"`,
		},
		{
			name: "Unauthorized error (*metav1.Status body)",
			openidConfigurationResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{
					"kind": "Status",
					"apiVersion": "v1",
					"metadata": {},
					"status": "Failure",
					"message": "Unauthorized",
					"reason": "Unauthorized",
					"code": 401
				}`))
			},
			jwksResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{
					"kind": "Status",
					"apiVersion": "v1",
					"metadata": {},
					"status": "Failure",
					"message": "Unauthorized",
					"reason": "Unauthorized",
					"code": 401
				}`))
			},
			expOIDCConfigError: `failed to get /.well-known/openid-configuration: error: You must be logged in to the server (Unauthorized)`,
			expJWKSError:       `failed to get /openid/v1/jwks: error: You must be logged in to the server (Unauthorized)`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/.well-known/openid-configuration":
					tc.openidConfigurationResponse(w, r)
					return
				case "/openid/v1/jwks":
					tc.jwksResponse(w, r)
					return
				default:
					t.Fatalf("unexpected request path: %s", r.URL.Path)
				}
			}))
			defer ts.Close()

			rc := makeRESTClient(t, ts)
			g := &DataGathererOIDC{cl: rc}

			anyRes, count, err := g.Fetch()
			require.NoError(t, err)
			require.Equal(t, 1, count)

			res, ok := anyRes.(*api.OIDCDiscoveryData)
			require.True(t, ok, "unexpected result type")

			require.Nil(t, res.OIDCConfig)
			require.NotEmpty(t, res.OIDCConfigError)
			require.Equal(t, tc.expOIDCConfigError, res.OIDCConfigError)

			require.Nil(t, res.JWKS)
			require.NotEmpty(t, res.JWKSError)
			require.Equal(t, tc.expJWKSError, res.JWKSError)
		})
	}
}
