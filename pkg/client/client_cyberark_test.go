package client_test

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/jetstack/venafi-connection-lib/http_client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/transport"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/internal/cyberark"
	"github.com/jetstack/preflight/pkg/internal/cyberark/servicediscovery"
	"github.com/jetstack/preflight/pkg/testutil"
	"github.com/jetstack/preflight/pkg/version"

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

// TestCyberArkClient_PostDataReadingsWithOptions_MockAPI demonstrates that the
// dataupload code works with the mock CyberArk APIs.
// The environment variables are chosen to match those expected by the mock
// server.
func TestCyberArkClient_PostDataReadingsWithOptions_MockAPI(t *testing.T) {
	t.Setenv("ARK_SUBDOMAIN", servicediscovery.MockDiscoverySubdomain)
	t.Setenv("ARK_USERNAME", "test@example.com")
	t.Setenv("ARK_SECRET", "somepassword")
	t.Run("success", func(t *testing.T) {
		logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
		ctx := klog.NewContext(t.Context(), logger)

		httpClient := testutil.FakeCyberArk(t)

		c, err := client.NewCyberArk(httpClient)
		require.NoError(t, err)

		readings := []*api.DataReading{
			genArkNamespacesDataReading("success-cluster-id"),
		}
		err = c.PostDataReadingsWithOptions(ctx, readings, client.Options{})
		require.NoError(t, err)
	})
}

// TestCyberArkClient_PostDataReadingsWithOptions_RealAPI demonstrates that the
// dataupload code works with the real CyberArk APIs.
//
// To enable verbose request logging:
//
//	go test ./pkg/internal/cyberark/dataupload/... \
//	  -v -count 1 -run TestCyberArkClient_PostDataReadingsWithOptions_RealAPI -args -testing.v 6
func TestCyberArkClient_PostDataReadingsWithOptions_RealAPI(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
		ctx := klog.NewContext(t.Context(), logger)

		var rootCAs *x509.CertPool
		httpClient := http_client.NewDefaultClient(version.UserAgent(), rootCAs)
		httpClient.Transport = transport.NewDebuggingRoundTripper(httpClient.Transport, transport.DebugByContext)

		c, err := client.NewCyberArk(httpClient)
		if err != nil {
			if errors.Is(err, cyberark.ErrMissingEnvironmentVariables) {
				t.Skipf("Skipping: %s", err)
			}
			require.NoError(t, err)
		}
		readings := testutil.ParseDataReadings(t, testutil.ReadGZIP(t, "testdata/example-1/datareadings.json.gz"))
		err = c.PostDataReadingsWithOptions(ctx, readings, client.Options{})
		require.NoError(t, err)
	})
}

func TestConvertDataReadingsToCyberarkSnapshot(t *testing.T) {
	dataReadings := testutil.ParseDataReadings(t, testutil.ReadGZIP(t, "testdata/example-1/datareadings.json.gz"))
	snapshot, err := client.ConvertDataReadingsToCyberarkSnapshot(dataReadings)
	require.NoError(t, err)

	actualSnapshotBytes, err := json.MarshalIndent(snapshot, "", "  ")
	require.NoError(t, err)

	goldenFilePath := "testdata/example-1/snapshot.json.gz"
	if _, update := os.LookupEnv("UPDATE_GOLDEN_FILES"); update {
		testutil.WriteGZIP(t, goldenFilePath, actualSnapshotBytes)
	} else {
		expectedSnapshotBytes := testutil.ReadGZIP(t, goldenFilePath)
		assert.JSONEq(t, string(expectedSnapshotBytes), string(actualSnapshotBytes))
	}
}
