package servicediscovery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jetstack/preflight/pkg/version"

	_ "embed"
)

//go:embed testdata/discovery_success.json
var discoverySuccessResponse string

func testHandler(w http.ResponseWriter, r *http.Request) {
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
	case "venafi-test":
		_, _ = w.Write([]byte(discoverySuccessResponse))

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

func Test_DiscoverIdentityAPIURL(t *testing.T) {
	tests := map[string]struct {
		subdomain     string
		expectedURL   string
		expectedError error
	}{
		"successful request": {
			subdomain:     "venafi-test",
			expectedURL:   "https://ajp5871.id.integration-cyberark.cloud",
			expectedError: nil,
		},
		"subdomain not found": {
			subdomain:     "something-random",
			expectedURL:   "",
			expectedError: fmt.Errorf("got an HTTP 404 response from service discovery; maybe the subdomain %q is incorrect or does not exist?", "something-random"),
		},
		"no identity service in response": {
			subdomain:     "no-identity",
			expectedURL:   "",
			expectedError: fmt.Errorf("didn't find %s in service discovery response, which may indicate a suspended tenant; unable to detect CyberArk Identity API URL", identityServiceName),
		},
		"unexpected HTTP response": {
			subdomain:     "bad-request",
			expectedURL:   "",
			expectedError: fmt.Errorf("got unexpected status code 400 Bad Request from request to service discovery API"),
		},
		"response JSON too long": {
			subdomain:     "json-too-long",
			expectedURL:   "",
			expectedError: fmt.Errorf("rejecting JSON response from server as it was too large or was truncated"),
		},
		"response JSON invalid": {
			subdomain:     "json-invalid",
			expectedURL:   "",
			expectedError: fmt.Errorf("failed to parse JSON from otherwise successful request to service discovery endpoint: invalid character 'a' looking for beginning of value"),
		},
	}

	for name, testSpec := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			ts := httptest.NewServer(http.HandlerFunc(testHandler))

			defer ts.Close()

			client := New(WithCustomEndpoint(ts.URL))

			apiURL, err := client.DiscoverIdentityAPIURL(ctx, testSpec.subdomain)
			if err != nil {
				if err.Error() != testSpec.expectedError.Error() {
					t.Errorf("expectedError=%v\nobservedError=%v", testSpec.expectedError, err)
				}
			}

			// NB: we don't exit here because we also want to check the API URL is empty in the event of an error

			if apiURL != testSpec.expectedURL {
				t.Errorf("expected API URL=%s\nobserved API URL=%s", testSpec.expectedURL, apiURL)
			}
		})
	}
}
