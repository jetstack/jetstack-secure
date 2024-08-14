package k8s

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/pmylund/go-cache"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	k8scache "k8s.io/client-go/tools/cache"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/datagatherer"
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
	// FieldSelectors is a list of field selectors to use when listing this resource
	FieldSelectors []string `yaml:"field-selectors"`
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
		FieldSelectors    []string `yaml:"field-selectors"`
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
	c.FieldSelectors = aux.FieldSelectors

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

	for _, selectorString := range c.FieldSelectors {
		_, err := fields.ParseSelector(selectorString)
		if err != nil {
			errors = append(errors, fmt.Sprintf("invalid field selector %q: %s", selectorString, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, ", "))
	}

	return nil
}

// sharedInformerFunc creates a SharedIndexInformer given a SharedInformerFactory
type sharedInformerFunc func(informers.SharedInformerFactory) k8scache.SharedIndexInformer

// kubernetesNativeResources map of the native kubernetes resources, linking each resource to a sharedInformerFunc for that resource.
// secrets are still treated as unstructured rather than corev1.Secret, for a faster unmarshaling
var kubernetesNativeResources = map[schema.GroupVersionResource]sharedInformerFunc{
	corev1.SchemeGroupVersion.WithResource("pods"): func(sharedFactory informers.SharedInformerFactory) k8scache.SharedIndexInformer {
		return sharedFactory.Core().V1().Pods().Informer()
	},
	corev1.SchemeGroupVersion.WithResource("nodes"): func(sharedFactory informers.SharedInformerFactory) k8scache.SharedIndexInformer {
		return sharedFactory.Core().V1().Nodes().Informer()
	},
	corev1.SchemeGroupVersion.WithResource("services"): func(sharedFactory informers.SharedInformerFactory) k8scache.SharedIndexInformer {
		return sharedFactory.Core().V1().Services().Informer()
	},
	appsv1.SchemeGroupVersion.WithResource("deployments"): func(sharedFactory informers.SharedInformerFactory) k8scache.SharedIndexInformer {
		return sharedFactory.Apps().V1().Deployments().Informer()
	},
	appsv1.SchemeGroupVersion.WithResource("daemonsets"): func(sharedFactory informers.SharedInformerFactory) k8scache.SharedIndexInformer {
		return sharedFactory.Apps().V1().DaemonSets().Informer()
	},
	appsv1.SchemeGroupVersion.WithResource("statefulsets"): func(sharedFactory informers.SharedInformerFactory) k8scache.SharedIndexInformer {
		return sharedFactory.Apps().V1().StatefulSets().Informer()
	},
	appsv1.SchemeGroupVersion.WithResource("replicasets"): func(sharedFactory informers.SharedInformerFactory) k8scache.SharedIndexInformer {
		return sharedFactory.Apps().V1().ReplicaSets().Informer()
	},
	appsv1.SchemeGroupVersion.WithResource("replicasets"): func(sharedFactory informers.SharedInformerFactory) k8scache.SharedIndexInformer {
		return sharedFactory.Apps().V1().ReplicaSets().Informer()
	},
	admissionregistrationv1.SchemeGroupVersion.WithResource("validatingwebhookconfigurations"): func(sharedFactory informers.SharedInformerFactory) k8scache.SharedIndexInformer {
		return sharedFactory.Admissionregistration().V1().ValidatingWebhookConfigurations().Informer()
	},
	admissionregistrationv1.SchemeGroupVersion.WithResource("mutatingwebhookconfigurations"): func(sharedFactory informers.SharedInformerFactory) k8scache.SharedIndexInformer {
		return sharedFactory.Admissionregistration().V1().MutatingWebhookConfigurations().Informer()
	},
	batchv1.SchemeGroupVersion.WithResource("jobs"): func(sharedFactory informers.SharedInformerFactory) k8scache.SharedIndexInformer {
		return sharedFactory.Batch().V1().Jobs().Informer()
	},
}

