package identity

// This file contains tests for the LoginUsernamePassword function in the
// identity package. The tests cover both a mock API server and the real API,
// depending on the environment variables set. The tests are intended to
// demonstrate that the mock API behaves the same as the real API

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/internal/cyberark/servicediscovery"
	arktesting "github.com/jetstack/preflight/internal/cyberark/testing"

	_ "k8s.io/klog/v2/ktesting/init"
)

// inputs holds the various input values for the tests.
type inputs struct {
	httpClient *http.Client
	baseURL    string
	subdomain  string
	username   string
	password   string
}

// TestLoginUsernamePassword_MockAPI tests the LoginUsernamePassword function
// against a mock API server. The mock server is configured to return different
// responses based on the username and password used in the request.
func TestLoginUsernamePassword_MockAPI(t *testing.T) {
	loginUsernamePasswordTests(t, func(t testing.TB) inputs {
		baseURL, httpClient := MockIdentityServer(t)
		return inputs{
			httpClient: httpClient,
			baseURL:    baseURL,
			subdomain:  "subdomain-ignored-by-mock",
			username:   successUser,
			password:   successPassword,
		}
	})
}

// TestLoginUsernamePassword_RealAPI tests the LoginUsernamePassword function
// against the real API. The environment variables are used to configure the
// client.
func TestLoginUsernamePassword_RealAPI(t *testing.T) {
	arktesting.SkipIfNoEnv(t)
	subdomain := os.Getenv("ARK_SUBDOMAIN")
	httpClient := http.DefaultClient
	services, err := servicediscovery.New(httpClient).DiscoverServices(t.Context(), subdomain)
	require.NoError(t, err)

	loginUsernamePasswordTests(t, func(t testing.TB) inputs {
		return inputs{
			httpClient: httpClient,
			baseURL:    services.Identity.API,
			subdomain:  subdomain,
			username:   os.Getenv("ARK_USERNAME"),
			password:   os.Getenv("ARK_SECRET"),
		}
	})
}

// loginUsernamePasswordTests runs tests which are expected to pass regardless of
// whether the mock or real API is used.
func loginUsernamePasswordTests(t *testing.T, inputsGenerator func(t testing.TB) inputs) {
	type testCase struct {
		name          string
		modifier      func(in *inputs)
		expectedError string
	}
	tests := []testCase{
		{
			name: "success",
		},
		{
			name: "bad-username",
			modifier: func(in *inputs) {
				in.username = failureUser
			},
			expectedError: `^got a failure response from request to advance authentication: ` +
				`message="Authentication \(login or challenge\) has failed\. ` +
				`Please try again or contact your system administrator\."`,
		},
		{
			name: "empty-username",
			modifier: func(in *inputs) {
				in.username = ""
			},
			expectedError: `^got a failure response from request to start authentication: ` +
				`message="Authentication \(login or challenge\) has failed\. ` +
				`Please try again or contact your system administrator\."`,
		},
		{
			name: "bad-password",
			modifier: func(in *inputs) {
				in.password = "bad-password"
			},
			expectedError: `^got a failure response from request to advance authentication: ` +
				`message="Authentication \(login or challenge\) has failed\. ` +
				`Please try again or contact your system administrator\."`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
			ctx := klog.NewContext(t.Context(), logger)

			in := inputsGenerator(t)
			if test.modifier != nil {
				test.modifier(&in)
			}
			cl := New(in.httpClient, in.baseURL, in.subdomain)
			err := cl.LoginUsernamePassword(ctx, in.username, []byte(in.password))
			if test.expectedError != "" {
				if assert.Error(t, err) {
					assert.Regexp(t, test.expectedError, err.Error())
				}
				return
			}
			require.NoError(t, err)
		})
	}
}
