package oidc

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

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
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	res, ok := anyRes.(*api.OIDCDiscoveryData)
	if !ok {
		t.Fatalf("unexpected result type: %T", anyRes)
	}

	if res.OIDCConfig == nil {
		t.Fatalf("expected OIDCConfig, got nil")
	}
	if iss, _ := res.OIDCConfig["issuer"].(string); iss != "https://example" {
		t.Fatalf("unexpected issuer: %v", res.OIDCConfig["issuer"])
	}

	if res.JWKS == nil {
		t.Fatalf("expected JWKS, got nil")
	}
	if _, ok := res.JWKS["keys"].([]any); !ok {
		t.Fatalf("expected keys to be a slice, got %#v", res.JWKS["keys"])
	}
}

func TestFetch_Errors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			// return server error
			http.Error(w, "boom", http.StatusInternalServerError)
		case "/openid/v1/jwks":
			// return invalid JSON
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`}{`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	rc := makeRESTClient(t, ts)
	g := &DataGathererOIDC{cl: rc}

	anyRes, _, err := g.Fetch()
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	res, ok := anyRes.(*api.OIDCDiscoveryData)
	if !ok {
		t.Fatalf("unexpected result type: %T", anyRes)
	}

	if res.OIDCConfig != nil {
		t.Fatalf("expected nil OIDCConfig on error, got %#v", res.OIDCConfig)
	}
	if res.OIDCConfigError != "failed to get OIDC discovery document: an error on the server (\"boom\") has prevented the request from succeeding" {
		t.Fatalf("unexpected OIDCConfigError: %q", res.OIDCConfigError)
	}
	if res.JWKS != nil {
		t.Fatalf("expected nil JWKS on malformed JSON, got %#v", res.JWKS)
	}
	if res.JWKSError != "failed to unmarshal JWKS response: invalid character '}' looking for beginning of value" {
		t.Fatalf("unexpected JWKSError: %q", res.JWKSError)
	}
}
