// Package datagatherer provides the DataGatherer interface.
package datagatherer

import (
	"context"
	"log"

	"github.com/jetstack/preflight/pkg/datagatherer/aks"
	"github.com/jetstack/preflight/pkg/datagatherer/eks"
	"github.com/jetstack/preflight/pkg/datagatherer/gke"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"github.com/jetstack/preflight/pkg/datagatherer/local"
)

// DataGatherer is the interface for Data Gatherers. Data Gatherers are in charge of fetching data from a certain cloud provider API or Kubernetes component.
type DataGatherer interface {
	// Fetch retrieves data.
	Fetch() (interface{}, error)
}

type DataGatherersConfig struct {
	GKE   *gke.GKEDataGathererConfig     `mapstructure:"gke,omitempty"`
	AKS   *aks.AKSDataGathererConfig     `mapstructure:"aks,omitempty"`
	EKS   *eks.EKSDataGathererConfig     `mapstructure:"eks,omitempty"`
	Pods  *k8s.PodsDataGathererConfig    `mapstructure:"k8s/pods,omitempty"`
	Local *local.LocalDataGathererConfig `mapstructure:"local,omitempty"`
}

func NewDataGatherers(ctx context.Context, config *DataGatherersConfig) map[string]DataGatherer {
	dataGatherers := make(map[string]DataGatherer, 0)
	var dataGatherer DataGatherer
	if config.GKE != nil {
		if config.GKE.DataPath != "" {
			dataGatherer = local.NewLocalDataGatherer(ctx, &local.LocalDataGathererConfig{
				DataPath: config.GKE.DataPath,
			})
		} else {
			dataGatherer = gke.NewGKEDataGatherer(ctx, config.GKE)
		}
		dataGatherers["gke"] = dataGatherer
	}
	if config.AKS != nil {
		if config.AKS.DataPath != "" {
			dataGatherer = local.NewLocalDataGatherer(ctx, &local.LocalDataGathererConfig{
				DataPath: config.AKS.DataPath,
			})
		} else {
			var err error
			dataGatherer, err = aks.NewAKSDataGatherer(ctx, config.AKS)
			if err != nil {
				log.Fatalf("Cannot instantiate AKS datagatherer: %v", err)
			}
		}
		dataGatherers["aks"] = dataGatherer
	}
	if config.EKS != nil {
		if config.EKS.DataPath != "" {
			dataGatherer = local.NewLocalDataGatherer(ctx, &local.LocalDataGathererConfig{
				DataPath: config.EKS.DataPath,
			})
		} else {
			dataGatherer = eks.NewEKSDataGatherer(ctx, config.EKS)
		}
		dataGatherers["eks"] = dataGatherer
	}
	if config.Pods != nil {
		if config.EKS.DataPath != "" {
			dataGatherer = local.NewLocalDataGatherer(ctx, &local.LocalDataGathererConfig{
				DataPath: config.EKS.DataPath,
			})
		} else {
			dataGatherer = k8s.NewPodsDataGatherer(ctx, config.Pods)
		}
		dataGatherers["pods"] = dataGatherer
	}
	if config.Local != nil {
		dataGatherer = local.NewLocalDataGatherer(ctx, config.Local)
		dataGatherers["local"] = dataGatherer
	}
	return dataGatherers
}
