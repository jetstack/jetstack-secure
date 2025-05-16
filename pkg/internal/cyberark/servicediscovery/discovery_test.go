package servicediscovery

import (
	"context"
	"fmt"
	"testing"
)

func Test_DiscoverIdentityAPIURL(t *testing.T) {
	tests := map[string]struct {
		subdomain     string
		expectedURL   string
		expectedError error
	}{
		"successful request": {
			subdomain:     MockDiscoverySubdomain,
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

			ts := MockDiscoveryServer()
			defer ts.Close()

			client := New(WithCustomEndpoint(ts.Server.URL))

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
