package dataupload_test

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/pkg/internal/cyberark/identity"
	"github.com/jetstack/preflight/pkg/internal/cyberark/servicediscovery"
	"github.com/jetstack/preflight/pkg/internal/cyberark/testutil"

	_ "k8s.io/klog/v2/ktesting/init"
)

func genNamespace(name string) *unstructured.Unstructured {
	o := &unstructured.Unstructured{}
	o.SetAPIVersion("")
	o.SetKind("Namespace")
	o.SetName(name)
	return o
}

func genArkNamespacesDataReading(clusterID types.UID) *api.DataReading {
	kubeSystemNamespace := genNamespace("kube-system")
	kubeSystemNamespace.SetUID(clusterID)
	return &api.DataReading{
		ClusterID:    "ignored-tlspk-cluster-id",
		DataGatherer: "ark/namespaces",
		Data: &api.DynamicData{
			Items: []*api.GatheredResource{
				{
					Resource: kubeSystemNamespace,
				},
				{
					Resource: genNamespace("kube-public"),
				},
				{
					Resource: genNamespace("venafi"),
				},
				{
					Resource: genNamespace("cert-manager"),
				},
			},
		},
		SchemaVersion: "v1",
	}
}

func TestCyberArkClient_PostDataReadings_MockAPI(t *testing.T) {
	defaultDataReadings := []*api.DataReading{
		genArkNamespacesDataReading("success-cluster-id"),
	}

	setToken := func(token string) func(*http.Request) error {
		return func(req *http.Request) error {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			return nil
		}
	}

	tests := []struct {
		name         string
		readings     []*api.DataReading
		authenticate func(req *http.Request) error
		requireFn    func(t *testing.T, err error)
	}{
		{
			name:         "successful upload",
			readings:     defaultDataReadings,
			authenticate: setToken("success-token"),
			requireFn: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:         "error when cluster ID not found among data readings",
			readings:     nil,
			authenticate: setToken("success-token"),
			requireFn: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "while converting datareadings to Cyberark snapshot format: failed to compute a clusterID from the data-readings")
			},
		},
		{
			name:         "error when bearer token is incorrect",
			readings:     defaultDataReadings,
			authenticate: setToken("fail-token"),
			requireFn: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "while retrieving snapshot upload URL: received response with status code 500: should authenticate using the correct bearer token")
			},
		},
		{
			name:     "error contains authenticate error",
			readings: defaultDataReadings,
			authenticate: func(_ *http.Request) error {
				return errors.New("simulated-authenticate-error")
			},
			requireFn: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "while retrieving snapshot upload URL: failed to authenticate request: simulated-authenticate-error")
			},
		},
		{
			name: "invalid JSON from server (RetrievePresignedUploadURL step)",
			readings: []*api.DataReading{
				genArkNamespacesDataReading("invalid-json-retrieve-presigned"),
			},
			authenticate: setToken("success-token"),
			requireFn: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "while retrieving snapshot upload URL: rejecting JSON response from server as it was too large or was truncated")
			},
		},
		{
			name: "500 from server (RetrievePresignedUploadURL step)",
			readings: []*api.DataReading{
				genArkNamespacesDataReading("invalid-response-post-data"),
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

			server := dataupload.MockDataUploadServer()
			defer server.Close()

			certPool := x509.NewCertPool()
			require.True(t, certPool.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: server.Server.TLS.Certificates[0].Certificate[0],
			})))

			cyberArkClient, err := dataupload.NewCyberArkClient(certPool, server.Server.URL, tc.authenticate)
			require.NoError(t, err)

			err = cyberArkClient.PostDataReadings(ctx, tc.readings)
			tc.requireFn(t, err)
		})
	}
}

// TestCyberArkClient_PostDataReadings_RealAPI demonstrates that the dataupload code works with the real inventory API.
// An API token is obtained by authenticating with the ARK_USERNAME and ARK_SECRET from the environment.
// ARK_SUBDOMAIN should be your tenant subdomain.
// ARK_PLATFORM_DOMAIN should be either integration-cyberark.cloud or cyberark.cloud
//
// To enable verbose request logging:
//
//	go test ./pkg/internal/cyberark/dataupload/... \
//	  -v -count 1 -run TestCyberArkClient_PostDataReadings_RealAPI -args -testing.v 6
func TestCyberArkClient_PostDataReadings_RealAPI(t *testing.T) {
	platformDomain := os.Getenv("ARK_PLATFORM_DOMAIN")
	subdomain := os.Getenv("ARK_SUBDOMAIN")
	username := os.Getenv("ARK_USERNAME")
	secret := os.Getenv("ARK_SECRET")

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

	var (
		identityClient *identity.Client
		err            error
	)
	if platformDomain == "cyberark.cloud" {
		identityClient, err = identity.New(ctx, subdomain)
	} else {
		discoveryClient := servicediscovery.New(servicediscovery.WithIntegrationEndpoint())
		identityClient, err = identity.NewWithDiscoveryClient(ctx, discoveryClient, subdomain)
	}
	require.NoError(t, err)

	err = identityClient.LoginUsernamePassword(ctx, username, []byte(secret))
	require.NoError(t, err)

	cyberArkClient, err := dataupload.NewCyberArkClient(nil, serviceURL, identityClient.AuthenticateRequest)
	require.NoError(t, err)

	dataReadings := testutil.ParseDataReadings(t, testutil.ReadGZIP(t, "testdata/example-1/datareadings.json.gz"))
	err = cyberArkClient.PostDataReadings(
		ctx,
		dataReadings,
	)
	require.NoError(t, err)
}
