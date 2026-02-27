package cyberark_test

import (
	"crypto/x509"
	"testing"

	"github.com/jetstack/venafi-connection-lib/http_client"
	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/internal/cyberark"
	"github.com/jetstack/preflight/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/internal/cyberark/servicediscovery"
	arktesting "github.com/jetstack/preflight/internal/cyberark/testing"
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

	discoveryClient := servicediscovery.New(httpClient, cfg.Subdomain)

	serviceMap, tenantUUID, err := discoveryClient.DiscoverServices(t.Context())
	if err != nil {
		t.Fatalf("failed to discover mock services: %v", err)
	}

	cl, err := cyberark.NewDatauploadClient(ctx, httpClient, serviceMap, tenantUUID, cfg)
	require.NoError(t, err)

	err = cl.PutSnapshot(ctx, dataupload.Snapshot{
		ClusterID:    "ffffffff-ffff-ffff-ffff-ffffffffffff",
		AgentVersion: version.PreflightVersion,
	})

	require.NoError(t, err)
}

// TestCyberArkClient_PutSnapshot_RealAPI demonstrates that NewDatauploadClient works with the real inventory API.
//
// An API token is obtained by authenticating with the ARK_USERNAME and ARK_SECRET from the environment.
// ARK_SUBDOMAIN should be your tenant subdomain.
//
// To test against a tenant on the integration platform, also set:
// ARK_DISCOVERY_API=https://platform-discovery.integration-cyberark.cloud/
//
// To enable verbose request logging:
//
//	go test ./internal/cyberark \
//	  -v -count 1 -run TestCyberArkClient_PutSnapshot_RealAPI -args -testing.v 6
func TestCyberArkClient_PutSnapshot_RealAPI(t *testing.T) {
	arktesting.SkipIfNoEnv(t)

	t.Log("This test runs against a live service and has been known to flake. If you see timeout issues it's possible that the test is flaking and it could be unrelated to your changes.")

	logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
	ctx := klog.NewContext(t.Context(), logger)

	var rootCAs *x509.CertPool
	httpClient := http_client.NewDefaultClient(version.UserAgent(), rootCAs)

	cfg, err := cyberark.LoadClientConfigFromEnvironment()
	require.NoError(t, err)

	discoveryClient := servicediscovery.New(httpClient, cfg.Subdomain)

	serviceMap, tenantUUID, err := discoveryClient.DiscoverServices(t.Context())
	if err != nil {
		t.Fatalf("failed to discover services: %v", err)
	}

	cl, err := cyberark.NewDatauploadClient(ctx, httpClient, serviceMap, tenantUUID, cfg)
	require.NoError(t, err)

	err = cl.PutSnapshot(ctx, dataupload.Snapshot{
		ClusterID:    "ffffffff-ffff-ffff-ffff-ffffffffffff",
		AgentVersion: version.PreflightVersion,
	})

	require.NoError(t, err)
}
