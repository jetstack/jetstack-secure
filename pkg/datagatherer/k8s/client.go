// Package k8s provides datagatherers for different parts of the Kubernetes API.
package k8s

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// NewDynamicClient creates a new 'dynamic' clientset using the provided kubeconfig.
// If kubeconfigPath is not set/empty, it will attempt to load the InClusterConfig.
func NewDynamicClient(kubeconfigPath string) (dynamic.Interface, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cl, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return cl, nil
}
