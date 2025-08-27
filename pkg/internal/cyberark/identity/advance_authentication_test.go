package identity

import (
	"fmt"
	"testing"

	"github.com/jetstack/preflight/pkg/internal/cyberark/servicediscovery"
)

func Test_IdentityAdvanceAuthentication(t *testing.T) {
	tests := map[string]struct {
		username    string
		password    []byte
		advanceBody advanceAuthenticationRequestBody

		expectedError error
	}{
		"success": {
			username: successUser,
			password: []byte(successPassword),
			advanceBody: advanceAuthenticationRequestBody{
				Action:          ActionAnswer,
				MechanismID:     successMechanismID,
				SessionID:       successSessionID,
				TenantID:        "foo",
				PersistentLogin: true,
			},

			expectedError: nil,
		},
		"incorrect password": {
			username: successUser,
			password: []byte("foo"),
			advanceBody: advanceAuthenticationRequestBody{
				Action:          ActionAnswer,
				MechanismID:     successMechanismID,
				SessionID:       successSessionID,
				TenantID:        "foo",
				PersistentLogin: true,
			},

			expectedError: fmt.Errorf(`got a failure response from request to advance authentication: message="Authentication (login or challenge) has failed. Please try again or contact your system administrator.", error="aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee:55555555555555555555555555555555"`),
		},
		"bad action": {
			username: successUser,
			password: []byte(successPassword),
			advanceBody: advanceAuthenticationRequestBody{
				Action:          "foo",
				MechanismID:     successMechanismID,
				SessionID:       successSessionID,
				TenantID:        "foo",
				PersistentLogin: true,
			},

			expectedError: fmt.Errorf(`got a failure response from request to advance authentication: message="Authentication (login or challenge) has failed. Please try again or contact your system administrator.", error="aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee:55555555555555555555555555555555"`),
		},
		"bad mechanism id": {
			username: successUser,
			password: []byte(successPassword),
			advanceBody: advanceAuthenticationRequestBody{
				Action:          ActionAnswer,
				MechanismID:     "foo",
				SessionID:       successSessionID,
				TenantID:        "foo",
				PersistentLogin: true,
			},

			expectedError: fmt.Errorf(`got a failure response from request to advance authentication: message="Authentication (login or challenge) has failed. Please try again or contact your system administrator.", error="aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee:55555555555555555555555555555555"`),
		},
		"bad session id": {
			username: successUser,
			password: []byte(successPassword),
			advanceBody: advanceAuthenticationRequestBody{
				Action:          ActionAnswer,
				MechanismID:     successMechanismID,
				SessionID:       "foo",
				TenantID:        "foo",
				PersistentLogin: true,
			},

			expectedError: fmt.Errorf(`got a failure response from request to advance authentication: message="Authentication (login or challenge) has failed. Please try again or contact your system administrator.", error="aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee:55555555555555555555555555555555"`),
		},
		"persistent login not set": {
			username: successUser,
			password: []byte(successPassword),
			advanceBody: advanceAuthenticationRequestBody{
				Action:          ActionAnswer,
				MechanismID:     successMechanismID,
				SessionID:       successSessionID,
				TenantID:        "foo",
				PersistentLogin: false,
			},

			expectedError: fmt.Errorf("got unexpected status code 403 Forbidden from request to advance authentication in CyberArk Identity API"),
		},
	}

	for name, testSpec := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()

			identityAPI, httpClient := MockIdentityServer(t)

			client := New(httpClient, identityAPI, servicediscovery.MockDiscoverySubdomain)

			err := client.doAdvanceAuthentication(ctx, testSpec.username, &testSpec.password, testSpec.advanceBody)
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

			if len(client.tokenCached) == 0 {
				t.Errorf("expected token for %s to be set to %q but wasn't found", testSpec.username, mockSuccessfulStartAuthenticationToken)
				return
			}

			if client.tokenCached != mockSuccessfulStartAuthenticationToken {
				t.Errorf("expected token for %s to be set to %q but was set to %q", testSpec.username, mockSuccessfulStartAuthenticationToken, client.tokenCached)
			}
		})
	}
}
