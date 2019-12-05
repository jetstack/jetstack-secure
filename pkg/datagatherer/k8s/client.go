// Package k8s provides datagatherers for different parts of the Kubernetes API.
package k8s

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NewClient creates a Kubernetes ClientSet from kubeconfig path. If kubeconfigPath is empty will fallback to using InClusterConfig.
func NewClient(kubeconfigPath string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return clientset, nil
}