// NewDataGatherer constructs a new instance of the generic K8s data-gatherer for the provided
func (c *ConfigDynamic) NewDataGatherer(ctx context.Context) (datagatherer.DataGatherer, error) {
	cl, err := NewDynamicClient(c.KubeConfigPath)
	if err != nil {
		return nil, err
	}

	if isNativeResource(c.GroupVersionResource) {
		clientset, err := NewClientSet(c.KubeConfigPath)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return c.newDataGathererWithClient(ctx, nil, clientset)
	}

	return c.newDataGathererWithClient(ctx, cl, nil)
}

func (c *ConfigDynamic) newDataGathererWithClient(ctx context.Context, cl dynamic.Interface, clientset kubernetes.Interface) (datagatherer.DataGatherer, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	// init shared informer for selected namespaces
	fieldSelector := generateFieldSelector(c.ExcludeNamespaces)

	// add any custom field selectors to the namespace selector
	for _, selectorString := range c.FieldSelectors {
		fieldSelector = fields.AndSelectors(fieldSelector, fields.ParseSelectorOrDie(selectorString))
	}

	// init cache to store gathered resources
	dgCache := cache.New(5*time.Minute, 30*time.Second)

	newDataGatherer := &DataGathererDynamic{
		ctx:                  ctx,
		cl:                   cl,
		k8sClientSet:         clientset,
		groupVersionResource: c.GroupVersionResource,
		fieldSelector:        fieldSelector.String(),
		namespaces:           c.IncludeNamespaces,
		cache:                dgCache,
	}

	// In order to reduce memory usage that might come from using Dynamic Informers
	// * https://github.com/kyverno/kyverno/issues/1832#issuecomment-968782166
	// * https://github.com/kubernetes/client-go/issues/832
	// * https://github.com/kubernetes/client-go/issues/871
	// we use SharedIndexInformer for known resources, these informers have less of an impact on the
	// memory usage. Dynamic datagatheres will use them for some of the native resources instead of
	// dynamic informers.

	if informerFunc, ok := kubernetesNativeResources[c.GroupVersionResource]; ok {
		factory := informers.NewSharedInformerFactoryWithOptions(clientset,
			60*time.Second,
			informers.WithNamespace(metav1.NamespaceAll),
			informers.WithTweakListOptions(func(options *metav1.ListOptions) {
				options.FieldSelector = fieldSelector.String()
			}))
		newDataGatherer.nativeSharedInformer = factory
		informer := informerFunc(factory)
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
		newDataGatherer.informer = informer
		return newDataGatherer, nil
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		cl,
		60*time.Second,
		metav1.NamespaceAll,
		func(options *metav1.ListOptions) { options.FieldSelector = fieldSelector.String() },
	)
	resourceInformer := factory.ForResource(c.GroupVersionResource)
	informer := resourceInformer.Informer()
	newDataGatherer.dynamicSharedInformer = factory
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
	newDataGatherer.informer = informer

	return newDataGatherer, nil
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
	// The k8s clientset used for fetching known resources.
	k8sClientSet kubernetes.Interface
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
	informer              k8scache.SharedIndexInformer
	dynamicSharedInformer dynamicinformer.DynamicSharedInformerFactory
	nativeSharedInformer  informers.SharedInformerFactory

	// isInitialized is set to true when data is first collected, prior to
	// this the fetch method will return an error
	isInitialized bool
}

