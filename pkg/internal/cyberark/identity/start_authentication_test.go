package identity

import (
	"context"
	"fmt"
	"testing"

	"github.com/jetstack/preflight/pkg/internal/cyberark/servicediscovery"
)

func Test_IdentityStartAuthentication(t *testing.T) {
	tests := map[string]struct {
		username string

		expectedError error
	}{
		"successful request": {
			username:      successUser,
			expectedError: nil,
		},
		"successful request, multiple challenges": {
			username:      successUserMultipleChallenges,
			expectedError: fmt.Errorf("got 2 challenges in response to start authentication, which means MFA may be enabled; unable to log in"),
		},
		"successful request, multiple mechanisms": {
			username:      successUserMultipleMechanisms,
			expectedError: fmt.Errorf("got 2 mechanisms in response to start authentication, which means MFA may be enabled; unable to log in"),
		},
		"successful request, no username / password (UP) mechanism available": {
			username:      noUPMechanism,
			expectedError: errNoUPMechanism,
		},
		"failed request": {
			// experimentally we've seen the failure response when passing an empty username
			username:      "",
			expectedError: fmt.Errorf(`got a failure response from request to start authentication: message="Authentication (login or challenge) has failed. Please try again or contact your system administrator.", error="00000000-0400-4000-1111-222222222222:01234567890abcdef"`),
		},
	}

	for name, testSpec := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			identityServer := MockIdentityServer()
			defer identityServer.Close()

			mockDiscoveryServer := servicediscovery.MockDiscoveryServerWithCustomAPIURL(identityServer.Server.URL)
			defer mockDiscoveryServer.Close()

			discoveryClient := servicediscovery.New(servicediscovery.WithCustomEndpoint(mockDiscoveryServer.Server.URL))

			client, err := NewWithDiscoveryClient(ctx, discoveryClient, servicediscovery.MockDiscoverySubdomain)
			if err != nil {
				t.Errorf("failed to create identity client: %s", err)
				return
			}

			advanceBody, err := client.doStartAuthentication(ctx, testSpec.username)
			if err != nil {
				if testSpec.expectedError == nil {
					t.Errorf("didn't expect an error but got %v", err)
					return
				}

				if err.Error() != testSpec.expectedError.Error() {
					t.Errorf("expected err=%v\nbut got err=%v", testSpec.expectedError, err)
					return
				}
			}

			if testSpec.expectedError != nil {
				return
			}

			if advanceBody.TenantID != client.subdomain {
				t.Errorf("expected advanceAuthenticationRequestBody.TenantID to be %s but got %s", client.subdomain, advanceBody.TenantID)
			}

			if advanceBody.SessionID != successSessionID {
				t.Errorf("expected advanceAuthenticationRequestBody.SessionID to be %s but got %s", successSessionID, advanceBody.SessionID)
			}

			if advanceBody.MechanismID != successMechanismID {
				t.Errorf("expected advanceAuthenticationRequestBody.MechanismID to be %s but got %s", successMechanismID, advanceBody.MechanismID)
			}

			if advanceBody.Action != ActionAnswer {
				t.Errorf("expected advanceAuthenticationRequestBody.Action to be %s but got %s", ActionAnswer, advanceBody.Action)
			}

			if !advanceBody.PersistentLogin {
				t.Error("expected advanceAuthenticationRequestBody.PersistentLogin to be true but it wasn't")
			}

		})
	}
}
