package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// It gets the UID of the 'kube-system' namespace to use as the cluster ID, once at startup.
// The UID is assumed to be stable for the lifetime of the cluster.
// - https://github.com/kubernetes/kubernetes/issues/77487#issuecomment-489786023
func (c *ConfigDiscovery) NewDataGatherer(ctx context.Context) (datagatherer.DataGatherer, error) {
	cl, err := NewDiscoveryClient(c.KubeConfigPath)
	if err != nil {
		return nil, err
	}
	cs, err := NewClientSet(c.KubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("while creating new clientset: %s", err)
	}
	kubesystemNS, err := cs.CoreV1().Namespaces().Get(ctx, "kube-system", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("while getting the kube-system namespace: %s", err)
	}
	return &DataGathererDiscovery{
		cl:        cl,
		clusterID: string(kubesystemNS.UID),
	}, nil
}

// DataGathererDiscovery stores the config for a k8s-discovery datagatherer
type DataGathererDiscovery struct {
	// The 'discovery' client used for fetching data.
	cl *discovery.DiscoveryClient
	// The cluster ID, derived from the UID of the 'kube-system' namespace.
	clusterID string
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
	data, err := g.cl.ServerVersion()
	if err != nil {
		return nil, -1, fmt.Errorf("failed to get server version: %v", err)
	}
	response := &api.DiscoveryData{
		ClusterID:     g.clusterID,
		ServerVersion: data,
	}
	return response, 1, nil
}
