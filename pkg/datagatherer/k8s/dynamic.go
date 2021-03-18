package k8s

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/pkg/errors"
	"github.com/pmylund/go-cache"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes/scheme"
	k8scache "k8s.io/client-go/tools/cache"
)

// ConfigDynamic contains the configuration for the data-gatherer.
type ConfigDynamic struct {
	// KubeConfigPath is the path to the kubeconfig file. If empty, will assume it runs in-cluster.
	KubeConfigPath string `yaml:"kubeconfig"`
	// GroupVersionResource identifies the resource type to gather.
	GroupVersionResource schema.GroupVersionResource
	// ExcludeNamespaces is a list of namespaces to exclude.
	ExcludeNamespaces []string `yaml:"exclude-namespaces"`
	// IncludeNamespaces is a list of namespaces to include.
	IncludeNamespaces []string `yaml:"include-namespaces"`
}

// UnmarshalYAML unmarshals the ConfigDynamic resolving GroupVersionResource.
func (c *ConfigDynamic) UnmarshalYAML(unmarshal func(interface{}) error) error {
	aux := struct {
		KubeConfigPath string `yaml:"kubeconfig"`
		ResourceType   struct {
			Group    string `yaml:"group"`
			Version  string `yaml:"version"`
			Resource string `yaml:"resource"`
		} `yaml:"resource-type"`
		ExcludeNamespaces []string `yaml:"exclude-namespaces"`
		IncludeNamespaces []string `yaml:"include-namespaces"`
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
	c.IncludeNamespaces = aux.IncludeNamespaces

	return nil
}

// validate validates the configuration.
func (c *ConfigDynamic) validate() error {
	var errors []string
	if len(c.ExcludeNamespaces) > 0 && len(c.IncludeNamespaces) > 0 {
		errors = append(errors, "cannot set excluded and included namespaces")
	}

	if c.GroupVersionResource.Resource == "" {
		errors = append(errors, "invalid configuration: GroupVersionResource.Resource cannot be empty")
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, ", "))
	}

	return nil
}

// NewDataGatherer constructs a new instance of the generic K8s data-gatherer for the provided
// GroupVersionResource.
func (c *ConfigDynamic) NewDataGatherer(ctx context.Context) (datagatherer.DataGatherer, error) {
	cl, err := NewDynamicClient(c.KubeConfigPath)
	if err != nil {
		return nil, err
	}

	return c.newDataGathererWithClient(ctx, cl)
}

func (c *ConfigDynamic) newDataGathererWithClient(ctx context.Context, cl dynamic.Interface) (datagatherer.DataGatherer, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	fieldSelector := generateFieldSelector(c.ExcludeNamespaces)
	// init cache
	dgCache := cache.New(5*time.Minute, 30*time.Second)
	// init shared informer
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(cl, 30*time.Second, metav1.NamespaceAll, func(options *metav1.ListOptions) {
		options.FieldSelector = fieldSelector
	})
	resourceInformer := factory.ForResource(c.GroupVersionResource)
	informer := resourceInformer.Informer()
	informer.AddEventHandler(k8scache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			onAdd(obj, dgCache)
		},
		UpdateFunc: func(old, new interface{}) {
			onUpdate(old, new, dgCache)
		},
		DeleteFunc: func(obj interface{}) {
			onDelete(obj, dgCache)
		},
	})

	return &DataGathererDynamic{
		ctx:                  ctx,
		cl:                   cl,
		groupVersionResource: c.GroupVersionResource,
		fieldSelector:        fieldSelector,
		namespaces:           c.IncludeNamespaces,
		cache:                dgCache,
		sharedInformer:       factory,
		informer:             informer,
	}, nil
}

