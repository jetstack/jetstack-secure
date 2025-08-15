package client

import (
	"context"
	"fmt"
	"os"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/pkg/internal/cyberark/identity"
	"github.com/jetstack/preflight/pkg/internal/cyberark/servicediscovery"
)

type MachineHubClient struct{}

func NewMachineHub() Client {
	return &MachineHubClient{}
}

func (o *MachineHubClient) PostDataReadingsWithOptions(ctx context.Context, readings []*api.DataReading, _ Options) error {
	platformDomain := os.Getenv("ARK_PLATFORM_DOMAIN")
	subdomain := os.Getenv("ARK_SUBDOMAIN")
	username := os.Getenv("ARK_USERNAME")
	password := []byte(os.Getenv("ARK_SECRET"))

	const (
		discoveryContextServiceName = "inventory"
		separator                   = "."
	)

	// TODO(wallrj): Maybe get this URL via the service discovery API.
	// https://platform-discovery.integration-cyberark.cloud/api/public/tenant-discovery?allEndpoints=true&bySubdomain=tlskp-test
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
	if err != nil {
		return fmt.Errorf("while creating the CyberArk identity client: %v", err)
	}
	if err := identityClient.LoginUsernamePassword(ctx, username, password); err != nil {
		return fmt.Errorf("while logging in: %v", err)
	}
	duClient, err := dataupload.NewCyberArkClient(nil, serviceURL, identityClient.AuthenticateRequest)
	if err != nil {
		return fmt.Errorf("while creating the CyberArk dataupload client: %v", err)
	}

	return duClient.PostDataReadings(ctx, readings)
}
