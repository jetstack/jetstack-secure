package k8s

import (
	"context"
	"fmt"
	"regexp"
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
	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/logs"
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
	var errs []string
	if len(c.ExcludeNamespaces) > 0 && len(c.IncludeNamespaces) > 0 {
		errs = append(errs, "cannot set excluded and included namespaces")
	}

	if c.GroupVersionResource.Resource == "" {
		errs = append(errs, "invalid configuration: GroupVersionResource.Resource cannot be empty")
	}

	for i, selectorString := range c.FieldSelectors {
		if selectorString == "" {
			errs = append(errs, fmt.Sprintf("invalid field selector %d: must not be empty", i))
		}
		_, err := fields.ParseSelector(selectorString)
		if err != nil {
			errs = append(errs, fmt.Sprintf("invalid field selector %d: %s", i, err))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
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
	if isNativeResource(c.GroupVersionResource) {
		clientset, err := NewClientSet(c.KubeConfigPath)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		return c.newDataGathererWithClient(ctx, nil, clientset)
	} else {
		cl, err := NewDynamicClient(c.KubeConfigPath)
		if err != nil {
			return nil, err
		}

		return c.newDataGathererWithClient(ctx, cl, nil)
	}
}

func (c *ConfigDynamic) newDataGathererWithClient(ctx context.Context, cl dynamic.Interface, clientset kubernetes.Interface) (datagatherer.DataGatherer, error) {
	log := klog.FromContext(ctx)
	if err := c.validate(); err != nil {
		return nil, err
	}
	// init shared informer for selected namespaces
	fieldSelector := generateExcludedNamespacesFieldSelector(c.ExcludeNamespaces)

	// Add any custom field selectors to the excluded namespaces selector
	// The selectors have already been validated, so it is safe to use
	// ParseSelectorOrDie here.
	for _, selectorString := range c.FieldSelectors {
		fieldSelector = fields.AndSelectors(fieldSelector, fields.ParseSelectorOrDie(selectorString))
	}

	// init cache to store gathered resources
	dgCache := cache.New(5*time.Minute, 30*time.Second)

	newDataGatherer := &DataGathererDynamic{
		ctx:                  ctx,
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
			}),
		)
		newDataGatherer.informer = informerFunc(factory)
	} else {
		factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
			cl,
			60*time.Second,
			metav1.NamespaceAll,
			func(options *metav1.ListOptions) {
				options.FieldSelector = fieldSelector.String()
			},
		)
		newDataGatherer.informer = factory.ForResource(c.GroupVersionResource).Informer()
	}

	registration, err := newDataGatherer.informer.AddEventHandler(k8scache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			onAdd(log, obj, dgCache)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			onUpdate(log, oldObj, newObj, dgCache)
		},
		DeleteFunc: func(obj interface{}) {
			onDelete(log, obj, dgCache)
		},
	})
	if err != nil {
		return nil, err
	}
	newDataGatherer.registration = registration

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
	informer     k8scache.SharedIndexInformer
	registration k8scache.ResourceEventHandlerRegistration

	ExcludeAnnotKeys []*regexp.Regexp
	ExcludeLabelKeys []*regexp.Regexp
}

