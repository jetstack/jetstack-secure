// Package eks provides a datagatherer for AWS EKS.
package eks

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
)

// Config is the configuration for an EKS DataGatherer.
type Config struct {
	// ClusterName is the ID of the cluster in EKS.
	ClusterName string
}

// validate validates the configuration.
func (c *Config) validate() error {
	if c.ClusterName == "" {
		return fmt.Errorf("invalid configuration: ClusterName cannot be empty")
	}
	return nil
}

// NewDataGatherer creates a new EKS DataGatherer. It performs a config validation.
func (c *Config) NewDataGatherer(ctx context.Context) (*DataGatherer, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	return &DataGatherer{
		client:      eks.New(session.New()),
		clustername: c.ClusterName,
	}, nil
}

// DataGatherer is a data-gatherer for EKS.
type DataGatherer struct {
	client      *eks.EKS
	clustername string
}

// Info contains the data retrieved from EKS.
type Info struct {
	// Cluster represents an EKS cluster: https://docs.aws.amazon.com/sdk-for-go/api/service/eks/#Cluster
	Cluster *eks.Cluster
}

// Fetch retrieves cluster information from EKS.
func (g *DataGatherer) Fetch() (interface{}, error) {
	input := &eks.DescribeClusterInput{
		Name: aws.String(g.clustername),
	}

	result, err := g.client.DescribeCluster(input)
	if err != nil {
		return nil, err
	}

	return &Info{
		Cluster: result.Cluster,
	}, nil
}
