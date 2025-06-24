package dataupload_test

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/internal/cyberark/dataupload"
)

func TestCyberArkClient_PostDataReadingsWithOptions(t *testing.T) {
	fakeTime := time.Unix(123, 0)
	defaultPayload := api.DataReadingsPost{
		AgentMetadata: &api.AgentMetadata{
			Version:   "test-version",
			ClusterID: "test",
		},
		DataGatherTime: fakeTime,
		DataReadings: []*api.DataReading{
			{
				ClusterID:     "success-cluster-id",
				DataGatherer:  "test-gatherer",
				Timestamp:     api.Time{Time: fakeTime},
				Data:          map[string]interface{}{"test": "data"},
				SchemaVersion: "v1",
			},
		},
	}
	defaultOpts := dataupload.Options{
		ClusterName:        "success-cluster-id",
		ClusterDescription: "success-cluster-description",
	}

	setToken := func(token string) func(*http.Request) error {
		return func(req *http.Request) error {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			return nil
		}
	}

	tests := []struct {
		name         string
		payload      api.DataReadingsPost
		authenticate func(req *http.Request) error
		opts         dataupload.Options
		requireFn    func(t *testing.T, err error)
	}{
		{
			name:         "successful upload",
			payload:      defaultPayload,
			opts:         defaultOpts,
			authenticate: setToken("success-token"),
			requireFn: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:         "error when cluster name is empty",
			payload:      defaultPayload,
			opts:         dataupload.Options{ClusterName: ""},
			authenticate: setToken("success-token"),
			requireFn: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "programmer mistake: the cluster name")
			},
		},
		{
			name:         "error when bearer token is incorrect",
			payload:      defaultPayload,
			opts:         defaultOpts,
			authenticate: setToken("fail-token"),
			requireFn: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "received response with status code 500: should authenticate using the correct bearer token")
			},
		},
		{
			name:         "invalid JSON from server (RetrievePresignedUploadURL step)",
			payload:      defaultPayload,
			opts:         dataupload.Options{ClusterName: "invalid-json-retrieve-presigned", ClusterDescription: defaultOpts.ClusterDescription},
			authenticate: setToken("success-token"),
			requireFn: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "rejecting JSON response from server as it was too large or was truncated")
			},
		},
		{
			name:         "500 from server (PostData step)",
			payload:      defaultPayload,
			opts:         dataupload.Options{ClusterName: "invalid-response-post-data", ClusterDescription: defaultOpts.ClusterDescription},
			authenticate: setToken("success-token"),
			requireFn: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "received response with status code 500: mock error")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := dataupload.MockDataUploadServer()
			defer server.Close()

			certPool := x509.NewCertPool()
			require.True(t, certPool.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: server.Server.TLS.Certificates[0].Certificate[0],
			})))

			cyberArkClient, err := dataupload.NewCyberArkClient(certPool, server.Server.URL, tc.authenticate)
			require.NoError(t, err)

			err = cyberArkClient.PostDataReadingsWithOptions(t.Context(), tc.payload, tc.opts)
			tc.requireFn(t, err)
		})
	}
}
