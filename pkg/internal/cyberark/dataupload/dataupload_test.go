package dataupload_test

import (
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/jetstack/venafi-connection-lib/http_client"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/transport"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/pkg/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/pkg/internal/cyberark/identity"
	"github.com/jetstack/preflight/pkg/internal/cyberark/servicediscovery"
	"github.com/jetstack/preflight/pkg/version"

	_ "k8s.io/klog/v2/ktesting/init"
)

// TestCyberArkClient_PutSnapshot_MockAPI tests the dataupload code against a
// mock API server. The mock server is configured to return different responses
// based on the cluster ID and bearer token used in the request.
func TestCyberArkClient_PutSnapshot_MockAPI(t *testing.T) {
	setToken := func(token string) func(*http.Request) error {
		return func(req *http.Request) error {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			return nil
		}
	}

	tests := []struct {
		name         string
		snapshot     dataupload.Snapshot
		authenticate func(req *http.Request) error
		requireFn    func(t *testing.T, err error)
	}{
		{
			name: "successful upload",
			snapshot: dataupload.Snapshot{
				ClusterID:    "success-cluster-id",
				AgentVersion: "test-version",
			},
			authenticate: setToken("success-token"),
			requireFn: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "error when cluster ID is empty",
			snapshot: dataupload.Snapshot{
				ClusterID:    "",
				AgentVersion: "test-version",
			},
			authenticate: setToken("success-token"),
			requireFn: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "programmer mistake: the snapshot cluster ID cannot be left empty")
			},
		},
		{
			name: "error when bearer token is incorrect",
			snapshot: dataupload.Snapshot{
				ClusterID:    "test",
				AgentVersion: "test-version",
			},
			authenticate: setToken("fail-token"),
			requireFn: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "while retrieving snapshot upload URL: received response with status code 500: should authenticate using the correct bearer token")
			},
		},
		{
			name: "invalid JSON from server (RetrievePresignedUploadURL step)",
			snapshot: dataupload.Snapshot{
				ClusterID:    "invalid-json-retrieve-presigned",
				AgentVersion: "test-version",
			},
			authenticate: setToken("success-token"),
			requireFn: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "while retrieving snapshot upload URL: rejecting JSON response from server as it was too large or was truncated")
			},
		},
		{
			name: "500 from server (RetrievePresignedUploadURL step)",
			snapshot: dataupload.Snapshot{
				ClusterID:    "invalid-response-post-data",
				AgentVersion: "test-version",
			},
			authenticate: setToken("success-token"),
			requireFn: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "while retrieving snapshot upload URL: received response with status code 500: mock error")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
			ctx := klog.NewContext(t.Context(), logger)

			datauploadAPIBaseURL, httpClient := dataupload.MockDataUploadServer(t)

			cyberArkClient := dataupload.New(httpClient, datauploadAPIBaseURL, tc.authenticate)

			err := cyberArkClient.PutSnapshot(ctx, tc.snapshot)
			tc.requireFn(t, err)
		})
	}
}

// TestCyberArkClient_PutSnapshot_RealAPI demonstrates that the dataupload code works with the real inventory API.
// An API token is obtained by authenticating with the ARK_USERNAME and ARK_SECRET from the environment.
// ARK_SUBDOMAIN should be your tenant subdomain.
//
// To test against a tenant on the integration platform, also set:
// ARK_DISCOVERY_API=https://platform-discovery.integration-cyberark.cloud/api/v2
//
// To enable verbose request logging:
//
//	go test ./pkg/internal/cyberark/dataupload/... \
//	  -v -count 1 -run TestCyberArkClient_PutSnapshot_RealAPI -args -testing.v 6
func TestCyberArkClient_PutSnapshot_RealAPI(t *testing.T) {
	subdomain := os.Getenv("ARK_SUBDOMAIN")
	username := os.Getenv("ARK_USERNAME")
	secret := os.Getenv("ARK_SECRET")

	if subdomain == "" || username == "" || secret == "" {
		t.Skip("Skipping because one of the following environment variables is unset or empty: ARK_SUBDOMAIN, ARK_USERNAME, ARK_SECRET")
		return
	}

	logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
	ctx := klog.NewContext(t.Context(), logger)

	var rootCAs *x509.CertPool
	httpClient := http_client.NewDefaultClient(version.UserAgent(), rootCAs)
	httpClient.Transport = transport.NewDebuggingRoundTripper(httpClient.Transport, transport.DebugByContext)

	discoveryClient := servicediscovery.New(httpClient)

	services, err := discoveryClient.DiscoverServices(ctx, subdomain)
	require.NoError(t, err)

	identityClient := identity.New(httpClient, services.Identity.API, subdomain)
	err = identityClient.LoginUsernamePassword(ctx, username, []byte(secret))
	require.NoError(t, err)

	cyberArkClient := dataupload.New(httpClient, services.DiscoveryContext.API, identityClient.AuthenticateRequest)
	err = cyberArkClient.PutSnapshot(ctx, dataupload.Snapshot{
		ClusterID: "bb068932-c80d-460d-88df-34bc7f3f3297",
	})
	require.NoError(t, err)
}
