package servicediscovery

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"text/template"

	"k8s.io/client-go/transport"

	"github.com/jetstack/preflight/pkg/version"

	_ "embed"
)

const (
	// MockDiscoverySubdomain is the subdomain for which the MockDiscoveryServer will return a success response
	MockDiscoverySubdomain = "tlskp-test"

	mockIdentityAPIURL         = "https://ajp5871.id.integration-cyberark.cloud"
	mockDiscoveryContextAPIURL = "https://venafi-test.inventory.integration-cyberark.cloud/"
	prefix                     = "/api/public/tenant-discovery?bySubdomain="
)

//go:embed testdata/discovery_success.json.template
var discoverySuccessTemplate string

type mockDiscoveryServer struct {
	t               testing.TB
	successResponse string
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
// The returned HTTP client has a transport which logs requests and responses
// depending on log level of the logger supplied in the context.
func MockDiscoveryServer(t testing.TB, services Services) *http.Client {
	tmpl := template.Must(template.New("mockDiscoverySuccess").Parse(discoverySuccessTemplate))
	buf := &bytes.Buffer{}
	err := tmpl.Execute(buf, services)
	if err != nil {
		panic(err)
	}
	mds := &mockDiscoveryServer{
		t:               t,
		successResponse: buf.String(),
	}
	server := httptest.NewTLSServer(mds)
	t.Cleanup(server.Close)
	t.Setenv("ARK_DISCOVERY_API", server.URL)
	httpClient := server.Client()
	httpClient.Transport = transport.NewDebuggingRoundTripper(httpClient.Transport, transport.DebugByContext)
	return httpClient
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
