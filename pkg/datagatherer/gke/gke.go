// Package gke provides a datagatherer for GKE.
package gke

import (
	"context"
	"fmt"
	"log"
	"strings"

	"golang.org/x/oauth2/google"
	container "google.golang.org/api/container/v1"
	"google.golang.org/api/option"
)

// Config is the configuration for a GKE DataGatherer.
type Config struct {
	// Cluster contains the details about to identify the cluster to gather information from.
	Cluster *Cluster
	// CredentialsPath is the path to the JSON file containing the credentials to authenticate against the GKE API.
	CredentialsPath string
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	errs := []string{}
	emptyMsg := "%s should be a non empty string"

	if c.Cluster.Project == "" {
		errs = append(errs, fmt.Sprintf(emptyMsg, "Cluster.Project"))
	}
	if c.Cluster.Name == "" {
		errs = append(errs, fmt.Sprintf(emptyMsg, "Cluster.Name"))
	}
	if c.Cluster.Zone != "" {
		if c.Cluster.Location != "" {
			errs = append(errs, "Cluster.Location and Cluster.Zone cannot be used at the same time, use only Location")
		}
		errs = append(errs, "Cluster.Zone is deprecated and will be deleted soon. Please use Cluster.Location instead")
	} else if c.Cluster.Location == "" {
		errs = append(errs, fmt.Sprintf(emptyMsg, "Cluster.Location"))
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("invalid configuration: %s", strings.Join(errs, ";"))
}

// Cluster holds details about the cluster required to query it using the API.
type Cluster struct {
	// Project is the Google Cloud Platform project the cluster belongs to.
	Project string
	// Deprecated: Zone of the cluster. Use Location instead.
	Zone string
	// Name is the identifier of the cluster.
	Name string
	// Location is the location of the cluster.
	Location string
}

// DataGatherer is a DataGatherer for GKE.
type DataGatherer struct {
	ctx             context.Context
	cluster         *Cluster
	credentialsPath string
}

// Info contains the data retrieved from GKE.
type Info struct {
	Cluster *container.Cluster
}

// NewDataGatherer creates a new DataGatherer for a cluster.
func NewDataGatherer(ctx context.Context, cfg *Config) (*DataGatherer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &DataGatherer{
		ctx:             ctx,
		cluster:         cfg.Cluster,
		credentialsPath: cfg.CredentialsPath,
	}, nil
}

// Fetch retrieves cluster information from GKE.
func (g *DataGatherer) Fetch() (interface{}, error) {
	var credsOpt option.ClientOption
	if len(g.credentialsPath) == 0 {
		log.Println("Credentials path for GKE was not provided. Attempting to use GCP Workload Identity.")
		// Connect to the Google Cloud Platform API using Workload Identity
		creds, err := google.FindDefaultCredentials(g.ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain credentials for Google Cloud Platform: try to run 'gcloud auth application-default login' to login to your account")
		}

		credsOpt = option.WithCredentials(creds)
	} else {
		credsOpt = option.WithCredentialsFile(g.credentialsPath)
	}

	containerService, err := container.NewService(g.ctx, credsOpt)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Google Cloud Platform container API: %v", err)
	}

	var cluster *container.Cluster
	if len(g.cluster.Location) > 0 {
		cluster, err = containerService.Projects.Locations.Clusters.Get(fmt.Sprintf("projects/%s/locations/%s/clusters/%s", g.cluster.Project, g.cluster.Location, g.cluster.Name)).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to get GKE cluster information (project: %s, location: %s, cluster: %s): %v", g.cluster.Project, g.cluster.Location, g.cluster.Name, err)
		}
	} else {
		cluster, err = containerService.Projects.Zones.Clusters.Get(g.cluster.Project, g.cluster.Zone, g.cluster.Name).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to get GKE cluster information (project: %s, zone: %s, cluster: %s): %v", g.cluster.Project, g.cluster.Zone, g.cluster.Name, err)
		}
	}

	return &Info{
		Cluster: cluster,
	}, nil
}
