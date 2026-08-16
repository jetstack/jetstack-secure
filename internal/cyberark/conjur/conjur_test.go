package conjur

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type staticSource struct{ tok string }

func (s staticSource) Read(context.Context) (string, error) { return s.tok, nil }

// mockConjurExchangeServerCountingExchanges is like MockConjurExchangeServer but
// also counts how many times the exchange endpoint was hit, to verify
// token/identity caching doesn't re-exchange on every AuthenticateRequest call.
func mockConjurExchangeServerCountingExchanges(t testing.TB, token string, count *int) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.FormValue("jwt") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		*count++
		_, _ = w.Write([]byte(token))
	}))
}

// buildJWSToken builds a base64-encoded Flattened-JWS-JSON token (Conjur's
// wire form) with the given `sub` claim, for test use only.
func buildJWSToken(t testing.TB, sub string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"sub": sub})
	require.NoError(t, err)
	obj := map[string]string{
		"protected": base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(`{"alg":"conjur.v2"}`)),
		"payload":   base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(payload),
		"signature": "sig",
	}
	raw, err := json.Marshal(obj)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

func TestAuthenticateRequest_ExchangesAndSetsBearer(t *testing.T) {
	srv, httpClient := MockConjurExchangeServer(t, "conjur-access-token")
	defer srv.Close()

	c := New(httpClient, srv.URL, "dev-cluster", "conjur", staticSource{tok: "the-jwt"})
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/snapshot-links", nil)
	_, err := c.AuthenticateRequest(req)
	require.NoError(t, err)
	require.Equal(t, `Bearer conjur-access-token`, req.Header.Get("Authorization"))
}

func TestAuthenticateRequest_ExchangeFailsClosed(t *testing.T) {
	srv, httpClient := MockConjurExchangeServerStatus(t, http.StatusUnauthorized)
	defer srv.Close()
	c := New(httpClient, srv.URL, "dev-cluster", "conjur", staticSource{tok: "the-jwt"})
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/x", nil)
	_, err := c.AuthenticateRequest(req)
	require.Error(t, err)
	require.Empty(t, req.Header.Get("Authorization"))
}

func TestAuthenticateRequest_ReturnsSubClaimFromToken(t *testing.T) {
	const sub = "host/data/k8s/test-cluster-uuid/workloads/system:serviceaccount:test:test-agent"
	token := buildJWSToken(t, sub)
	srv, httpClient := MockConjurExchangeServer(t, token)
	defer srv.Close()

	c := New(httpClient, srv.URL, "dev-cluster", "conjur", staticSource{tok: "the-jwt"})
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/snapshot-links", nil)
	identity, err := c.AuthenticateRequest(req)
	require.NoError(t, err)
	require.Equal(t, sub, identity)
	require.Equal(t, "Bearer "+token, req.Header.Get("Authorization"))
}

func TestAuthenticateRequest_OpaqueTokenFallsBackToServiceID(t *testing.T) {
	// Opaque placeholder tokens (as used elsewhere in this repo's tests) are
	// not JWS-JSON; extraction must fail gracefully, not error the request.
	srv, httpClient := MockConjurExchangeServer(t, "success-token")
	defer srv.Close()

	c := New(httpClient, srv.URL, "dev-cluster", "conjur", staticSource{tok: "the-jwt"})
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/snapshot-links", nil)
	identity, err := c.AuthenticateRequest(req)
	require.NoError(t, err)
	require.Equal(t, "dev-cluster", identity)
}

func TestAuthenticateRequest_CachesIdentityWithToken(t *testing.T) {
	const sub = "host/data/k8s/test-cluster-uuid/workloads/system:serviceaccount:test:test-agent"
	token := buildJWSToken(t, sub)
	var exchanges int
	srv := mockConjurExchangeServerCountingExchanges(t, token, &exchanges)
	defer srv.Close()

	c := New(srv.Client(), srv.URL, "dev-cluster", "conjur", staticSource{tok: "the-jwt"})
	req1, _ := http.NewRequest(http.MethodGet, "https://example.com/a", nil)
	identity1, err := c.AuthenticateRequest(req1)
	require.NoError(t, err)

	req2, _ := http.NewRequest(http.MethodGet, "https://example.com/b", nil)
	identity2, err := c.AuthenticateRequest(req2)
	require.NoError(t, err)

	require.Equal(t, sub, identity1)
	require.Equal(t, identity1, identity2)
	require.Equal(t, 1, exchanges, "expected only one exchange for two AuthenticateRequest calls within the token TTL")
}
