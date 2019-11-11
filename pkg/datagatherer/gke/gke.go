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

// GKEInfo contains the data retrieved from GKE.
type GKEInfo struct {
	Cluster *container.Cluster
}

// Cluster holds details about the cluster required to query it using the API.
type Cluster struct {
	Project string
	Zone    string
	Name    string
}

// NewGKEDataGatherer creates a new GKEDataGatherer for a cluster.
func NewGKEDataGatherer(ctx context.Context, cluster *Cluster, credsPath string) *GKEDataGatherer {
	return &GKEDataGatherer{
		ctx:             ctx,
		cluster:         cluster,
		credentialsPath: credsPath,
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

	cluster, err := containerService.Projects.Zones.Clusters.Get(g.cluster.Project, g.cluster.Zone, g.cluster.Name).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get GKE cluster information (project: %s, zone: %s, cluster: %s): %v", g.cluster.Project, g.cluster.Zone, g.cluster.Name, err)
	}

	return &GKEInfo{
		Cluster: cluster,
	}, nil
}
