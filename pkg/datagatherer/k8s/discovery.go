package k8s

import (
	"context"
	"fmt"

	"github.com/jetstack/preflight/pkg/datagatherer"
	"k8s.io/client-go/discovery"
)

// ConfigDiscovery contains the configuration for the k8s-discovery data-gatherer
type ConfigDiscovery struct {
	// KubeConfigPath is the path to the kubeconfig file. If empty, will assume it runs in-cluster.
	KubeConfigPath string `yaml:"kubeconfig"`
}

// UnmarshalYAML unmarshals the Config resolving GroupVersionResource.
func (c *ConfigDiscovery) UnmarshalYAML(unmarshal func(interface{}) error) error {
	aux := struct {
		KubeConfigPath string `yaml:"kubeconfig"`
	}{}
	err := unmarshal(&aux)
	if err != nil {
		return err
	}

	c.KubeConfigPath = aux.KubeConfigPath

	return nil
}

// NewDataGatherer constructs a new instance of the generic K8s data-gatherer for the provided
// GroupVersionResource.
func (c *ConfigDiscovery) NewDataGatherer(ctx context.Context) (datagatherer.DataGatherer, error) {
	cl, err := NewDiscoveryClient(c.KubeConfigPath)
	if err != nil {
		return nil, err
	}

	return &DataGathererDiscovery{cl: cl}, nil
}

// DataGathererDiscovery stores the config for a k8s-discovery datagatherer
type DataGathererDiscovery struct {
	// The 'discovery' client used for fetching data.
	cl discovery.DiscoveryClient
}

// Run starts the data gatherer's informers for resource collection.
// Returns error if the data gatherer informer wasn't initialized
func (g *DataGathererDiscovery) Run(stopCh <-chan struct{}) error {
	return fmt.Errorf("data gatherer's informer was not initialized")
}

// WaitForCacheSync waits for the data gatherer's informers cache to sync.
func (g *DataGathererDiscovery) WaitForCacheSync(stopCh <-chan struct{}) error {
	return fmt.Errorf("timed out waiting for caches to sync")
}

func (g *DataGathererDiscovery) Equals(old datagatherer.DataGatherer) bool {
	return false
}

// Fetch will fetch discovery data from the apiserver, or return an error
func (g *DataGathererDiscovery) Fetch() (interface{}, error) {
	data, err := g.cl.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %v", err)
	}

	response := map[string]interface{}{
		// data has type Info: https://godoc.org/k8s.io/apimachinery/pkg/version#Info
		"server_version": data,
	}

	return response, nil
}
