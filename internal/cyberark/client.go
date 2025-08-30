package cyberark

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/jetstack/preflight/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/internal/cyberark/identity"
	"github.com/jetstack/preflight/internal/cyberark/servicediscovery"
)

// ClientConfig holds the configuration needed to initialize a CyberArk client.
type ClientConfig struct {
	Subdomain string
	Username  string
	Secret    string
}

// ClientConfigLoader is a function type that loads and returns a ClientConfig.
type ClientConfigLoader func() (ClientConfig, error)

// ErrMissingEnvironmentVariables is returned when required environment variables are not set.
var ErrMissingEnvironmentVariables = errors.New("missing environment variables: ARK_SUBDOMAIN, ARK_USERNAME, ARK_SECRET")

// LoadClientConfigFromEnvironment loads the CyberArk client configuration from environment variables.
// It expects the following environment variables to be set:
// - ARK_SUBDOMAIN: The CyberArk subdomain to use.
// - ARK_USERNAME: The username for authentication.
// - ARK_SECRET: The secret for authentication.
func LoadClientConfigFromEnvironment() (ClientConfig, error) {
	subdomain := os.Getenv("ARK_SUBDOMAIN")
	username := os.Getenv("ARK_USERNAME")
	secret := os.Getenv("ARK_SECRET")

	if subdomain == "" || username == "" || secret == "" {
		return ClientConfig{}, ErrMissingEnvironmentVariables
	}

	return ClientConfig{
		Subdomain: subdomain,
		Username:  username,
		Secret:    secret,
	}, nil

}

// NewDatauploadClient initializes and returns a new CyberArk Data Upload client.
// It performs service discovery to find the necessary API endpoints and authenticates
// using the provided client configuration.
func NewDatauploadClient(ctx context.Context, httpClient *http.Client, cfg ClientConfig) (*dataupload.CyberArkClient, error) {
	discoveryClient := servicediscovery.New(httpClient)
	serviceMap, err := discoveryClient.DiscoverServices(ctx, cfg.Subdomain)
	if err != nil {
		return nil, err
	}
	identityAPI := serviceMap.Identity.API
	if identityAPI == "" {
		return nil, errors.New("service discovery returned an empty identity API")
	}
	identityClient := identity.New(httpClient, identityAPI, cfg.Subdomain)
	err = identityClient.LoginUsernamePassword(ctx, cfg.Username, []byte(cfg.Secret))
	if err != nil {
		return nil, err
	}
	discoveryAPI := serviceMap.DiscoveryContext.API
	if discoveryAPI == "" {
		return nil, errors.New("service discovery returned an empty discovery API")
	}
	return dataupload.New(httpClient, discoveryAPI, identityClient.AuthenticateRequest), nil
}
