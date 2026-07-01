package conjur

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// MockConjurExchangeServer returns a TLS server whose authn-jwt endpoint returns the given token.
func MockConjurExchangeServer(t *testing.T, token string) (*httptest.Server, *http.Client) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.FormValue("jwt") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(token))
	}))
	return srv, srv.Client()
}

func MockConjurExchangeServerStatus(t *testing.T, status int) (*httptest.Server, *http.Client) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	return srv, srv.Client()
}
