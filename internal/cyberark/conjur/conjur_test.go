package conjur

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
// wire form) with the given `sub` claim and no `exp` claim, for test use only.
func buildJWSToken(t testing.TB, sub string) string {
	t.Helper()
	return buildJWSTokenClaims(t, sub, 0)
}

// buildJWSTokenClaims is like buildJWSToken but also sets `exp` (unix
// seconds) when exp != 0.
func buildJWSTokenClaims(t testing.TB, sub string, exp int64) string {
	t.Helper()
	claims := map[string]any{"sub": sub}
	if exp != 0 {
		claims["exp"] = exp
	}
	payload, err := json.Marshal(claims)
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
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/snapshot-links", nil)
	_, err := c.AuthenticateRequest(req)
	require.NoError(t, err)
	require.Equal(t, `Bearer conjur-access-token`, req.Header.Get("Authorization"))
}

func TestAuthenticateRequest_ExchangeFailsClosed(t *testing.T) {
	srv, httpClient := MockConjurExchangeServerStatus(t, http.StatusUnauthorized)
	defer srv.Close()
	c := New(httpClient, srv.URL, "dev-cluster", "conjur", staticSource{tok: "the-jwt"})
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/x", nil)
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
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/snapshot-links", nil)
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
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/snapshot-links", nil)
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
	req1, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/a", nil)
	identity1, err := c.AuthenticateRequest(req1)
	require.NoError(t, err)

	req2, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/b", nil)
	identity2, err := c.AuthenticateRequest(req2)
	require.NoError(t, err)

	require.Equal(t, sub, identity1)
	require.Equal(t, identity1, identity2)
	require.Equal(t, 1, exchanges, "expected only one exchange for two AuthenticateRequest calls within the token TTL")
}

func TestAuthenticateRequest_ExpiredExpClaimTriggersReexchange(t *testing.T) {
	const sub = "host/data/k8s/test-cluster-uuid/workloads/system:serviceaccount:test:test-agent"
	token := buildJWSTokenClaims(t, sub, time.Now().Add(-time.Minute).Unix())
	var exchanges int
	srv := mockConjurExchangeServerCountingExchanges(t, token, &exchanges)
	defer srv.Close()

	// A long fallback TTL would keep this cached if exp weren't honored.
	c := New(srv.Client(), srv.URL, "dev-cluster", "conjur", staticSource{tok: "the-jwt"})

	req1, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/a", nil)
	_, err := c.AuthenticateRequest(req1)
	require.NoError(t, err)

	req2, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/b", nil)
	_, err = c.AuthenticateRequest(req2)
	require.NoError(t, err)

	require.Equal(t, 2, exchanges, "a token whose own exp claim has already passed must not be served from cache")
}

func TestAuthenticateRequest_FutureExpClaimOverridesShortFallbackTTL(t *testing.T) {
	const sub = "host/data/k8s/test-cluster-uuid/workloads/system:serviceaccount:test:test-agent"
	token := buildJWSTokenClaims(t, sub, time.Now().Add(time.Hour).Unix())
	var exchanges int
	srv := mockConjurExchangeServerCountingExchanges(t, token, &exchanges)
	defer srv.Close()

	c := New(srv.Client(), srv.URL, "dev-cluster", "conjur", staticSource{tok: "the-jwt"})
	c.tokenTTL = time.Nanosecond // would force a re-exchange if exp weren't honored

	req1, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/a", nil)
	_, err := c.AuthenticateRequest(req1)
	require.NoError(t, err)

	req2, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/b", nil)
	_, err = c.AuthenticateRequest(req2)
	require.NoError(t, err)

	require.Equal(t, 1, exchanges, "a token's own future exp claim must be honored over a short fallback TTL")
}

func TestAuthenticateRequest_ExpWithinRefreshSkewTriggersReexchange(t *testing.T) {
	const sub = "host/data/k8s/test-cluster-uuid/workloads/system:serviceaccount:test:test-agent"
	// Still valid now, but expires inside the refresh skew window, so it must
	// not be handed to a second request that could outlive it.
	token := buildJWSTokenClaims(t, sub, time.Now().Add(refreshSkew/2).Unix())
	var exchanges int
	srv := mockConjurExchangeServerCountingExchanges(t, token, &exchanges)
	defer srv.Close()

	c := New(srv.Client(), srv.URL, "dev-cluster", "conjur", staticSource{tok: "the-jwt"})

	req1, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/a", nil)
	_, err := c.AuthenticateRequest(req1)
	require.NoError(t, err)

	req2, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/b", nil)
	_, err = c.AuthenticateRequest(req2)
	require.NoError(t, err)

	require.Equal(t, 2, exchanges, "a token expiring within the refresh skew must be re-exchanged, not served from cache")
}

func TestInvalidate_ForcesReexchange(t *testing.T) {
	const sub = "host/data/k8s/test-cluster-uuid/workloads/system:serviceaccount:test:test-agent"
	token := buildJWSToken(t, sub)
	var exchanges int
	srv := mockConjurExchangeServerCountingExchanges(t, token, &exchanges)
	defer srv.Close()

	c := New(srv.Client(), srv.URL, "dev-cluster", "conjur", staticSource{tok: "the-jwt"})

	req1, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/a", nil)
	_, err := c.AuthenticateRequest(req1)
	require.NoError(t, err)

	c.Invalidate()

	req2, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/b", nil)
	_, err = c.AuthenticateRequest(req2)
	require.NoError(t, err)

	require.Equal(t, 2, exchanges, "Invalidate must force the next AuthenticateRequest call to re-exchange")
}

// This error propagates all the way up to a Kubernetes Pod Event
// (pkg/agent/run.go's PushingErr notification), readable by anyone with `get
// events` in the namespace — so Conjur's response body (which can contain
// policy structure, service IDs and host identities) must not appear in it.
// The body is still logged at V(2) for an operator to go find, but that's
// exercised via the "authn-jwt exchange rejected" log line, not asserted
// here — ktesting has no easy log-buffer assertion in this codebase.
func TestAuthenticateRequest_ExchangeErrorOmitsConjurResponseBody(t *testing.T) {
	srv, httpClient := MockConjurExchangeServerStatusBody(t, http.StatusUnauthorized, []byte(`{"error":{"message":"CONJ00001E Invalid JWT token"}}`))
	defer srv.Close()

	c := New(httpClient, srv.URL, "dev-cluster", "conjur", staticSource{tok: "the-jwt"})
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/x", nil)
	_, err := c.AuthenticateRequest(req)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "CONJ00001E Invalid JWT token")
	require.Contains(t, err.Error(), "authn-jwt exchange rejected (401)")
}
