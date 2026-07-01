package cyberark

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/internal/cyberark/conjur"
	"github.com/jetstack/preflight/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/internal/cyberark/identity"
	"github.com/jetstack/preflight/internal/cyberark/jwtsource"
	"github.com/jetstack/preflight/internal/cyberark/servicediscovery"
)

// ClientConfig holds the configuration needed to initialize a CyberArk client.
//
// Two authentication methods coexist (the product is GA; existing installs use
// username/password). The active method is selected by config presence, see
// selectAuthenticator: a Conjur authn-jwt ServiceID, when set, takes precedence
// over username/password.
type ClientConfig struct {
	Subdomain string

	// Conjur JWT exchange (preferred for new installs).
	ServiceID   string // authn-jwt service id (POC: per-cluster, e.g. "dev-cluster")
	Account     string // POC: "conjur"
	JWTSource   string // "file" (POC) | "spiffe" (deferred)
	JWTFilePath string // default jwtsource.DefaultTokenPath

	// Legacy CyberArk Identity username/password (backward compatibility).
	// Sourced from ARK_USERNAME / ARK_SECRET. Used only when ServiceID is unset.
	Username string
	Secret   []byte
}

// ClientConfigLoader is a function type that loads and returns a ClientConfig.
type ClientConfigLoader func() (ClientConfig, error)

// ErrMissingEnvironmentVariables is returned when required environment variables are not set.
var ErrMissingEnvironmentVariables = errors.New("missing environment variables: ARK_SUBDOMAIN")

// ErrNoAuthMethod is returned when neither a Conjur service-id nor
// username/password credentials are configured.
var ErrNoAuthMethod = errors.New("no CyberArk authentication method configured: set config.cyberark.service_id (Conjur JWT) or ARK_USERNAME + ARK_SECRET (legacy username/password)")

// LoadClientConfigFromEnvironment loads the CyberArk client configuration from environment variables.
// It expects the following environment variable to be set:
//   - ARK_SUBDOMAIN: The CyberArk subdomain to use (required).
//
// It also reads the optional legacy username/password credentials:
//   - ARK_USERNAME, ARK_SECRET: used only when no Conjur service-id is configured.
//
// Behavioral keys (ServiceID, Account, JWTSource, JWTFilePath) are set by the
// caller from the agent YAML config (config.cyberark.*).
func LoadClientConfigFromEnvironment() (ClientConfig, error) {
	subdomain := os.Getenv("ARK_SUBDOMAIN")
	if subdomain == "" {
		return ClientConfig{}, ErrMissingEnvironmentVariables
	}
	cfg := ClientConfig{
		Subdomain: subdomain,
		Username:  os.Getenv("ARK_USERNAME"),
	}
	if secret := os.Getenv("ARK_SECRET"); secret != "" {
		cfg.Secret = []byte(secret)
	}
	return cfg, nil
}

// selectAuthenticator builds the request authenticator for the configured auth
// method and returns it together with the discovery-context API endpoint.
//
// Selection (backward compatible — the product is GA):
//   - ServiceID set                  → Conjur JWT exchange (preferred).
//   - else Username+Secret present   → legacy CyberArk Identity UP login.
//   - neither                        → ErrNoAuthMethod.
//
// When both are configured, ServiceID wins (a migrating install can set the
// service-id without first removing its old credentials) and a warning is logged.
func selectAuthenticator(ctx context.Context, httpClient *http.Client, serviceMap *servicediscovery.Services, cfg ClientConfig) (identity.RequestAuthenticator, error) {
	identityAPI := serviceMap.Identity.API
	if identityAPI == "" {
		return nil, errors.New("service discovery returned an empty identity API")
	}

	hasConjur := cfg.ServiceID != ""
	hasUP := cfg.Username != "" && len(cfg.Secret) > 0

	switch {
	case hasConjur:
		if hasUP {
			klog.FromContext(ctx).Info("both Conjur service_id and ARK_USERNAME/ARK_SECRET are set; using the Conjur JWT exchange and ignoring the username/password credentials")
		}
		if cfg.JWTSource != "" && cfg.JWTSource != "file" {
			return nil, fmt.Errorf("jwt_source %q not supported in POC (only 'file')", cfg.JWTSource)
		}
		account := cfg.Account
		if account == "" {
			account = "conjur" // POC default; open item #2
		}
		src := jwtsource.NewFileSource(cfg.JWTFilePath)
		conjurClient := conjur.New(httpClient, identityAPI, cfg.ServiceID, account, src)
		return conjurClient.AuthenticateRequest, nil

	case hasUP:
		identityClient := identity.New(httpClient, identityAPI, cfg.Subdomain)
		if err := identityClient.LoginUsernamePassword(ctx, cfg.Username, cfg.Secret); err != nil {
			return nil, fmt.Errorf("CyberArk Identity username/password login failed: %w", err)
		}
		return identityClient.AuthenticateRequest, nil

	default:
		return nil, ErrNoAuthMethod
	}
}

// NewRequestAuthenticator selects and builds the configured request
// authenticator (Conjur JWT exchange or legacy username/password). Exposed for
// other consumers (e.g. envelope key fetching) that need the same auth seam
// without a dataupload client.
func NewRequestAuthenticator(ctx context.Context, httpClient *http.Client, serviceMap *servicediscovery.Services, cfg ClientConfig) (identity.RequestAuthenticator, error) {
	return selectAuthenticator(ctx, httpClient, serviceMap, cfg)
}

// NewDatauploadClient initializes and returns a new CyberArk Data Upload client.
// It performs service discovery to find the necessary API endpoints and
// authenticates using whichever method is configured (Conjur JWT exchange or
// legacy username/password — see selectAuthenticator).
func NewDatauploadClient(ctx context.Context, httpClient *http.Client, serviceMap *servicediscovery.Services, tenantUUID string, cfg ClientConfig) (*dataupload.CyberArkClient, error) {
	discoveryAPI := serviceMap.DiscoveryContext.API
	if discoveryAPI == "" {
		return nil, errors.New("service discovery returned an empty discovery API")
	}

	authenticate, err := selectAuthenticator(ctx, httpClient, serviceMap, cfg)
	if err != nil {
		return nil, err
	}
	return dataupload.New(httpClient, discoveryAPI, tenantUUID, authenticate), nil
}
