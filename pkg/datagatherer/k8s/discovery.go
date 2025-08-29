package k8s

import (
	"context"
	"fmt"

	"k8s.io/client-go/discovery"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/datagatherer"
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
	cl *discovery.DiscoveryClient
}

func (g *DataGathererDiscovery) Run(ctx context.Context) error {
	// no async functionality, see Fetch
	return nil
}

func (g *DataGathererDiscovery) WaitForCacheSync(ctx context.Context) error {
	// no async functionality, see Fetch
	return nil
}

// Fetch will fetch discovery data from the apiserver, or return an error
func (g *DataGathererDiscovery) Fetch() (interface{}, int, error) {
	serverVersion, err := g.cl.ServerVersion()
	if err != nil {
		return nil, -1, fmt.Errorf("failed to get server version: %v", err)
	}

	return &api.DiscoveryData{
		ServerVersion: serverVersion,
	}, 1, nil
}
