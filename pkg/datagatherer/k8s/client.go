// Package k8s provides datagatherers for different parts of the Kubernetes API.
package k8s

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewDynamicClient creates a new 'dynamic' clientset using the provided kubeconfig.
// If kubeconfigPath is not set/empty, it will attempt to load configuration using
// the default loading rules.
func NewDynamicClient(kubeconfigPath string) (dynamic.Interface, error) {
	cfg, err := loadRESTConfig(kubeconfigPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	cl, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return cl, nil
}

// NewDiscoveryClient creates a new 'discovery' client using the provided
// kubeconfig.  If kubeconfigPath is not set/empty, it will attempt to load
// configuration using the default loading rules.
func NewDiscoveryClient(kubeconfigPath string) (discovery.DiscoveryClient, error) {
	var discoveryClient *discovery.DiscoveryClient

	cfg, err := loadRESTConfig(kubeconfigPath)
	if err != nil {
		return discovery.DiscoveryClient{}, errors.WithStack(err)
	}

	discoveryClient, err = discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return *discoveryClient, errors.WithStack(err)
	}

	return *discoveryClient, nil
}

// NewClientSet creates a new kubernetes clientset using the provided kubeconfig.
// If kubeconfigPath is not set/empty, it will attempt to load configuration using
// the default loading rules.
func NewClientSet(kubeconfigPath string) (kubernetes.Interface, error) {
	var clientset *kubernetes.Clientset
	cfg, err := loadRESTConfig(kubeconfigPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	clientset, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return clientset, nil
}

func loadRESTConfig(path string) (*rest.Config, error) {
	switch path {
	// If the kubeconfig path is not provided, use the default loading rules
	// so we read the regular KUBECONFIG variable or create a non-interactive
	// client for agents running in cluster
	case "":
		loadingrules := clientcmd.NewDefaultClientConfigLoadingRules()
		cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingrules, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return cfg, nil
	// Otherwise use the explicitly named kubeconfig file.
	default:
		cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: path},
			&clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return cfg, nil
	}
}
