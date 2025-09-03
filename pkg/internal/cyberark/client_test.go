package cyberark_test

import (
	"crypto/x509"
	"errors"
	"testing"

	"github.com/jetstack/venafi-connection-lib/http_client"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/transport"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/pkg/internal/cyberark"
	"github.com/jetstack/preflight/pkg/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/pkg/internal/cyberark/servicediscovery"
	"github.com/jetstack/preflight/pkg/testutil"
	"github.com/jetstack/preflight/pkg/version"

	_ "k8s.io/klog/v2/ktesting/init"
)

// TestCyberArkClient_PutSnapshot_MockAPI demonstrates that NewDatauploadClient works with the mock API.
func TestCyberArkClient_PutSnapshot_MockAPI(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
	ctx := klog.NewContext(t.Context(), logger)

	httpClient := testutil.FakeCyberArk(t)

	cfg := cyberark.ClientConfig{
		Subdomain: servicediscovery.MockDiscoverySubdomain,
		Username:  "test@example.com",
		Secret:    "somepassword",
	}

	cl, err := cyberark.NewDatauploadClient(ctx, httpClient, cfg)
	require.NoError(t, err)

	err = cl.PutSnapshot(ctx, dataupload.Snapshot{
		ClusterID: "success-cluster-id",
	})
	require.NoError(t, err)
}

// TestCyberArkClient_PutSnapshot_RealAPI demonstrates that NewDatauploadClient works with the real inventory API.
//
// An API token is obtained by authenticating with the ARK_USERNAME and ARK_SECRET from the environment.
// ARK_SUBDOMAIN should be your tenant subdomain.
//
// To test against a tenant on the integration platform, also set:
// ARK_DISCOVERY_API=https://platform-discovery.integration-cyberark.cloud/api/public/tenant-discovery
//
// To enable verbose request logging:
//
//	go test ./pkg/internal/cyberark \
//	  -v -count 1 -run TestCyberArkClient_PutSnapshot_RealAPI -args -testing.v 6
func TestCyberArkClient_PutSnapshot_RealAPI(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
	ctx := klog.NewContext(t.Context(), logger)

	var rootCAs *x509.CertPool
	httpClient := http_client.NewDefaultClient(version.UserAgent(), rootCAs)
	httpClient.Transport = transport.NewDebuggingRoundTripper(httpClient.Transport, transport.DebugByContext)

	cfg, err := cyberark.LoadClientConfigFromEnvironment()
	if err != nil {
		if errors.Is(err, cyberark.ErrMissingEnvironmentVariables) {
			t.Skipf("Skipping: %s", err)
		}
		require.NoError(t, err)
	}

	cl, err := cyberark.NewDatauploadClient(ctx, httpClient, cfg)
	require.NoError(t, err)

	err = cl.PutSnapshot(ctx, dataupload.Snapshot{
		ClusterID: "bb068932-c80d-460d-88df-34bc7f3f3297",
	})
	require.NoError(t, err)
}
