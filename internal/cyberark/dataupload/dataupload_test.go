package dataupload_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/internal/cyberark/identity"
	"github.com/jetstack/preflight/pkg/version"

	_ "k8s.io/klog/v2/ktesting/init"
)

// TestCyberArkClient_PutSnapshot_MockAPI tests the dataupload code against a
// mock API server. The mock server is configured to return different responses
// based on the cluster ID and bearer token used in the request.
func TestCyberArkClient_PutSnapshot_MockAPI(t *testing.T) {
	setToken := func(token string) identity.RequestAuthenticator {
		return func(req *http.Request) (string, error) {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			return "foo@example.com", nil // set a dummy username for testing purposes; the actual value is not important for these tests
		}
	}

	tests := []struct {
		name         string
		snapshot     dataupload.Snapshot
		authenticate identity.RequestAuthenticator
		requireFn    func(t *testing.T, err error)
	}{
		{
			name: "successful upload",
			snapshot: dataupload.Snapshot{
				ClusterID:    "ffffffff-ffff-ffff-ffff-ffffffffffff",
				AgentVersion: version.PreflightVersion,
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

			cyberArkClient := dataupload.New(httpClient, datauploadAPIBaseURL, "test-tenant-uuid", tc.authenticate)

			err := cyberArkClient.PutSnapshot(ctx, tc.snapshot)
			tc.requireFn(t, err)
		})
	}
}
