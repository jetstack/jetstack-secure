package k8s

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// PodsDataGatherer fetches lists of core/v1.Pods from the Kubernetes API.
type PodsDataGatherer struct {
	*genericGatherer
}


// NewPodsDataGatherer creates a new PodsDataGatherer.
func NewPodsDataGatherer(client dynamic.Interface) *PodsDataGatherer {
	return &PodsDataGatherer{
		genericGatherer: &genericGatherer{
			cl:                   client,
			groupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		},
	}
}

// Fetch retrieves Pods information from the Kubernetes API.
func (g *PodsDataGatherer) Fetch() (interface{}, error) {
	return g.genericGatherer.Fetch()
}
