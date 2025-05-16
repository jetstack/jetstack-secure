package identity

import (
	"context"
	"fmt"
	"testing"

	"github.com/jetstack/preflight/pkg/internal/cyberark/servicediscovery"
)

func Test_IdentityAdvanceAuthentication(t *testing.T) {
	tests := map[string]struct {
		username    string
		advanceBody *advanceAuthenticationRequestBody

		expectedError error
	}{
		"success": {
			username: successUser,
			advanceBody: &advanceAuthenticationRequestBody{
				Action:          ActionAnswer,
				Answer:          successPassword,
				MechanismID:     successMechanismID,
				SessionID:       successSessionID,
				TenantID:        "foo",
				PersistantLogin: true,
			},

			expectedError: nil,
		},
		"incorrect password": {
			username: successUser,
			advanceBody: &advanceAuthenticationRequestBody{
				Action:          ActionAnswer,
				Answer:          "foo",
				MechanismID:     successMechanismID,
				SessionID:       successSessionID,
				TenantID:        "foo",
				PersistantLogin: true,
			},

			expectedError: fmt.Errorf(`got a failure response from request to advance authentication: message="Authentication (login or challenge) has failed. Please try again or contact your system administrator.", error="aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee:55555555555555555555555555555555"`),
		},
		"bad action": {
			username: successUser,
			advanceBody: &advanceAuthenticationRequestBody{
				Action:          "foo",
				Answer:          successPassword,
				MechanismID:     successMechanismID,
				SessionID:       successSessionID,
				TenantID:        "foo",
				PersistantLogin: true,
			},

			expectedError: fmt.Errorf(`got a failure response from request to advance authentication: message="Authentication (login or challenge) has failed. Please try again or contact your system administrator.", error="aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee:55555555555555555555555555555555"`),
		},
		"bad mechanism id": {
			username: successUser,
			advanceBody: &advanceAuthenticationRequestBody{
				Action:          ActionAnswer,
				Answer:          successPassword,
				MechanismID:     "foo",
				SessionID:       successSessionID,
				TenantID:        "foo",
				PersistantLogin: true,
			},

			expectedError: fmt.Errorf(`got a failure response from request to advance authentication: message="Authentication (login or challenge) has failed. Please try again or contact your system administrator.", error="aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee:55555555555555555555555555555555"`),
		},
		"bad session id": {
			username: successUser,
			advanceBody: &advanceAuthenticationRequestBody{
				Action:          ActionAnswer,
				Answer:          successPassword,
				MechanismID:     successMechanismID,
				SessionID:       "foo",
				TenantID:        "foo",
				PersistantLogin: true,
			},

			expectedError: fmt.Errorf(`got a failure response from request to advance authentication: message="Authentication (login or challenge) has failed. Please try again or contact your system administrator.", error="aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee:55555555555555555555555555555555"`),
		},
		"persistant login not set": {
			username: successUser,
			advanceBody: &advanceAuthenticationRequestBody{
				Action:          ActionAnswer,
				Answer:          successPassword,
				MechanismID:     successMechanismID,
				SessionID:       successSessionID,
				TenantID:        "foo",
				PersistantLogin: false,
			},

			expectedError: fmt.Errorf("got unexpected status code 403 Forbidden from request to advance authentication in CyberArk Identity API"),
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

			err = client.doAdvanceAuthentication(ctx, testSpec.username, testSpec.advanceBody)
			if testSpec.expectedError != err {
				if testSpec.expectedError == nil {
					t.Errorf("didn't expect an error but got %v", err)
					return
				}

				if err == nil {
					t.Errorf("expected no error but got err=%v", testSpec.expectedError)
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

			val, ok := client.tokenCache[testSpec.username]

			if !ok {
				t.Errorf("expected token for %s to be set to %q but wasn't found", testSpec.username, mockSuccessfulStartAuthenticationToken)
				return
			}

			if val != mockSuccessfulStartAuthenticationToken {
				t.Errorf("expected token for %s to be set to %q but was set to %q", testSpec.username, mockSuccessfulStartAuthenticationToken, val)
			}
		})
	}
}
