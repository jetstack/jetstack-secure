package cyberark

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jetstack/preflight/pkg/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/pkg/internal/cyberark/identity"
	"github.com/jetstack/preflight/pkg/internal/cyberark/servicediscovery"
)

type ClientConfig struct {
	platformDomain string
	subdomain      string
	username       string
	secret         string
}

type ClientConfigLoader func() (ClientConfig, error)

func LoadClientConfigFromEnvironment() (ClientConfig, error) {
	platformDomain := os.Getenv("ARK_PLATFORM_DOMAIN")
	subdomain := os.Getenv("ARK_SUBDOMAIN")
	username := os.Getenv("ARK_USERNAME")
	secret := os.Getenv("ARK_SECRET")

	if platformDomain == "" || subdomain == "" || username == "" || secret == "" {
		return ClientConfig{}, errors.New(
			"missing environment variables: ARK_PLATFORM_DOMAIN, ARK_SUBDOMAIN, ARK_USERNAME, ARK_SECRET")
	}

	return ClientConfig{
		platformDomain: platformDomain,
		subdomain:      subdomain,
		username:       username,
		secret:         secret,
	}, nil

}

func NewDatauploadClient(ctx context.Context, cfg ClientConfig) (*dataupload.CyberArkClient, error) {
	const (
		discoveryContextServiceName = "inventory"
		separator                   = "."
	)

	serviceURL := fmt.Sprintf("https://%s%s%s.%s", cfg.subdomain, separator, discoveryContextServiceName, cfg.platformDomain)

	var (
		identityClient *identity.Client
		err            error
	)
	if cfg.platformDomain == "cyberark.cloud" {
		identityClient, err = identity.New(ctx, cfg.subdomain)
	} else {
		discoveryClient := servicediscovery.New(servicediscovery.WithIntegrationEndpoint())
		identityClient, err = identity.NewWithDiscoveryClient(ctx, discoveryClient, cfg.subdomain)
	}
	if err != nil {
		return nil, err
	}

	err = identityClient.LoginUsernamePassword(ctx, cfg.username, []byte(cfg.secret))
	if err != nil {
		return nil, err
	}

	cyberArkClient, err := dataupload.NewCyberArkClient(nil, serviceURL, identityClient.AuthenticateRequest)
	if err != nil {
		return nil, err
	}
	return cyberArkClient, nil
}