// Run starts the dynamic data gatherer's informers for resource collection.
// Returns error if the data gatherer informer wasn't initialized
func (g *DataGathererDynamic) Run(stopCh <-chan struct{}) error {
	if g.dynamicSharedInformer == nil && g.nativeSharedInformer == nil {
		return fmt.Errorf("informer was not initialized, impossible to start")
	}

	// attach WatchErrorHandler, it needs to be set before starting an informer
	err := g.informer.SetWatchErrorHandler(func(r *k8scache.Reflector, err error) {
		if strings.Contains(fmt.Sprintf("%s", err), "the server could not find the requested resource") {
			log.Printf("server missing resource for datagatherer of %q ", g.groupVersionResource)
		} else {
			log.Printf("datagatherer informer for %q has failed and is backing off due to error: %s", g.groupVersionResource, err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to SetWatchErrorHandler on informer: %s", err)
	}

	// start shared informer
	if g.dynamicSharedInformer != nil {
		g.dynamicSharedInformer.Start(stopCh)
	}

	if g.nativeSharedInformer != nil {
		g.nativeSharedInformer.Start(stopCh)
	}

	return nil
}

// WaitForCacheSync waits for the data gatherer's informers cache to sync
// before collecting the resources.
func (g *DataGathererDynamic) WaitForCacheSync(stopCh <-chan struct{}) error {
	if !k8scache.WaitForCacheSync(stopCh, g.informer.HasSynced) {
		return fmt.Errorf("timed out waiting for Kubernetes caches to sync")
	}

	return nil
}

// Delete will flush the cache being used to stored resources gathered by the
// informer
func (g *DataGathererDynamic) Delete() error {
	g.cache.Flush()
	return nil
}

// Fetch will fetch the requested data from the apiserver, or return an error
// if fetching the data fails.
func (g *DataGathererDynamic) Fetch() (interface{}, int, error) {
	if g.groupVersionResource.String() == "" {
		return nil, -1, fmt.Errorf("resource type must be specified")
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
		if resource, ok := cacheObject.Resource.(cacheResource); ok {
			namespace := resource.GetNamespace()
			if isIncludedNamespace(namespace, fetchNamespaces) {
				items = append(items, cacheObject)
			}
			continue
		}
		return nil, -1, fmt.Errorf("failed to parse cached resource")
	}

	// Redact Secret data
	err := redactList(items)
	if err != nil {
		return nil, -1, errors.WithStack(err)
	}

	// add gathered resources to items
	list["items"] = items

	return list, len(items), nil
}

func redactList(list []*api.GatheredResource) error {
	for i := range list {
		if item, ok := list[i].Resource.(*unstructured.Unstructured); ok {
			// Determine the kind of items in case this is a generic 'mixed' list.
			gvks, _, err := scheme.Scheme.ObjectKinds(item)
			if err != nil {
				return errors.WithStack(err)
			}

			resource := item

			// Redact item if it is a:
			for _, gvk := range gvks {
				// secret object
				if gvk.Kind == "Secret" && (gvk.Group == "core" || gvk.Group == "") {
					Select(SecretSelectedFields, resource)

					// route object
				} else if gvk.Kind == "Route" && gvk.Group == "route.openshift.io" {
					Select(RouteSelectedFields, resource)
				}
			}

			// remove managedFields from all resources
			Redact(RedactFields, resource)
			continue
		}

		// objectMeta interface is used to give resources from sharedIndexInformers, (core.Pod|apps.Deployment), a common interface
		// with access to the metav1.Object
		type objectMeta interface{ GetObjectMeta() metav1.Object }
		// all objects fetched from sharedIndexInformers is now redacted
		// removing the managedFields and `kubectl.kubernetes.io/last-applied-configuration` annotation
		if item, ok := list[i].Resource.(objectMeta); ok {
			item.GetObjectMeta().SetManagedFields(nil)
			delete(item.GetObjectMeta().GetAnnotations(), "kubectl.kubernetes.io/last-applied-configuration")

			resource := item.(runtime.Object)
			gvks, _, err := scheme.Scheme.ObjectKinds(resource)
			if err != nil {
				return errors.WithStack(err)
			}

			// During the internal marshal/unmarshal the runtime.Object the metav1.TypeMeta seems to be lost
			// this section reassigns the TypeMeta to the resource
			for _, gvk := range gvks {
				if len(gvk.Kind) == 0 {
					continue
				}
				if len(gvk.Version) == 0 || gvk.Version == runtime.APIVersionInternal {
					continue
				}
				resource.GetObjectKind().SetGroupVersionKind(gvk)
				break
			}

			continue
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
func generateFieldSelector(excludeNamespaces []string) fields.Selector {
	fieldSelector := fields.Nothing()
	for _, excludeNamespace := range excludeNamespaces {
		if excludeNamespace == "" {
			continue
		}
		fieldSelector = fields.AndSelectors(fields.OneTermNotEqualSelector("metadata.namespace", excludeNamespace), fieldSelector)
	}
	return fieldSelector
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

func isNativeResource(gvr schema.GroupVersionResource) bool {
	_, ok := kubernetesNativeResources[gvr]
	return ok
}
