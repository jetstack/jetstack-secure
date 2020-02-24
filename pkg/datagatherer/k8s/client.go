// Package k8s provides datagatherers for different parts of the Kubernetes API.
package k8s

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/dynamic"
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

func loadRESTConfig(path string) (*rest.Config, error) {
	switch path {
	// If the kubeconfig path is not provided, use the default loading rules
	// so we read the regular KUBECONFIG variable.
	case "":
		apicfg, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		cfg, err := clientcmd.NewDefaultClientConfig(*apicfg, &clientcmd.ConfigOverrides{}).ClientConfig()
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
