package k8s

import (
	"fmt"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// Config contains the configuration for the data-gatherer.
type Config struct {
	// KubeConfigPath is the path to the kubeconfig file. If empty, will assume it runs in-cluster.
	KubeConfigPath string
	// GroupVersionResource identifies the resource type to gather.
	GroupVersionResource schema.GroupVersionResource
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.GroupVersionResource.Resource == "" {
		return fmt.Errorf("invalid configuration: GroupVersionResource.Resource cannot be empty")
	}

	return nil
}

// NewDataGatherer constructs a new instance of the generic K8s data-gatherer for the provided
// GroupVersionResource.
func NewDataGatherer(cfg *Config) (*DataGatherer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	cl, err := NewDynamicClient(cfg.KubeConfigPath)
	if err != nil {
		return nil, err
	}

	return &DataGatherer{
		cl:                   cl,
		groupVersionResource: cfg.GroupVersionResource,
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
}

// Fetch will fetch the requested data from the apiserver, or return an error
// if fetching the data fails.
func (g *DataGatherer) Fetch() (interface{}, error) {
	if g.groupVersionResource.Resource == "" {
		return nil, fmt.Errorf("resource type must be specified")
	}
	resourceInterface := namespaceResourceInterface(g.cl.Resource(g.groupVersionResource), g.namespace)
	list, err := resourceInterface.List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return list, nil
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