// DataGathererDynamic is a generic gatherer for Kubernetes. It knows how to request
// a list of generic resources from the Kubernetes apiserver.
// It does not deserialize the objects into structured data, instead utilising
// the Kubernetes `Unstructured` type for data handling.
// This is to allow us to support arbitrary CRDs and resources that Preflight
// does not have registered as part of its `runtime.Scheme`.
type DataGathererDynamic struct {
	ctx context.Context
	// The 'dynamic' client used for fetching data.
	cl dynamic.Interface
	// groupVersionResource is the name of the API group, version and resource
	// that should be fetched by this data gatherer.
	groupVersionResource schema.GroupVersionResource
	// namespace, if specified, limits the namespace of the resources returned.
	// This field *must* be omitted when the groupVersionResource refers to a
	// non-namespaced resource.
	namespaces []string
	// fieldSelector is a field selector string used to filter resources
	// returned by the Kubernetes API.
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/
	fieldSelector string
	// cache holds all resources watched by the data gatherer, default object expiry time 5 minutes
	// 30 seconds purge time https://pkg.go.dev/github.com/patrickmn/go-cache
	cache *cache.Cache
	// informer watches the events around the targeted resource and updates the cache
	informer       k8scache.SharedIndexInformer
	sharedInformer dynamicinformer.DynamicSharedInformerFactory
}

// Run starts the dynamic data gatherer's informers for resource collection.
// Returns error if the data gatherer informer wasn't initialized
func (g *DataGathererDynamic) Run(stopCh <-chan struct{}) error {
	if g.sharedInformer == nil {
		return fmt.Errorf("data gatherer informer was not initialized")
	}
	// start shared informer
	g.sharedInformer.Start(stopCh)
	return nil
}

// WaitForCacheSync waits for the data gatherer's informers cache to sync before collecting the resources.
func (g *DataGathererDynamic) WaitForCacheSync(stopCh <-chan struct{}) error {
	if !k8scache.WaitForCacheSync(stopCh, g.informer.HasSynced) {
		return fmt.Errorf("timed out waiting for caches to sync")
	}
	return nil
}

func (g *DataGathererDynamic) Equals(old datagatherer.DataGatherer) bool {
	// shallow equality
	dg, ok := old.(*DataGathererDynamic)
	if !ok {
		return false
	}

	if !reflect.DeepEqual(g.groupVersionResource, dg.groupVersionResource) ||
		!reflect.DeepEqual(g.namespaces, dg.namespaces) || !reflect.DeepEqual(g.fieldSelector, dg.fieldSelector) {
		return false
	}

	return true
}

// Fetch will fetch the requested data from the apiserver, or return an error
// if fetching the data fails.
func (g *DataGathererDynamic) Fetch() (interface{}, error) {
	if g.groupVersionResource.Resource == "" {
		return nil, fmt.Errorf("resource type must be specified")
	}

	var list = map[string]interface{}{}
	var items = []*api.GatheredResource{}

	fetchNamespaces := g.namespaces
	if len(fetchNamespaces) == 0 {
		// then they must have been looking for all namespaces
		fetchNamespaces = []string{metav1.NamespaceAll}
	}

	//delete expired items from the cache
	g.cache.DeleteExpired()
	for _, item := range g.cache.Items() {
		// filter cache items by namespace
		cacheObject := item.Object.(*api.GatheredResource)
		resource, ok := cacheObject.Resource.(*unstructured.Unstructured)
		if !ok {
			return nil, fmt.Errorf("failed to parse cached resource")
		}
		namespace := resource.GetNamespace()
		if isIncludedNamespace(namespace, fetchNamespaces) {
			items = append(items, cacheObject)
		}
	}

	// Redact Secret data
	err := redactList(items)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// add gathered resources to items
	list["items"] = items

	return list, nil
}

func redactList(list []*api.GatheredResource) error {
	for i := range list {
		item := list[i].Resource.(*unstructured.Unstructured)
		// Determine the kind of items in case this is a generic 'mixed' list.
		gvks, _, err := scheme.Scheme.ObjectKinds(item)
		if err != nil {
			return errors.WithStack(err)
		}

		resource := item

		for _, gvk := range gvks {
			// If this item is a Secret then we need to redact it.
			if gvk.Kind == "Secret" && (gvk.Group == "core" || gvk.Group == "") {
				Select(SecretSelectedFields, resource)

				// break when the object has been processed as a secret, no
				// other kinds have redact modifications
				break
			}

		}

		// remove managedFields from all resources
		Redact(RedactFields, resource)

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

func isIncludedNamespace(namespace string, namespaces []string) bool {
	if namespaces[0] == metav1.NamespaceAll {
		return true
	}
	for _, current := range namespaces {
		if namespace == current {
			return true
		}
	}
	return false
}
