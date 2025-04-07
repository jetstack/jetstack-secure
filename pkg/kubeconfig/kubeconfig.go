package kubeconfig

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// LoadRESTConfig loads the kube config from the provided path. If the path is
// empty, the kube config will be loaded from KUBECONFIG, and if KUBECONFIG
// isn't set, the in-cluster config will be used.
func LoadRESTConfig(path string) (*rest.Config, error) {
	loadingrules := clientcmd.NewDefaultClientConfigLoadingRules()

	// If the kubeconfig path is provided, use that file and fail if it does
	// not exist.
	// If the kubeconfig path is not provided, use the default loading rules
	// so we read the regular KUBECONFIG variable or create a non-interactive
	// client for agents running in cluster
	loadingrules.ExplicitPath = path

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingrules,
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return cfg, nil
}
