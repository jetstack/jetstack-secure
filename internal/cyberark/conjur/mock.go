package conjur

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// MockConjurExchangeServer returns a TLS server whose authn-jwt endpoint returns the given token.
func MockConjurExchangeServer(t testing.TB, token string) (*httptest.Server, *http.Client) {
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

func MockConjurExchangeServerStatus(t testing.TB, status int) (*httptest.Server, *http.Client) {
	t.Helper()
	return MockConjurExchangeServerStatusBody(t, status, nil)
}

// MockConjurExchangeServerStatusBody is like MockConjurExchangeServerStatus
// but also writes body, for asserting the client surfaces Conjur's own error
// message rather than discarding it.
func MockConjurExchangeServerStatusBody(t testing.TB, status int, body []byte) (*httptest.Server, *http.Client) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	return srv, srv.Client()
}
