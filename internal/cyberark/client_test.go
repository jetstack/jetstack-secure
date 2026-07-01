package cyberark_test

import (
	"crypto/tls"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/internal/cyberark"
	"github.com/jetstack/preflight/internal/cyberark/conjur"
	"github.com/jetstack/preflight/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/internal/cyberark/servicediscovery"
	arktesting "github.com/jetstack/preflight/internal/cyberark/testing"
	"github.com/jetstack/preflight/pkg/version"

	_ "k8s.io/klog/v2/ktesting/init"
)

// TestCyberArkClient_PutSnapshot_MockAPI demonstrates that NewDatauploadClient works with the mock API.
func TestCyberArkClient_PutSnapshot_MockAPI(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
	ctx := klog.NewContext(t.Context(), logger)

	const conjurToken = "success-token" // matches dataupload mock's expected bearer token

	jwtFile, err := os.CreateTemp(t.TempDir(), "jwt-*")
	require.NoError(t, err)
	_, err = jwtFile.WriteString("fake-service-account-jwt")
	require.NoError(t, err)
	require.NoError(t, jwtFile.Close())

	conjurSrv, _ := conjur.MockConjurExchangeServer(t, conjurToken)
	t.Cleanup(conjurSrv.Close)

	discoveryContextAPI, _ := dataupload.MockDataUploadServer(t)

	httpClient := servicediscovery.MockDiscoveryServer(t, servicediscovery.Services{
		Identity: servicediscovery.ServiceEndpoint{
			API: conjurSrv.URL,
		},
		DiscoveryContext: servicediscovery.ServiceEndpoint{
			API: discoveryContextAPI,
		},
	})

	cfg := cyberark.ClientConfig{
		Subdomain:   servicediscovery.MockDiscoverySubdomain,
		ServiceID:   "dev-cluster",
		JWTFilePath: jwtFile.Name(),
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

// TestNewDatauploadClient_UsesConjurExchanger asserts that NewDatauploadClient wires
// the conjur exchange as the dataupload RequestAuthenticator. It builds its own
// mock stack so that the Bearer token path can be verified end-to-end.
func TestNewDatauploadClient_UsesConjurExchanger(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
	ctx := klog.NewContext(t.Context(), logger)

	const conjurToken = "success-token" // matches dataupload mock's expected bearer token

	// Write a temp JWT file — NewFileSource reads it during AuthenticateRequest.
	jwtFile, err := os.CreateTemp(t.TempDir(), "jwt-*")
	require.NoError(t, err)
	_, err = jwtFile.WriteString("fake-service-account-jwt")
	require.NoError(t, err)
	require.NoError(t, jwtFile.Close())

	// Stand up a conjur exchange mock that validates the JWT and returns the token.
	conjurSrv, _ := conjur.MockConjurExchangeServer(t, conjurToken)
	defer conjurSrv.Close()

	// Stand up a dataupload mock. It expects Authorization: Bearer success-token.
	// The returned httpClient trusts the TLS cert of the dataupload mock server;
	// it can also reach plain-HTTP servers (the conjur mock) without issue.
	discoveryContextAPI, httpClient := dataupload.MockDataUploadServer(t)

	serviceMap := &servicediscovery.Services{
		Identity: servicediscovery.ServiceEndpoint{
			API: conjurSrv.URL, // conjur exchange endpoint base
		},
		DiscoveryContext: servicediscovery.ServiceEndpoint{
			API: discoveryContextAPI,
		},
	}

	cfg := cyberark.ClientConfig{
		JWTFilePath: jwtFile.Name(),
		ServiceID:   "dev-cluster",
		// Account defaults to "conjur"
	}

	cl, err := cyberark.NewDatauploadClient(ctx, httpClient, serviceMap, "tenant-uuid-1234", cfg)
	require.NoError(t, err)

	err = cl.PutSnapshot(ctx, dataupload.Snapshot{
		ClusterID:    "ffffffff-ffff-ffff-ffff-ffffffffffff",
		AgentVersion: version.PreflightVersion,
	})
	require.NoError(t, err)
}

// TestCyberArkClient_PutSnapshot_RealAPI demonstrates that NewDatauploadClient works with the real inventory API.
//
// An API token is obtained by authenticating with the conjur JWT exchange from the environment.
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
	if strings.ToLower(os.Getenv("ARK_LIVE_TEST")) != "true" {
		t.Skip("set ARK_LIVE_TEST=true to run this test against the live service")
		return
	}

	arktesting.SkipIfNoEnv(t)

	t.Log("This test runs against a live service and has been known to flake. If you see timeout issues it's possible that the test is flaking and it could be unrelated to your changes.")

	logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
	ctx := klog.NewContext(t.Context(), logger)

	// Use a plain http.Client for real API calls; a proper user-agent transport would
	// normally be wired here but the venafi-connection-lib import is avoided to keep
	// this package buildable without private-module credentials in developer environments.
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{},
		},
	}

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