// Run starts the dynamic data gatherer's informers for resource collection.
// Returns error if the data gatherer informer wasn't initialized, Run blocks
// until the stopCh is closed.
func (g *DataGathererDynamic) Run(stopCh <-chan struct{}) error {
	log := klog.FromContext(g.ctx)
	if g.informer == nil {
		return fmt.Errorf("informer was not initialized, impossible to start")
	}

	// attach WatchErrorHandler, it needs to be set before starting an informer
	err := g.informer.SetWatchErrorHandler(func(r *k8scache.Reflector, err error) {
		if strings.Contains(fmt.Sprintf("%s", err), "the server could not find the requested resource") {
			log.V(logs.Debug).Info("Server missing resource for datagatherer", "groupVersionResource", g.groupVersionResource)
		} else {
			log.Info("datagatherer informer has failed and is backing off", "groupVersionResource", g.groupVersionResource, "reason", err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to SetWatchErrorHandler on informer: %s", err)
	}

	// start shared informer
	g.informer.Run(stopCh)

	return nil
}

var ErrCacheSyncTimeout = fmt.Errorf("timed out waiting for Kubernetes cache to sync")

// WaitForCacheSync waits for the data gatherer's informers cache to sync before
// collecting the resources. Use errors.Is(err, ErrCacheSyncTimeout) to check if
// the cache sync failed.
func (g *DataGathererDynamic) WaitForCacheSync(stopCh <-chan struct{}) error {
	if !k8scache.WaitForCacheSync(stopCh, g.registration.HasSynced) {
		return ErrCacheSyncTimeout
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

	// delete expired items from the cache
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
	err := redactList(items, g.ExcludeAnnotKeys, g.ExcludeLabelKeys)
	if err != nil {
		return nil, -1, errors.WithStack(err)
	}

	// add gathered resources to items
	list["items"] = items

	return list, len(items), nil
}

func redactList(list []*api.GatheredResource, excludeAnnotKeys, excludeLabelKeys []*regexp.Regexp) error {
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
					if err := Select(SecretSelectedFields, resource); err != nil {
						return err
					}

					// route object
				} else if gvk.Kind == "Route" && gvk.Group == "route.openshift.io" {
					if err := Select(RouteSelectedFields, resource); err != nil {
						return err
					}
				}
			}

			// remove managedFields from all resources
			Redact(RedactFields, resource)

			RemoveUnstructuredKeys(excludeAnnotKeys, resource, "metadata", "annotations")
			RemoveUnstructuredKeys(excludeLabelKeys, resource, "metadata", "labels")

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

			RemoveTypedKeys(excludeAnnotKeys, item.GetObjectMeta().GetAnnotations())
			RemoveTypedKeys(excludeLabelKeys, item.GetObjectMeta().GetLabels())

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

// Meant for typed clientset objects.
func RemoveTypedKeys(excludeAnnotKeys []*regexp.Regexp, m map[string]string) {
	for key := range m {
		for _, excludeAnnotKey := range excludeAnnotKeys {
			if excludeAnnotKey.MatchString(key) {
				delete(m, key)
			}
		}
	}
}

// Meant for unstructured clientset objects. Removes the keys from the field
// given as input. For example, let's say we have the following object:
//
//	{
//	  "metadata": {
//	    "annotations": {
//	      "key1": "value1",
//	      "key2": "value2"
//	    }
//	  }
//	}
//
// Then, the following call:
//
//	RemoveUnstructuredKeys("^key1$", obj, "metadata", "annotations")
//
// Will result in:
//
//	{
//	  "metadata": {
//	    "annotations": {"key2": "value2"}
//	  }
//	}
//
// If the given path doesn't exist or leads to a non-map object, nothing
// happens. The leaf object must either be a map[string]interface{} (that's
// what's returned by the unstructured clientset) or a map[string]string (that's
// what's returned by the typed clientset).
func RemoveUnstructuredKeys(excludeKeys []*regexp.Regexp, obj *unstructured.Unstructured, path ...string) {
	annotsRaw, ok, err := unstructured.NestedFieldNoCopy(obj.Object, path...)
	if err != nil {
		return
	}
	if !ok {
		return
	}

	// The field may be nil since yaml.Unmarshal's omitempty might not be set
	// on this struct field.
	if annotsRaw == nil {
		return
	}

	// The only possible type in an unstructured.Unstructured object is
	// map[string]interface{}. That's because the yaml.Unmarshal func is used
	// with an empty map[string]interface{} object, which means all nested
	// objects will be unmarshalled to a map[string]interface{}.
	annots, ok := annotsRaw.(map[string]interface{})
	if !ok {
		return
	}

	for key := range annots {
		for _, excludeAnnotKey := range excludeKeys {
			if excludeAnnotKey.MatchString(key) {
				delete(annots, key)
			}
		}
	}
}

// generateExcludedNamespacesFieldSelector creates a field selector string from
// a list of namespaces to exclude.
func generateExcludedNamespacesFieldSelector(excludeNamespaces []string) fields.Selector {
	var selectors []fields.Selector
	for _, excludeNamespace := range excludeNamespaces {
		if excludeNamespace == "" {
			continue
		}
		selectors = append(selectors, fields.OneTermNotEqualSelector("metadata.namespace", excludeNamespace))
	}
	return fields.AndSelectors(selectors...)
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
