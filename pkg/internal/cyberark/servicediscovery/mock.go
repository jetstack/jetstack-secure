package servicediscovery

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"text/template"

	"github.com/jetstack/preflight/pkg/version"
)

const (
	// MockDiscoverySubdomain is the subdomain for which the MockDiscoveryServer will return a success response
	MockDiscoverySubdomain = "venafi-test"

	defaultIdentityAPIURL = "https://ajp5871.id.integration-cyberark.cloud"
)

//go:embed testdata/discovery_success.json.template
var discoverySuccessTemplate string

type mockDiscoveryServer struct {
	Server *httptest.Server

	successResponse string
}

// MockDiscoveryServer returns a mocked discovery server with a default value for the Identity API.
// The returned server should be Closed by the caller after use.
func MockDiscoveryServer() *mockDiscoveryServer {
	return MockDiscoveryServerWithCustomAPIURL(defaultIdentityAPIURL)
}

func MockDiscoveryServerWithCustomAPIURL(apiURL string) *mockDiscoveryServer {
	tmpl := template.Must(template.New("mockDiscoverySuccess").Parse(discoverySuccessTemplate))

	buf := &bytes.Buffer{}

	err := tmpl.Execute(buf, struct{ IdentityAPIURL string }{apiURL})
	if err != nil {
		panic(err)
	}

	mds := &mockDiscoveryServer{
		successResponse: buf.String(),
	}

	server := httptest.NewServer(mds)

	mds.Server = server

	return mds
}

func (mds *mockDiscoveryServer) Close() {
	mds.Server.Close()
}

func (mds *mockDiscoveryServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		// This was observed by making a POST request to the integration environment
		// Normally, we'd expect 405 Method Not Allowed but we match the observed response here
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Missing Authentication Token"}`))
		return
	}

	if !strings.HasPrefix(r.URL.String(), "/services/subdomain/") {
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

	subdomain := strings.TrimPrefix(r.URL.String(), "/services/subdomain/")

	switch subdomain {
	case MockDiscoverySubdomain:
		_, _ = w.Write([]byte(mds.successResponse))

	case "no-identity":
		// return a snippet of valid service discovery JSON, but don't include the identity service
		_, _ = w.Write([]byte(`{"data_privacy": {"ui": "https://ui.dataprivacy.integration-cyberark.cloud/", "api": "https://us-east-1.dataprivacy.integration-cyberark.cloud/api", "bootstrap": "https://venafi-test-data_privacy.integration-cyberark.cloud", "region": "us-east-1"}}`))

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
