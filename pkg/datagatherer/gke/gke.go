// Package gke provides a datagatherer for GKE.
package gke

import (
	"context"
	"fmt"
	"log"

	"golang.org/x/oauth2/google"
	container "google.golang.org/api/container/v1"
	"google.golang.org/api/option"
)

// GKEDataGatherer is a DataGatherer for GKE.
type GKEDataGatherer struct {
	ctx             context.Context
	cluster         *Cluster
	credentialsPath string
}

type GKEDataGathererConfig struct {
	Project     string `mapstructure:"project"`
	Location    string `mapstructure:"location"`
	Zone        string `mapstructure:"location"` // Deprecated
	Cluster     string `mapstructure:"cluster"`
	Credentials string `mapstructure:"credentials"`
	DataPath    string `mapstructure:"data-path"`
}

// GKEInfo contains the data retrieved from GKE.
type GKEInfo struct {
	Cluster *container.Cluster
}

// Cluster holds details about the cluster required to query it using the API.
type Cluster struct {
	Project string
	// Zone is deprecated, since now Location works for both Zones and regions
	Zone     string
	Name     string
	Location string
}

// NewGKEDataGatherer creates a new GKEDataGatherer for a cluster.
func NewGKEDataGatherer(ctx context.Context, config *GKEDataGathererConfig) *GKEDataGatherer {
	return &GKEDataGatherer{
		ctx: ctx,
		cluster: &Cluster{
			Project:  config.Project,
			Zone:     config.Zone,
			Name:     config.Cluster,
			Location: config.Location,
		},
		credentialsPath: config.Credentials,
	}
}

// Fetch retrieves cluster information from GKE.
func (g *GKEDataGatherer) Fetch() (interface{}, error) {
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

	return &GKEInfo{
		Cluster: cluster,
	}, nil
}
