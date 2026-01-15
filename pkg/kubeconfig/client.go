package kubeconfig

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// NewDynamicClient creates a new 'dynamic' clientset using the provided kubeconfig.
// If kubeconfigPath is not set/empty, it will attempt to load configuration using
// the default loading rules.
func NewDynamicClient(kubeconfigPath string) (dynamic.Interface, error) {
	cfg, err := LoadRESTConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	cl, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return cl, nil
}

// NewDiscoveryClient creates a new 'discovery' client using the provided
// kubeconfig.  If kubeconfigPath is not set/empty, it will attempt to load
// configuration using the default loading rules.
func NewDiscoveryClient(kubeconfigPath string) (*discovery.DiscoveryClient, error) {
	cfg, err := LoadRESTConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return discoveryClient, nil
}

// NewClientSet creates a new kubernetes clientset using the provided kubeconfig.
// If kubeconfigPath is not set/empty, it will attempt to load configuration using
// the default loading rules.
func NewClientSet(kubeconfigPath string) (kubernetes.Interface, error) {
	cfg, err := LoadRESTConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}
