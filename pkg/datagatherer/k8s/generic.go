package k8s

import (
	"context"
	"fmt"

	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
)

// Config contains the configuration for the data-gatherer.
type Config struct {
	// KubeConfigPath is the path to the kubeconfig file. If empty, will assume it runs in-cluster.
	KubeConfigPath string `yaml:"kubeconfig"`
	// GroupVersionResource identifies the resource type to gather.
	GroupVersionResource schema.GroupVersionResource
	// ExcludeNamespaces is a list of namespaces to exclude.
	ExcludeNamespaces []string `yaml:"exclude-namespaces"`
}

// UnmarshalYAML unmarshals the Config resolving GroupVersionResource.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	aux := struct {
		KubeConfigPath string `yaml:"kubeconfig"`
		ResourceType   struct {
			Group    string `yaml:"group"`
			Version  string `yaml:"version"`
			Resource string `yaml:"resource"`
		} `yaml:"resource-type"`
		ExcludeNamespaces []string `yaml:"exclude-namespaces"`
	}{}
	err := unmarshal(&aux)
	if err != nil {
		return err
	}

	c.KubeConfigPath = aux.KubeConfigPath
	c.GroupVersionResource.Group = aux.ResourceType.Group
	c.GroupVersionResource.Version = aux.ResourceType.Version
	c.GroupVersionResource.Resource = aux.ResourceType.Resource
	c.ExcludeNamespaces = aux.ExcludeNamespaces

	return nil
}

// validate validates the configuration.
func (c *Config) validate() error {
	if c.GroupVersionResource.Resource == "" {
		return fmt.Errorf("invalid configuration: GroupVersionResource.Resource cannot be empty")
	}

	return nil
}

// NewDataGatherer constructs a new instance of the generic K8s data-gatherer for the provided
// GroupVersionResource.
func (c *Config) NewDataGatherer(ctx context.Context) (datagatherer.DataGatherer, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	cl, err := NewDynamicClient(c.KubeConfigPath)
	if err != nil {
		return nil, err
	}

	return &DataGatherer{
		cl:                   cl,
		groupVersionResource: c.GroupVersionResource,
		fieldSelector:        generateFieldSelector(c.ExcludeNamespaces),
	}, nil
}

// DataGatherer is a generic gatherer for Kubernetes. It knows how to request
// a list of generic resources from the Kubernetes apiserver.
// It does not deserialize the objects into structured data, instead utilising
// the Kubernetes `Unstructured` type for data handling.
// This is to allow us to support arbitrary CRDs and resources that Preflight
// does not have registered as part of its `runtime.Scheme`.
type DataGatherer struct {
	// The 'dynamic' client used for fetching data.
	cl dynamic.Interface
	// groupVersionResource is the name of the API group, version and resource
	// that should be fetched by this data gatherer.
	groupVersionResource schema.GroupVersionResource
	// namespace, if specified, limits the namespace of the resources returned.
	// This field *must* be omitted when the groupVersionResource refers to a
	// non-namespaced resource.
	namespace string
	// fieldSelector is a field selector string used to filter resources
	// returned by the Kubernetes API.
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/
	fieldSelector string
}

// Fetch will fetch the requested data from the apiserver, or return an error
// if fetching the data fails.
func (g *DataGatherer) Fetch() (interface{}, error) {
	if g.groupVersionResource.Resource == "" {
		return nil, fmt.Errorf("resource type must be specified")
	}
	resourceInterface := namespaceResourceInterface(g.cl.Resource(g.groupVersionResource), g.namespace)
	list, err := resourceInterface.List(metav1.ListOptions{
		FieldSelector: g.fieldSelector,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// Redact Secret data
	err = redactList(list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return list, nil
}

func redactList(list *unstructured.UnstructuredList) error {
	// In principal we could only redact the list if it's kind is SecretList or
	// a generic mixed List, however the test suite does not set the list kind
	// and it is safer to always check for Secrets.
	// Iterate over the items in the list.
	for i := range list.Items {
		// Determine the kind of items in case this is a generic 'mixed' list.
		gvks, _, err := scheme.Scheme.ObjectKinds(&list.Items[i])
		if err != nil {
			return errors.WithStack(err)
		}
		for _, gvk := range gvks {
			// If this item is a Secret then we need to redact it.
			if gvk.Kind == "Secret" && (gvk.Group == "core" || gvk.Group == "") {
				// Redact the Secret by overwriting its data.
				list.Items[i].Object["data"] = map[string]interface{}{}
				break
			}
		}
	}
	return nil
}

// namespaceResourceInterface will 'namespace' a NamespaceableResourceInterface
// if the 'namespace' parameter is non-empty, otherwise it will return the
// given ResourceInterface as-is.
func namespaceResourceInterface(iface dynamic.NamespaceableResourceInterface, namespace string) dynamic.ResourceInterface {
	if namespace == "" {
		return iface
	}
	return iface.Namespace(namespace)
}

// generateFieldSelector creates a field selector string from a list of
// namespaces to exclude.
func generateFieldSelector(excludeNamespaces []string) string {
	fieldSelector := fields.Nothing()
	for _, excludeNamespace := range excludeNamespaces {
		if excludeNamespace == "" {
			continue
		}
		fieldSelector = fields.AndSelectors(fields.OneTermNotEqualSelector("metadata.namespace", excludeNamespace), fieldSelector)
	}
	return fieldSelector.String()
}
