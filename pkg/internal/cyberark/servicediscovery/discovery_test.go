package servicediscovery

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	_ "k8s.io/klog/v2/ktesting/init"
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
			expectedError: fmt.Errorf("didn't find %s in service discovery response, which may indicate a suspended tenant; unable to detect CyberArk Identity API URL", IdentityServiceName),
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
			logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
			ctx := klog.NewContext(t.Context(), logger)

			httpClient := MockDiscoveryServer(t, Services{
				Identity: ServiceEndpoint{
					API: mockIdentityAPIURL,
				},
				DiscoveryContext: ServiceEndpoint{
					API: mockDiscoveryContextAPIURL,
				},
			})

			client := New(httpClient)

			services, err := client.DiscoverServices(ctx, testSpec.subdomain)
			if testSpec.expectedError != nil {
				assert.EqualError(t, err, testSpec.expectedError.Error())
				assert.Nil(t, services)
				return
			}
			require.NoError(t, err)
			if services.Identity.API != testSpec.expectedURL {
				t.Errorf("expected API URL=%s\nobserved API URL=%s", testSpec.expectedURL, services.Identity.API)
			}
		})
	}
}
