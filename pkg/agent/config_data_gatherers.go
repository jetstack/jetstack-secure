package agent

import (
	"context"
	"fmt"

	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/datagatherer/aks"
	"github.com/jetstack/preflight/pkg/datagatherer/eks"
	"github.com/jetstack/preflight/pkg/datagatherer/gke"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"github.com/jetstack/preflight/pkg/datagatherer/local"
	"gopkg.in/yaml.v2"
)

//LoadDataGatherer instantiates a DataGatherer with the provided configuration.
func LoadDataGatherer(ctx context.Context, kind string, configBytes []byte) (datagatherer.DataGatherer, error) {
	var err error
	var dg datagatherer.DataGatherer

	var cfg datagatherer.Config

	switch kind {
	case "gke":
		cfg = &gke.Config{}
	case "eks":
		cfg = &eks.Config{}
	case "aks":
		cfg = &aks.Config{}
	case "k8s":
		cfg = &k8s.Config{}
	case "local":
		cfg = &local.Config{}
	// dummy dataGatherer is just used for testing
	case "dummy":
		cfg = &dummyConfig{}
	default:
		return dg, fmt.Errorf("cannot load data gatherer, kind '%s' is not supported", kind)
	}

	err = yaml.Unmarshal(configBytes, cfg)
	if err != nil {
		return dg, fmt.Errorf("failed to load %s DataGatherer configuration: %v", kind, err)
	}

	dg, err = cfg.NewDataGatherer(ctx)
	if err != nil {
		return dg, fmt.Errorf("failed to instantiate %s DataGatherer: %v", kind, err)
	}

	return dg, nil
}
