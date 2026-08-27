package identity

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuthenticateRequest_NoTokenCached(t *testing.T) {
	baseURL, httpClient := MockIdentityServer(t)
	c := New(httpClient, baseURL, "subdomain-ignored-by-mock")

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/x", nil)
	_, err := c.AuthenticateRequest(req)
	require.Error(t, err)
	require.Empty(t, req.Header.Get("Authorization"))
}

func TestAuthenticateRequest_UsesCachedTokenWithinTTL(t *testing.T) {
	baseURL, httpClient := MockIdentityServer(t)
	c := New(httpClient, baseURL, "subdomain-ignored-by-mock")
	require.NoError(t, c.LoginUsernamePassword(t.Context(), successUser, []byte(successPassword)))

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/x", nil)
	username, err := c.AuthenticateRequest(req)
	require.NoError(t, err)
	require.Equal(t, successUser, username)
	require.NotEmpty(t, req.Header.Get("Authorization"))
}

func TestAuthenticateRequest_RefreshesPastTokenTTL(t *testing.T) {
	baseURL, httpClient := MockIdentityServer(t)
	c := New(httpClient, baseURL, "subdomain-ignored-by-mock")
	require.NoError(t, c.LoginUsernamePassword(t.Context(), successUser, []byte(successPassword)))

	// Force the cached token to look stale, then let AuthenticateRequest
	// re-login on its own — this is the regression a caller that only calls
	// LoginUsernamePassword once at startup (e.g. keyfetch) would otherwise hit.
	c.tokenTTL = time.Nanosecond
	c.tokenCachedTime = time.Now().Add(-time.Hour)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/x", nil)
	username, err := c.AuthenticateRequest(req)
	require.NoError(t, err)
	require.Equal(t, successUser, username)
	require.NotEmpty(t, req.Header.Get("Authorization"))
	require.WithinDuration(t, time.Now(), c.tokenCachedTime, time.Second, "expected AuthenticateRequest to have refreshed the cached token")
}

func TestAuthenticateRequest_FailedRefreshFallsBackToCachedToken(t *testing.T) {
	baseURL, httpClient := MockIdentityServer(t)
	c := New(httpClient, baseURL, "subdomain-ignored-by-mock")
	require.NoError(t, c.LoginUsernamePassword(t.Context(), successUser, []byte(successPassword)))
	cachedToken := c.tokenCached.Token

	// Simulate the password having changed server-side: a refresh attempt
	// will now fail, but the previously cached token must still be served
	// rather than the request failing outright.
	c.secret = []byte("wrong-password")
	c.tokenTTL = time.Nanosecond
	c.tokenCachedTime = time.Now().Add(-time.Hour)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/x", nil)
	username, err := c.AuthenticateRequest(req)
	require.NoError(t, err)
	require.Equal(t, successUser, username)
	require.Equal(t, "Bearer "+cachedToken, req.Header.Get("Authorization"))
}
