package dataupload_test

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jetstack/venafi-connection-lib/http_client"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/transport"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/pkg/internal/cyberark/identity"
	"github.com/jetstack/preflight/pkg/internal/cyberark/servicediscovery"
	"github.com/jetstack/preflight/pkg/version"

	_ "k8s.io/klog/v2/ktesting/init"
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
		ClusterName: "success-cluster-id",
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
				require.ErrorContains(t, err, "while retrieving snapshot upload URL: received response with status code 500: should authenticate using the correct bearer token")
			},
		},
		{
			name:         "invalid JSON from server (RetrievePresignedUploadURL step)",
			payload:      defaultPayload,
			opts:         dataupload.Options{ClusterName: "invalid-json-retrieve-presigned"},
			authenticate: setToken("success-token"),
			requireFn: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "while retrieving snapshot upload URL: rejecting JSON response from server as it was too large or was truncated")
			},
		},
		{
			name:         "500 from server (RetrievePresignedUploadURL step)",
			payload:      defaultPayload,
			opts:         dataupload.Options{ClusterName: "invalid-response-post-data"},
			authenticate: setToken("success-token"),
			requireFn: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "while retrieving snapshot upload URL: received response with status code 500: mock error")
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

// TestPostDataReadingsWithOptionsWithRealAPI demonstrates that the dataupload code works with the real inventory API.
// An API token is obtained by authenticating with the ARK_USERNAME and ARK_SECRET from the environment.
// ARK_SUBDOMAIN should be your tenant subdomain.
// ARK_PLATFORM_DOMAIN should be either integration-cyberark.cloud or cyberark.cloud
//
// To test against a tenant on the integration platform, also set:
// ARK_DISCOVERY_API=https://platform-discovery.integration-cyberark.cloud/api/v2
//
// To enable verbose request logging:
//
//	go test ./pkg/internal/cyberark/dataupload/... \
//	  -v -count 1 -run TestPostDataReadingsWithOptionsWithRealAPI -args -testing.v 6
func TestPostDataReadingsWithOptionsWithRealAPI(t *testing.T) {
	platformDomain := os.Getenv("ARK_PLATFORM_DOMAIN")
	subdomain := os.Getenv("ARK_SUBDOMAIN")
	username := os.Getenv("ARK_USERNAME")
	secret := os.Getenv("ARK_SECRET")
	serviceDiscoveryAPI := os.Getenv("ARK_DISCOVERY_API")
	if serviceDiscoveryAPI == "" {
		serviceDiscoveryAPI = servicediscovery.ProdDiscoveryEndpoint
	}

	if platformDomain == "" || subdomain == "" || username == "" || secret == "" {
		t.Skip("Skipping because one of the following environment variables is unset or empty: ARK_PLATFORM_DOMAIN, ARK_SUBDOMAIN, ARK_USERNAME, ARK_SECRET")
		return
	}

	logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
	ctx := klog.NewContext(t.Context(), logger)

	const (
		discoveryContextServiceName = "inventory"
		separator                   = "."
	)

	serviceURL := fmt.Sprintf("https://%s%s%s.%s", subdomain, separator, discoveryContextServiceName, platformDomain)

	var rootCAs *x509.CertPool
	httpClient := http_client.NewDefaultClient(version.UserAgent(), rootCAs)
	httpClient.Transport = transport.NewDebuggingRoundTripper(httpClient.Transport, transport.DebugByContext)

	discoveryClient := servicediscovery.New(
		servicediscovery.WithHTTPClient(httpClient),
		servicediscovery.WithCustomEndpoint(serviceDiscoveryAPI),
	)

	identityAPI, err := discoveryClient.DiscoverIdentityAPIURL(ctx, subdomain)
	require.NoError(t, err)

	identityClient := identity.New(httpClient, identityAPI, subdomain)
	err = identityClient.LoginUsernamePassword(ctx, username, []byte(secret))
	require.NoError(t, err)

	cyberArkClient, err := dataupload.NewCyberArkClient(nil, serviceURL, identityClient.AuthenticateRequest)
	require.NoError(t, err)

	err = cyberArkClient.PostDataReadingsWithOptions(ctx, api.DataReadingsPost{}, dataupload.Options{
		ClusterName: "bb068932-c80d-460d-88df-34bc7f3f3297",
	})
	require.NoError(t, err)
}
