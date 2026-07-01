package conjur

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type staticSource struct{ tok string }

func (s staticSource) Read(context.Context) (string, error) { return s.tok, nil }

func TestAuthenticateRequest_ExchangesAndSetsBearer(t *testing.T) {
	srv, httpClient := MockConjurExchangeServer(t, "conjur-access-token")
	defer srv.Close()

	c := New(httpClient, srv.URL, "dev-cluster", "conjur", staticSource{tok: "the-jwt"})
	req, _ := http.NewRequest(http.MethodGet, "https://disco/snapshot-links", nil)
	_, err := c.AuthenticateRequest(req)
	require.NoError(t, err)
	require.Equal(t, `Bearer conjur-access-token`, req.Header.Get("Authorization"))
}

func TestAuthenticateRequest_ExchangeFailsClosed(t *testing.T) {
	srv, httpClient := MockConjurExchangeServerStatus(t, http.StatusUnauthorized)
	defer srv.Close()
	c := New(httpClient, srv.URL, "dev-cluster", "conjur", staticSource{tok: "the-jwt"})
	req, _ := http.NewRequest(http.MethodGet, "https://disco/x", nil)
	_, err := c.AuthenticateRequest(req)
	require.Error(t, err)
	require.Empty(t, req.Header.Get("Authorization"))
}
