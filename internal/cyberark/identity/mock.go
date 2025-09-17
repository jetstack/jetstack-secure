package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/transport"

	arkapi "github.com/jetstack/preflight/internal/cyberark/api"
	"github.com/jetstack/preflight/pkg/version"

	_ "embed"
)

const (
	successUser                   = "test@example.com"
	failureUser                   = "test-fail@example.com"
	successUserMultipleChallenges = "test-multiple-challenges@example.com"
	successUserMultipleMechanisms = "test-multiple-mechanisms@example.com"
	noUPMechanism                 = "noup@example.com"

	successMechanismID = "aaaaaaa_AAAAAAAAAAAAAAAAAAAAAAAAAAAA-1111111"
	successSessionID   = "mysessionid101"
	successPassword    = "somepassword"

	// mockSuccessfulStartAuthenticationToken is the token returned by the
	// mock server in response to a successful AdvanceAuthentication request
	// Must match what's in testdata/advance_authentication_success.json
	mockSuccessfulStartAuthenticationToken = "success-token"
)

var (
	//go:embed testdata/start_authentication_success.json
	startAuthenticationSuccessResponse string

	//go:embed testdata/start_authentication_bad_user_session_id.json
	startAuthenticationBadUserResponse string

	//go:embed testdata/start_authentication_success_multiple_challenges.json
	startAuthenticationSuccessMultipleChallengesResponse string

	//go:embed testdata/start_authentication_success_multiple_mechanisms.json
	startAuthenticationSuccessMultipleMechanismsResponse string

	//go:embed testdata/start_authentication_success_no_up_mechanism.json
	startAuthenticationNoUPMechanismResponse string

	//go:embed testdata/start_authentication_failure.json
	startAuthenticationFailureResponse string

	//go:embed testdata/advance_authentication_success.json
	advanceAuthenticationSuccessResponse string

	//go:embed testdata/advance_authentication_failure.json
	advanceAuthenticationFailureResponse string
)

type mockIdentityServer struct {
	t testing.TB
}

// MockIdentityServer returns a URL of a mocked CyberArk identity server and an
// HTTP client with the CA certs needed to connect to it..
func MockIdentityServer(t testing.TB) (string, *http.Client) {
	mis := &mockIdentityServer{
		t: t,
	}
	server := httptest.NewTLSServer(mis)
	t.Cleanup(server.Close)
	httpClient := server.Client()
	httpClient.Transport = transport.NewDebuggingRoundTripper(httpClient.Transport, transport.DebugByContext)
	return server.URL, httpClient
}

func (mis *mockIdentityServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mis.t.Log(r.Method, r.RequestURI)
	switch r.URL.String() {
	case "/Security/StartAuthentication":
		mis.handleStartAuthentication(w, r)
		return

	case "/Security/AdvanceAuthentication":
		mis.handleAdvanceAuthentication(w, r)
		return

	default:
		// The server returns an HTML page for this case, but that doesn't seem important for us to replicate
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}
}

func checkRequestHeaders(r *http.Request) error {
	var errs []error

	if r.Header.Get("User-Agent") != version.UserAgent() {
		errs = append(errs, fmt.Errorf("should set user agent on all requests"))
	}

	if r.Header.Get("Content-Type") != "application/json" {
		errs = append(errs, fmt.Errorf("should request JSON on all requests"))
	}

	if r.Header.Get("X-IDAP-NATIVE-CLIENT") != "true" { //nolint: canonicalheader
		errs = append(errs, fmt.Errorf("should set X-IDAP-NATIVE-CLIENT header to true on all requests"))
	}

	if r.Header.Get(arkapi.TelemetryHeaderKey) == "" {
		errs = append(errs, fmt.Errorf("should set telemetry header on all requests"))
	}

	return errors.Join(errs...)
}

func (mis *mockIdentityServer) handleStartAuthentication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		// Empirically we saw that a PUT and a DELETE request to this endpoint was actually successful,
		// but the endpoint is documented to use POST so we'll ensure that only that method is used.
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"endpoint is documented to only accept POST"}`))
		return
	}

	if err := checkRequestHeaders(r); !assert.NoError(mis.t, err, "request headers are not correct") {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	reqBody := startAuthenticationRequestBody{}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&reqBody); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"message":"failed to unmarshal request body: %s"}`, err)
		return
	}

	switch reqBody.User {
	case successUser:
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(startAuthenticationSuccessResponse))

	case successUserMultipleChallenges:
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(startAuthenticationSuccessMultipleChallengesResponse))

	case successUserMultipleMechanisms:
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(startAuthenticationSuccessMultipleMechanismsResponse))

	case noUPMechanism:
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(startAuthenticationNoUPMechanismResponse))

	case "":
		// experimentally, this case produces a 200 response but a "failed" body
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(startAuthenticationFailureResponse))

	case failureUser:
		// Experimentally, the real API produces a 200 response and what looks
		// like a success response body. but the login is rejected later by the
		// AdvanceAuthentication stage, perhaps by virtue of the sessionID which
		// is returned here and supplied to AdvanceAuthentication.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(startAuthenticationBadUserResponse))

	default:
		panic("programmer error: should not be reached")
	}
}

func (mis *mockIdentityServer) handleAdvanceAuthentication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"endpoint is documented to only accept POST"}`))
		return
	}

	if err := checkRequestHeaders(r); err != nil {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"message":"issues with headers sent to mock server: %s"}`, err.Error())
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	advanceBody := &advanceAuthenticationRequestBody{}

	if err := decoder.Decode(&advanceBody); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"message":"failed to unmarshal request body: %s"}`, err)
		return
	}

	// Important: The actual server will return 200 OK even if the login fails.
	// Most failure responses should copy that.

	if !advanceBody.PersistentLogin {
		// this is something we enforce but wouldn't actually be an error from
		// a real server, so we return a different error here
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`expected PersistentLogin to be true`))
		return
	}

	if advanceBody.SessionID != successSessionID ||
		advanceBody.MechanismID != successMechanismID ||
		advanceBody.Action != ActionAnswer ||
		advanceBody.Answer != successPassword {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(advanceAuthenticationFailureResponse))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(advanceAuthenticationSuccessResponse))
}
