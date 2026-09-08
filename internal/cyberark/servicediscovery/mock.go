package servicediscovery

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"text/template"

	"k8s.io/client-go/transport"

	arkapi "github.com/jetstack/preflight/internal/cyberark/api"
	cyberarktesting "github.com/jetstack/preflight/internal/cyberark/testing"
	"github.com/jetstack/preflight/pkg/version"

	_ "embed"
)

const (
	// MockDiscoverySubdomain is the subdomain for which the MockDiscoveryServer will return a success response
	MockDiscoverySubdomain = "tlskp-test"

	mockIdentityAPIURL         = "https://ajp5871.id.integration-cyberark.cloud"
	mockDiscoveryContextAPIURL = "https://venafi-test.inventory.integration-cyberark.cloud/"
	mockSecretsManagerAPIURL   = "https://venafi-test.secretsmgr.integration-cyberark.cloud/api"
	prefix                     = "/api/public/tenant-discovery?bySubdomain="
)

//go:embed testdata/discovery_success.json.template
var discoverySuccessTemplate string

type mockDiscoveryServer struct {
	t               testing.TB
	successResponse string
}

var fakeHostCounter atomic.Uint64

// launderIfLoopback rewrites rawURL to an allowlisted-domain-looking
// hostname and registers a dial redirect (via cyberarktesting.RegisterMockHost)
// to its real address, if rawURL's host is a loopback IP (a real httptest
// mock server address). Any other value — including deliberately-invalid
// test hosts like "attacker.example" — is returned unchanged, since those
// must still be rejected by the code under test, not laundered into passing.
func launderIfLoopback(rawURL string) string {
	if rawURL == "" {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return rawURL
	}
	fakeHost := fmt.Sprintf("mock-%d.integration-cyberark.cloud", fakeHostCounter.Add(1))
	cyberarktesting.RegisterMockHost(fakeHost, u.Host)
	u.Host = fakeHost
	return u.String()
}

// MockDiscoveryServer starts a mocked CyberArk service discovery server and
// returns an HTTP client with the CA certs needed to connect to it.
//
// The URL of the mock server is set in the `ARK_DISCOVERY_API` environment
// variable, so any code using the `servicediscovery.Client` will use this mock
// server.
//
// The mock server will return a successful response when the subdomain is
// `MockDiscoverySubdomain`, and the API URLs in the response will match those
// supplied in `services`.
// Other subdomains, can be used to trigger various failure responses.
//
// Any of services' API fields that point at a real loopback mock server
// (rather than a fake CyberArk-domain-looking test hostname) is laundered —
// see launderIfLoopback — into a fake CyberArk-domain hostname, with a dial
// redirect registered via cyberarktesting.RegisterMockHost so any
// WrapMockTransport-wrapped client (not just this one) can still reach it.
// This keeps DiscoverServices' domain/HTTPS allowlist (which now also
// covers ARK_DISCOVERY_API itself, see CP-26002) from stripping out real
// dataupload/conjur/identity mock addresses that other packages' tests
// embed here.
//
// The returned HTTP client has a transport which logs requests and responses
// depending on log level of the logger supplied in the context.
func MockDiscoveryServer(t testing.TB, services Services) *http.Client {
	services.Identity.API = launderIfLoopback(services.Identity.API)
	services.DiscoveryContext.API = launderIfLoopback(services.DiscoveryContext.API)
	services.SecretsManager.API = launderIfLoopback(services.SecretsManager.API)

	tmpl := template.Must(template.New("mockDiscoverySuccess").Parse(discoverySuccessTemplate))
	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, services); err != nil {
		panic(err)
	}

	mds := &mockDiscoveryServer{t: t, successResponse: buf.String()}
	server := httptest.NewTLSServer(mds)
	t.Cleanup(server.Close)

	httpClient := server.Client()
	baseTransport := httpClient.Transport.(*http.Transport).Clone()
	cyberarktesting.WrapMockTransport(baseTransport)

	discoveryFakeHost := fmt.Sprintf("mock-%d.integration-cyberark.cloud", fakeHostCounter.Add(1))
	cyberarktesting.RegisterMockHost(discoveryFakeHost, mustHostPort(t, server.URL))
	t.Setenv("ARK_DISCOVERY_API", "https://"+discoveryFakeHost)

	httpClient.Transport = transport.NewDebuggingRoundTripper(baseTransport, transport.DebugByContext)
	return httpClient
}

func mustHostPort(t testing.TB, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("MockDiscoveryServer: invalid server URL %q: %v", rawURL, err)
	}
	return u.Host
}

func (mds *mockDiscoveryServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mds.t.Log(r.Method, r.RequestURI)
	if r.Method != http.MethodGet {
		// This was observed by making a POST request to the integration environment
		// Normally, we'd expect 405 Method Not Allowed but we match the observed response here
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Missing Authentication Token"}`))
		return
	}

	if !strings.HasPrefix(r.URL.String(), prefix) {
		// This was observed by making a request to /api/v2/services/asd
		// Normally, we'd expect 404 Not Found but we match the observed response here
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Missing Authentication Token"}`))
		return
	}

	if r.Header.Get("User-Agent") != version.UserAgent() {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("should set user agent on all requests"))
		return
	}

	if r.Header.Get(arkapi.TelemetryHeaderKey) == "" {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("should set telemetry header on all requests"))
		return
	}

	if r.Header.Get("Accept") != "application/json" {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("should request JSON on all requests"))
		return
	}

	subdomain := strings.TrimPrefix(r.URL.String(), prefix)

	switch subdomain {
	case MockDiscoverySubdomain:
		_, _ = w.Write([]byte(mds.successResponse))

	case "no-identity":
		// return a snippet of valid service discovery JSON, but don't include the identity service
		_, _ = w.Write([]byte(`{
			"services": [
				{
					"service_name": "data_privacy",
					"region": "us-east-1",
					"endpoints": [
						{
							"is_active": true,
							"type": "main",
							"ui": "https://ui.dataprivacy.integration-cyberark.cloud/",
							"api": "https://us-east-1.dataprivacy.integration-cyberark.cloud/api"
						}
					]
				}
			]
		}`))

	case "bad-request":
		// test how the client handles a random unexpected response
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("{}"))

	case "json-invalid":
		// test that the client correctly rejects handles invalid JSON
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"a": a}`))

	case "json-too-long":
		// test that the client correctly rejects JSON which is too long
		w.WriteHeader(http.StatusOK)

		// we'll hex encode the random bytes (doubling the size)
		longData := make([]byte, 1+maxDiscoverBodySize/2)
		_, _ = rand.Read(longData)

		longJSON, err := json.Marshal(map[string]string{"key": hex.EncodeToString(longData)})
		if err != nil {
			panic(err)
		}

		_, _ = w.Write(longJSON)

	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("{}"))
	}
}
