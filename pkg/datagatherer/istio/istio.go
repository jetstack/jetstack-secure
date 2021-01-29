// Package isito provides a datagatherer for Istio.
package isito

import (
	"context"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"strings"
	"time"

	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"istio.io/istio/galley/pkg/config/analysis/analyzers"
	"istio.io/istio/galley/pkg/config/analysis/local"
	"istio.io/istio/pkg/config/resource"
	istioSchema "istio.io/istio/pkg/config/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/jetstack/preflight/pkg/datagatherer"
)

// Config is the configuration for the Istio DataGatherer.
type Config struct {
	// KubeConfigPath is the path to the kubeconfig file. If empty, will assume it runs in-cluster.
	KubeConfigPath string `yaml:"kubeconfig"`
	// IstioNamespace is the Namespace for Istio components, this is usually `istio-system`.
	IstioNamespace string `yaml:"istio-namespace"`
	// ExcludeNamespaces is a list of namespaces to exclude.
	ExcludeNamespaces []string `yaml:"exclude-namespaces"`
	// IncludeNamespaces is a list of namespaces to include.
	IncludeNamespaces []string `yaml:"include-namespaces"`
	// Resources is a list of GroupVersionResources to collect for Istio analysis.
	Resources []schema.GroupVersionResource
}

// validate validates the configuration.
func (c *Config) validate() error {
	var errors []string

	if len(c.ExcludeNamespaces) > 0 && len(c.IncludeNamespaces) > 0 {
		errors = append(errors, "cannot set excluded and included namespaces")
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, ", "))
	}

	return nil
}

// DataGatherer is a DataGatherer for Istio.
type DataGatherer struct {
	sa       *local.SourceAnalyzer
	dynamics []datagatherer.DataGatherer
}

// NewDataGatherer creates a new DataGatherer for a cluster.
func (c *Config) NewDataGatherer(ctx context.Context) (datagatherer.DataGatherer, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	// If the Istio namespace is not set then default it to 'istio-system'.
	istioNamespace := c.IstioNamespace
	if istioNamespace == "" {
		istioNamespace = "istio-system"
	}

	// If a list of resources is not set then use the data gatherer default list.
	gvrs := c.Resources
	if len(gvrs) == 0 {
		gvrs = defaultGVRs
	}

	// Create a list of Kubernetes dynamic data gatherers to fetch all the required resources for Istio analysis.
	var dynamics []datagatherer.DataGatherer
	for _, gvr := range gvrs {
		configDynamic := k8s.ConfigDynamic{
			KubeConfigPath:       c.KubeConfigPath,
			GroupVersionResource: gvr,
			ExcludeNamespaces:    c.ExcludeNamespaces,
			IncludeNamespaces:    c.IncludeNamespaces,
		}
		dynamic, err := configDynamic.NewDataGatherer(ctx)
		if err != nil {
			// TODO: Might be better to collect errors here rather than just give up straight away.
			return nil, err
		}
		dynamics = append(dynamics, dynamic)
	}

	// Create an Istio SourceAnalyzer.
	sa := local.NewSourceAnalyzer(istioSchema.MustGet(), analyzers.AllCombined(),
		"", resource.Namespace(istioNamespace), nil, true, 30*time.Second)

	return &DataGatherer{
		sa:       sa,
		dynamics: dynamics,
	}, nil
}

// Fetch retrieves resources from the K8s API and runs Istio analysis
func (g *DataGatherer) Fetch() (interface{}, error) {

	// Fetch resources from all data gatherers and accumulate in an Unstructured slice.
	var allResources []unstructured.Unstructured
	for _, dynamic := range g.dynamics {
		rawResources, err := dynamic.Fetch()
		if err != nil {
			// Data gatherers will error if the resource kind they are trying to fetch is not present in the cluster.
			// This could be because the cluster does not yet have Istio installed. However we should still run the
			// analysis on the resources that are available as it is useful for pre-checking a cluster for future Istio
			// installations.
			continue
		}
		resources, ok := rawResources.(*unstructured.UnstructuredList)
		if !ok {
			return nil, fmt.Errorf("failed to parse resources loaded from DataGatherer")
		}
		allResources = append(allResources, resources.Items...)
	}

	// Convert the slice of Unstructured resources into a YAML document string.
	allResourcesYAMLString := ""
	for _, resource := range allResources {
		// Marshall the unstructured resource to a YAML string.
		resourceYAML, err := yaml.Marshal(resource.Object)
		if err != nil {
			return nil, err
		}
		allResourcesYAMLString = string(resourceYAML) + "---\n" + allResourcesYAMLString
	}

	// Pass the YAML document string to the Istio analyzer wrapped in a Reader.
	g.sa.AddReaderKubeSource([]local.ReaderSource{{Name: "", Reader: strings.NewReader(allResourcesYAMLString)}})

	// Perform Istio analysis.
	result, err := g.sa.Analyze(nil)
	if err != nil {
		return nil, err
	}

	// Marshal Istio analysis results to JSON and return as a string.
	jsonOutput, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return string(jsonOutput), nil
}
