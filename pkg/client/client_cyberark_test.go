package client_test

import (
	"crypto/x509"
	"errors"
	"testing"

	"github.com/jetstack/venafi-connection-lib/http_client"
	"github.com/stretchr/testify/require"
	k8sversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/internal/cyberark"
	"github.com/jetstack/preflight/internal/cyberark/servicediscovery"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/testutil"
	"github.com/jetstack/preflight/pkg/version"

	_ "k8s.io/klog/v2/ktesting/init"
)

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

		readings := fakeReadings()
		err = c.PostDataReadingsWithOptions(ctx, readings, client.Options{})
		require.NoError(t, err)
	})
}

// TestCyberArkClient_PostDataReadingsWithOptions_RealAPI demonstrates that the
// dataupload code works with the real CyberArk APIs.
//
// To enable verbose request logging:
//
//	go test ./internal/cyberark/dataupload/... \
//	  -v -count 1 -run TestCyberArkClient_PostDataReadingsWithOptions_RealAPI -args -testing.v 6
func TestCyberArkClient_PostDataReadingsWithOptions_RealAPI(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
		ctx := klog.NewContext(t.Context(), logger)

		var rootCAs *x509.CertPool
		httpClient := http_client.NewDefaultClient(version.UserAgent(), rootCAs)

		c, err := client.NewCyberArk(httpClient)
		if err != nil {
			if errors.Is(err, cyberark.ErrMissingEnvironmentVariables) {
				t.Skipf("Skipping: %s", err)
			}
			require.NoError(t, err)
		}
		readings := fakeReadings()
		err = c.PostDataReadingsWithOptions(ctx, readings, client.Options{})
		require.NoError(t, err)
	})
}

// defaultDynamicDatagathererNames is the list of dynamic datagatherers that
// are included in the defaultExtractorFunctions map in client_cyberark.go.
// This is used by fakeReadings to generate empty readings for all the
// dynamic datagatherers.
var defaultDynamicDatagathererNames = []string{
	"ark/secrets",
	"ark/serviceaccounts",
	"ark/configmaps",
	"ark/esoexternalsecrets",
	"ark/esosecretstores",
	"ark/esoclusterexternalsecrets",
	"ark/esoclustersecretstores",
	"ark/roles",
	"ark/clusterroles",
	"ark/rolebindings",
	"ark/clusterrolebindings",
	"ark/jobs",
	"ark/cronjobs",
	"ark/deployments",
	"ark/statefulsets",
	"ark/daemonsets",
	"ark/pods",
}

// fakeReadings returns a set of fake readings that includes a discovery reading
// and empty readings for all the default dynamic datagatherers.
func fakeReadings() []*api.DataReading {
	readings := make([]*api.DataReading, len(defaultDynamicDatagathererNames))

	for i, name := range defaultDynamicDatagathererNames {
		readings[i] = &api.DataReading{
			DataGatherer: name,
			Data:         &api.DynamicData{},
		}
	}

	return append([]*api.DataReading{
		{
			DataGatherer: "ark/oidc",
			Data: &api.OIDCDiscoveryData{
				OIDCConfigError: "Failed to fetch /.well-known/openid-configuration: 404 Not Found",
				JWKSError:       "Failed to fetch /openid/v1/jwks: 404 Not Found",
			},
		},
		{
			DataGatherer: "ark/discovery",
			Data: &api.DiscoveryData{
				ClusterID: "ffffffff-ffff-ffff-ffff-ffffffffffff",
				ServerVersion: &k8sversion.Info{
					GitVersion: "v1.21.0",
				},
			},
		},
	}, readings...)
}
