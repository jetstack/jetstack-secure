package k8s

import (
	"fmt"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// The generic gatherer knows how to request a list of generic resources from
// the Kubernetes apiserver.
// It does not deserialize the objects into structured data, instead utilising
// the Kubernetes `Unstructured` type for data handling.
// This is to allow us to support arbitrary CRDs and resources that Preflight
// does not have registered as part of its `runtime.Scheme`.
type genericGatherer struct {
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

// NewGenericGatherer constructs a new instance of the generic Kubernetes data
// gatherer for the provided GroupVersionResource.
func NewGenericGatherer(cl dynamic.Interface, gvr schema.GroupVersionResource) *genericGatherer {
	return &genericGatherer{
		cl:                   cl,
		groupVersionResource: gvr,
	}
}

// Fetch will fetch the requested data from the apiserver, or return an error
// if fetching the data fails.
func (g *genericGatherer) Fetch() (interface{}, error) {
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
