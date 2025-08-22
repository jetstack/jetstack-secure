package client

import (
	"context"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/internal/cyberark"
	"github.com/jetstack/preflight/pkg/internal/cyberark/dataupload"
)

type CyberArkClient struct {
	configLoader cyberark.ClientConfigLoader
}

var _ Client = &CyberArkClient{}

func NewCyberArk() (*CyberArkClient, error) {
	configLoader := cyberark.LoadClientConfigFromEnvironment
	_, err := configLoader()
	if err != nil {
		return nil, err
	}
	return &CyberArkClient{
		configLoader: configLoader,
	}, nil
}

// An API token is obtained by authenticating with the ARK_USERNAME and ARK_SECRET from the environment.
// ARK_SUBDOMAIN should be your tenant subdomain.
// ARK_PLATFORM_DOMAIN should be either integration-cyberark.cloud or cyberark.cloud
func (o *CyberArkClient) PostDataReadingsWithOptions(ctx context.Context, readings []*api.DataReading, options Options) error {
	cfg, err := o.configLoader()
	if err != nil {
		return err
	}
	datauploadClient, err := cyberark.NewDatauploadClient(ctx, cfg)
	if err != nil {
		return err
	}

	err = datauploadClient.PostDataReadingsWithOptions(ctx, api.DataReadingsPost{}, dataupload.Options{
		ClusterName: "bb068932-c80d-460d-88df-34bc7f3f3297",
	})
	if err != nil {
		return err
	}
	return nil
}
