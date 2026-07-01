package cyberark_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/internal/cyberark"
	"github.com/jetstack/preflight/internal/cyberark/conjur"
	"github.com/jetstack/preflight/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/internal/cyberark/identity"
	"github.com/jetstack/preflight/internal/cyberark/servicediscovery"

	_ "k8s.io/klog/v2/ktesting/init"
)

// The agent supports two coexisting auth methods (the product is GA). These
// tests pin the selection rule in NewDatauploadClient / selectAuthenticator:
//   - ServiceID set                → Conjur JWT exchange
//   - else Username+Secret present → legacy username/password
//   - both set                     → Conjur wins
//   - neither                      → ErrNoAuthMethod
func TestNewDatauploadClient_AuthMethodSelection(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
	ctx := klog.NewContext(t.Context(), logger)

	const conjurToken = "success-token" // matches dataupload mock's expected bearer token

	writeJWT := func(t *testing.T) string {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "jwt-*")
		require.NoError(t, err)
		_, err = f.WriteString("fake-service-account-jwt")
		require.NoError(t, err)
		require.NoError(t, f.Close())
		return f.Name()
	}

	// stack builds a service map whose Identity API points at the given endpoint
	// and whose DiscoveryContext points at a dataupload mock. The dataupload mock
	// requires Authorization: Bearer success-token.
	stack := func(t *testing.T, identityAPI string) *servicediscovery.Services {
		t.Helper()
		discoveryContextAPI, _ := dataupload.MockDataUploadServer(t)
		return &servicediscovery.Services{
			Identity:         servicediscovery.ServiceEndpoint{API: identityAPI},
			DiscoveryContext: servicediscovery.ServiceEndpoint{API: discoveryContextAPI},
		}
	}

	t.Run("serviceID set -> conjur path", func(t *testing.T) {
		conjurSrv, _ := conjur.MockConjurExchangeServer(t, conjurToken)
		t.Cleanup(conjurSrv.Close)

		cfg := cyberark.ClientConfig{
			ServiceID:   "dev-cluster",
			JWTFilePath: writeJWT(t),
		}
		_, err := cyberark.NewDatauploadClient(ctx, conjurSrv.Client(), stack(t, conjurSrv.URL), "tenant", cfg)
		require.NoError(t, err)
	})

	t.Run("username/password only -> identity path", func(t *testing.T) {
		identityURL, httpClient := identity.MockIdentityServer(t)

		cfg := cyberark.ClientConfig{
			Subdomain: "tenant-sub",
			Username:  identity.MockSuccessUser,
			Secret:    []byte(identity.MockSuccessPassword),
		}
		// Login happens during construction; success proves the UP path ran.
		_, err := cyberark.NewDatauploadClient(ctx, httpClient, stack(t, identityURL), "tenant", cfg)
		require.NoError(t, err)
	})

	t.Run("both set -> conjur wins", func(t *testing.T) {
		conjurSrv, _ := conjur.MockConjurExchangeServer(t, conjurToken)
		t.Cleanup(conjurSrv.Close)

		cfg := cyberark.ClientConfig{
			ServiceID:   "dev-cluster",
			JWTFilePath: writeJWT(t),
			// UP creds present too — must be ignored. Deliberately bogus so that
			// if the identity path were taken, login would fail.
			Username: "should-not-be-used@example.com",
			Secret:   []byte("wrong-password"),
		}
		_, err := cyberark.NewDatauploadClient(ctx, conjurSrv.Client(), stack(t, conjurSrv.URL), "tenant", cfg)
		require.NoError(t, err) // conjur path used; bogus UP creds never exercised
	})

	t.Run("neither set -> ErrNoAuthMethod", func(t *testing.T) {
		cfg := cyberark.ClientConfig{Subdomain: "tenant-sub"}
		_, err := cyberark.NewDatauploadClient(ctx, &http.Client{}, stack(t, "https://identity.example"), "tenant", cfg)
		require.ErrorIs(t, err, cyberark.ErrNoAuthMethod)
	})
}
