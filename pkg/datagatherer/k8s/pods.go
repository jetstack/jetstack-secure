package k8s

import (
	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PodsDataGatherer is a DataGatherer for GKE.
type PodsDataGatherer struct {
	client *kubernetes.Clientset
}

// PodsInfo contains Pods information retrieved from the Kubernetes API.
type PodsInfo *core.PodList

// NewPodsDataGatherer creates a new PodsDataGatherer.
func NewPodsDataGatherer(client *kubernetes.Clientset) *PodsDataGatherer {
	return &PodsDataGatherer{
		client: client,
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
