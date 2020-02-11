package k8s

import (
	"context"
	"log"

	"github.com/jetstack/preflight/pkg/pathutils"
	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PodsDataGatherer is a DataGatherer for GKE.
type PodsDataGatherer struct {
	client *kubernetes.Clientset
}

type PodsDataGathererConfig struct {
	KubeConfig string `mapstructure:"kubeconfig,omitempty"`
}

// PodsInfo contains Pods information retrieved from the Kubernetes API.
type PodsInfo *core.PodList

// NewPodsDataGatherer creates a new PodsDataGatherer.
func NewPodsDataGatherer(ctx context.Context, config *PodsDataGathererConfig) *PodsDataGatherer {
	k8sClient, err := NewClient(pathutils.ExpandHome(config.KubeConfig))
	if err != nil {
		log.Fatalf("Cannot create k8s client: %+v", err)
	}
	return &PodsDataGatherer{
		client: k8sClient,
	}
}

// Fetch retrieves Pods information from the Kubernetes API.
func (g *PodsDataGatherer) Fetch() (interface{}, error) {
	k8sPodAPI := g.client.CoreV1().Pods("")

	pods, err := k8sPodAPI.List(meta.ListOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return pods, nil
}
