package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/internal/cyberark"
	"github.com/jetstack/preflight/pkg/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/pkg/version"
)

// CyberArkClient is a client for publishing data readings to CyberArk's discoverycontext API.
type CyberArkClient struct {
	configLoader cyberark.ClientConfigLoader
	httpClient   *http.Client
}

var _ Client = &CyberArkClient{}

// NewCyberArk initializes a CyberArk client using configuration from environment variables.
// It requires an HTTP client to be provided, which will be used for making requests.
// The environment variables ARK_SUBDOMAIN, ARK_USERNAME, and ARK_SECRET must be set for authentication.
// If the configuration is invalid or missing, an error is returned.
func NewCyberArk(httpClient *http.Client) (*CyberArkClient, error) {
	configLoader := cyberark.LoadClientConfigFromEnvironment
	_, err := configLoader()
	if err != nil {
		return nil, err
	}
	return &CyberArkClient{
		configLoader: configLoader,
		httpClient:   httpClient,
	}, nil
}

// PostDataReadingsWithOptions uploads data readings to CyberArk.
// It initializes a data upload client with the configured HTTP client and credentials,
// then uploads a snapshot.
// The supplied Options are not used by this publisher.
func (o *CyberArkClient) PostDataReadingsWithOptions(ctx context.Context, readings []*api.DataReading, _ Options) error {
	cfg, err := o.configLoader()
	if err != nil {
		return err
	}
	datauploadClient, err := cyberark.NewDatauploadClient(ctx, o.httpClient, cfg)
	if err != nil {
		return fmt.Errorf("while initializing data upload client: %s", err)
	}

	err = datauploadClient.PutSnapshot(ctx, dataupload.Snapshot{
		// Temporary hard coded cluster ID.
		// TODO(wallrj): The clusterID will eventually be extracted from the supplied readings.
		ClusterID:    "success-cluster-id",
		AgentVersion: version.PreflightVersion,
	})
	if err != nil {
		return fmt.Errorf("while uploading snapshot: %s", err)
	}
	return nil
}
