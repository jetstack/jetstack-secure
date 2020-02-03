// Package eks provides a datagatherer for EKS.
package eks

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
)

// EKSDataGatherer is a DataGatherer for EKS.
type EKSDataGatherer struct {
	client      *eks.EKS
	clusterName string
}

type EKSDataGathererConfig struct {
	ClusterName string
}

// EKSInfo contains the data retrieved from EKS.
type EKSInfo struct {
	// https://docs.aws.amazon.com/sdk-for-go/api/service/eks/#Cluster
	Cluster *eks.Cluster
}

// NewEKSDataGatherer creates a new EKSDataGatherer for a cluster.
func NewEKSDataGatherer(ctx context.Context, config *EKSDataGathererConfig) *EKSDataGatherer {
	return &EKSDataGatherer{
		client:      eks.New(session.New()),
		clusterName: config.ClusterName,
	}
}

// Fetch retrieves cluster information from EKS.
func (g *EKSDataGatherer) Fetch() (interface{}, error) {
	input := &eks.DescribeClusterInput{
		Name: aws.String(g.clusterName),
	}

	result, err := g.client.DescribeCluster(input)
	if err != nil {
		return nil, err
	}

	return &EKSInfo{
		Cluster: result.Cluster,
	}, nil
}
