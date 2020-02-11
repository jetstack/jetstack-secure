package agent

import (
	"context"
	"fmt"
	"os/user"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/datagatherer/aks"
	"github.com/jetstack/preflight/pkg/datagatherer/eks"
	"github.com/jetstack/preflight/pkg/datagatherer/gke"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"github.com/jetstack/preflight/pkg/datagatherer/local"
)

//LoadDataGatherer takes key:value config for a datagatherer and loads it into a DataGatherer
func LoadDataGatherer(ctx context.Context, kind string, config map[string]string) (datagatherer.DataGatherer, error) {
	// var err error
	var dg datagatherer.DataGatherer

	if kind == "gke" {
		dg, err := loadGKEConfig(ctx, config)
		if err != nil {
			return dg, fmt.Errorf("failed to load GKE DataGatherer: %v", err)
		}
		return dg, nil
	} else if kind == "k8s/pod" {
		dg, err := loadK8sConfig(config)
		if err != nil {
			return dg, fmt.Errorf("failed to load K8s DataGatherer: %v", err)
		}
		return dg, nil
	} else if kind == "eks" {
		dg, err := loadEKSConfig(config)
		if err != nil {
			return dg, fmt.Errorf("failed to load EKS DataGatherer: %v", err)
		}
		return dg, nil
	} else if kind == "aks" {
		dg, err := loadAKSConfig(ctx, config)
		if err != nil {
			return dg, fmt.Errorf("failed to load AKS DataGatherer: %v", err)
		}
		return dg, nil
	} else if kind == "local" {
		dg, err := loadLocalConfig(config)
		if err != nil {
			return dg, fmt.Errorf("failed to load Local DataGatherer: %v", err)
		}
		return dg, nil
	} else {
		return dg, fmt.Errorf("cannot load data gatherer, kind '%s' is not supported", kind)
	}
}

func loadGKEConfig(ctx context.Context, config map[string]string) (datagatherer.DataGatherer, error) {
	var result *multierror.Error

	if config["project"] == "" {
		result = multierror.Append(result, fmt.Errorf("project is required"))
	}
	if config["cluster"] == "" {
		result = multierror.Append(result, fmt.Errorf("cluster is required"))
	}
	if config["location"] == "" {
		result = multierror.Append(result, fmt.Errorf("location is required"))
	}
	if config["credentials"] == "" {
		result = multierror.Append(result, fmt.Errorf("credentials is required"))
	}

	cluster := gke.Cluster{
		Project:  config["project"],
		Name:     config["cluster"],
		Location: config["location"],
	}

	return gke.NewGKEDataGatherer(ctx, &cluster, config["credentials"]), result.ErrorOrNil()
}

func loadK8sConfig(config map[string]string) (datagatherer.DataGatherer, error) {
	var dg datagatherer.DataGatherer

	if config["kubeconfig"] == "" {
		return dg, fmt.Errorf("kubeconfig path is required")
	}

	k8sClient, err := k8s.NewClient(expandHome(config["kubeconfig"]))
	if err != nil {
		return dg, fmt.Errorf("failed to create K8s client %s", err)
	}

	return k8s.NewPodsDataGatherer(k8sClient), nil
}

func loadEKSConfig(config map[string]string) (datagatherer.DataGatherer, error) {
	var dg datagatherer.DataGatherer

	if config["cluster"] == "" {
		return dg, fmt.Errorf("cluster is required")
	}

	return eks.NewEKSDataGatherer(config["cluster"]), nil
}

func loadAKSConfig(ctx context.Context, config map[string]string) (datagatherer.DataGatherer, error) {
	var dg datagatherer.DataGatherer
	var result *multierror.Error

	if config["resource-group"] == "" {
		result = multierror.Append(result, fmt.Errorf("project is required"))
	}
	if config["cluster"] == "" {
		result = multierror.Append(result, fmt.Errorf("cluster is required"))
	}
	if config["credentials"] == "" {
		result = multierror.Append(result, fmt.Errorf("location is required"))
	}

	if result.ErrorOrNil() != nil {
		return dg, result.ErrorOrNil()
	}

	return aks.NewAKSDataGatherer(
		ctx, config["resource-group"], config["cluster"], config["credentials"])
}

func loadLocalConfig(config map[string]string) (datagatherer.DataGatherer, error) {
	var dg datagatherer.DataGatherer

	if config["data-path"] == "" {
		return dg, fmt.Errorf("data-path is required")
	}

	return local.NewLocalDataGatherer(config["data-path"]), nil
}

// TODO copied from check, perhaps move to utils or leave here until check has gone from the client?
func expandHome(path string) string {
	if len(path) == 0 {
		return ""
	}

	if path[:2] == "~/" {
		return filepath.Join(homeDir(), path[2:])
	}
	return path
}

// TODO copied from check, perhaps move to utils or leave here until check has gone from the client?
func homeDir() string {
	usr, err := user.Current()
	if err != nil {
		return ""
	}
	return usr.HomeDir
}
