package k8s

// The venafi-kubernetes-agent has a requirement that **all** resources should
// be uploaded, even short-lived secrets, which are created and deleted
// in-between data uploads. A cache was added to the datagatherer code, to
// satisfy this requirement. The cache stores all resources for 5 minutes. And
// the informer event handlers (onAdd, onUpdate, onDelete) update the cache
// accordingly. The onDelete handler does not remove the object from the cache,
// but instead marks the object as deleted by setting the DeletedAt field on the
// GatheredResource. This ensures that deleted resources are still present in
// the cache for the duration of the cache expiry time.
//
// The cache expiry is hard coded to 5 minutes, which is longer than the
// venafi-kubernetes-agent default upload interval of 1 minute. This means that
// even if a resource is created and deleted in-between data gatherer runs, it
// will still be present in the cache when the data gatherer runs.
//
// TODO(wallrj): When the agent is deployed as CyberArk disco-agent, the deleted
// items are currently discarded before upload. If this remains the case, then the cache is unnecessary
// and should be disabled to save memory.
// If, in the future, the CyberArk Discovery and Context service does want to
// see deleted items, the "deleted resource reporting mechanism" will need to be
// redesigned, so that deleted items are retained for the duration of the upload
// interval.
//
// TODO(wallrj): When the agent is deployed as CyberArk disco-agent, the upload
// interval is 12 hours by default, so the 5 minute cache expiry is not
// sufficient.
//
// TODO(wallrj): The shared informer is configured to refresh all relist all
// resources every 1 minute, which will cause unnecessary load on the apiserver.
// We need to look back at the Git history and understand whether this was done
// for good reason or due to some misunderstanding.

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

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
	// IncludeResourcesByLabels filters to include only resources that have all of the specified labels.
	// This controls which resources are collected, not which labels are included.
	IncludeResourcesByLabels map[string]string `yaml:"include-resources-by-labels"`
	// ExcludeResourcesByLabels filters to exclude resources that have any of the specified labels.
	// This controls which resources are collected, not which labels are excluded.
	ExcludeResourcesByLabels map[string]string `yaml:"exclude-resources-by-labels"`
	// IncludeResourcesByAnnotations filters to include only resources that have all of the specified annotations.
	// This controls which resources are collected, not which annotations are included.
	IncludeResourcesByAnnotations map[string]string `yaml:"include-resources-by-annotations"`
	// ExcludeResourcesByAnnotations filters to exclude resources that have any of the specified annotations.
	// This controls which resources are collected, not which annotations are excluded.
	ExcludeResourcesByAnnotations map[string]string `yaml:"exclude-resources-by-annotations"`
}

// UnmarshalYAML unmarshals the ConfigDynamic resolving GroupVersionResource.
func (c *ConfigDynamic) UnmarshalYAML(unmarshal func(any) error) error {
	aux := struct {
		KubeConfigPath string `yaml:"kubeconfig"`
		ResourceType   struct {
			Group    string `yaml:"group"`
			Version  string `yaml:"version"`
			Resource string `yaml:"resource"`
		} `yaml:"resource-type"`
		ExcludeNamespaces             []string          `yaml:"exclude-namespaces"`
		IncludeNamespaces             []string          `yaml:"include-namespaces"`
		FieldSelectors                []string          `yaml:"field-selectors"`
		IncludeResourcesByLabels      map[string]string `yaml:"include-resources-by-labels"`
		ExcludeResourcesByLabels      map[string]string `yaml:"exclude-resources-by-labels"`
		IncludeResourcesByAnnotations map[string]string `yaml:"include-resources-by-annotations"`
		ExcludeResourcesByAnnotations map[string]string `yaml:"exclude-resources-by-annotations"`
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
	c.IncludeResourcesByLabels = aux.IncludeResourcesByLabels
	c.ExcludeResourcesByLabels = aux.ExcludeResourcesByLabels
	c.IncludeResourcesByAnnotations = aux.IncludeResourcesByAnnotations
	c.ExcludeResourcesByAnnotations = aux.ExcludeResourcesByAnnotations

	return nil
}

// validate validates the configuration.
func (c *ConfigDynamic) validate() error {
	var errs []string
	if len(c.ExcludeNamespaces) > 0 && len(c.IncludeNamespaces) > 0 {
		errs = append(errs, "cannot set excluded and included namespaces")
	}

	if len(c.ExcludeResourcesByLabels) > 0 && len(c.IncludeResourcesByLabels) > 0 {
		errs = append(errs, "cannot use both include-resources-by-labels and exclude-resources-by-labels")
	}

	if len(c.ExcludeResourcesByAnnotations) > 0 && len(c.IncludeResourcesByAnnotations) > 0 {
		errs = append(errs, "cannot use both include-resources-by-annotations and exclude-resources-by-annotations")
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
	corev1.SchemeGroupVersion.WithResource("configmaps"): func(sharedFactory informers.SharedInformerFactory) k8scache.SharedIndexInformer {
		return sharedFactory.Core().V1().ConfigMaps().Informer()
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
			return nil, err
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
		groupVersionResource: c.GroupVersionResource,
		fieldSelector:        fieldSelector.String(),
		namespaces:           c.IncludeNamespaces,
		cache:                dgCache,
		includeLabels:        c.IncludeResourcesByLabels,
		excludeLabels:        c.ExcludeResourcesByLabels,
		includeAnnotations:   c.IncludeResourcesByAnnotations,
		excludeAnnotations:   c.ExcludeResourcesByAnnotations,
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
			// TODO(wallrj): This causes all resources to be relisted every 1
			// minute which will cause unnecessary load on the apiserver.
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
			// TODO(wallrj): This causes all resources to be relisted every 1
			// minute which will cause unnecessary load on the apiserver.
			60*time.Second,
			metav1.NamespaceAll,
			func(options *metav1.ListOptions) {
				options.FieldSelector = fieldSelector.String()
			},
		)
		newDataGatherer.informer = factory.ForResource(c.GroupVersionResource).Informer()
	}

	registration, err := newDataGatherer.informer.AddEventHandlerWithOptions(k8scache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			onAdd(log, obj, dgCache)
		},
		UpdateFunc: func(oldObj, newObj any) {
			onUpdate(log, oldObj, newObj, dgCache)
		},
		DeleteFunc: func(obj any) {
			onDelete(log, obj, dgCache)
		},
	}, k8scache.HandlerOptions{
		Logger: &log,
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

	// includeLabels and excludeLabels filter resources based on their labels
	includeLabels map[string]string
	excludeLabels map[string]string
	// includeAnnotations and excludeAnnotations filter resources based on their annotations
	includeAnnotations map[string]string
	excludeAnnotations map[string]string
}

// Run starts the dynamic data gatherer's informers for resource collection.
// Returns error if the data gatherer informer wasn't initialized, Run blocks
// until the stopCh is closed.
func (g *DataGathererDynamic) Run(ctx context.Context) error {
	log := klog.FromContext(ctx)
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
	g.informer.RunWithContext(ctx)

	return nil
}

var ErrCacheSyncTimeout = fmt.Errorf("timed out waiting for Kubernetes cache to sync")

// WaitForCacheSync waits for the data gatherer's informers cache to sync before
// collecting the resources. Use errors.Is(err, ErrCacheSyncTimeout) to check if
// the cache sync failed.
func (g *DataGathererDynamic) WaitForCacheSync(ctx context.Context) error {
	// Don't use WaitForNamedCacheSync, since we don't want to log extra messages.
	if !k8scache.WaitForCacheSync(ctx.Done(), g.registration.HasSynced) {
		return ErrCacheSyncTimeout
	}

	return nil
}

// Fetch will fetch the requested data from the apiserver, or return an error
// if fetching the data fails.
func (g *DataGathererDynamic) Fetch() (any, int, error) {
	if g.groupVersionResource.String() == "" {
		return nil, -1, fmt.Errorf("resource type must be specified")
	}

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
			if !isIncludedNamespace(namespace, fetchNamespaces) {
				continue
			}

			// filter by labels
			labels := resource.GetLabels()
			if !matchesLabelFilter(labels, g.includeLabels, g.excludeLabels) {
				continue
			}

			// filter by annotations
			annotations := resource.GetAnnotations()
			if !matchesAnnotationFilter(annotations, g.includeAnnotations, g.excludeAnnotations) {
				continue
			}

			items = append(items, cacheObject)
			continue
		}
		return nil, -1, fmt.Errorf("failed to parse cached resource")
	}

	// Redact Secret data
	err := redactList(items, g.ExcludeAnnotKeys, g.ExcludeLabelKeys)
	if err != nil {
		return nil, -1, err
	}

	return &api.DynamicData{
		Items: items,
	}, len(items), nil
}

// redactList removes sensitive and superfluous data from the supplied resource list.
// All resources have superfluous managed-data fields removed.
// All resources have sensitive labels and annotations removed.
// Secret and Route are processed as special cases. For these
// resources there is an allow-list of fields that should be retained.
// For Secret resources, the `data` is redacted, to prevent private keys or sensitive
// data being collected; only the tls.crt and ca.crt data keys are retained.
// For Route resources, only the fields related to CA certificate and policy are retained.
// TODO(wallrj): A short coming of the current allow-list implementation is that
// you have to specify absolute fields paths. It is not currently possible to
// select all metadata with: `{metadata}`. This means that the metadata for
// Secret and Route has fewer fields than the metadata for all other resources.
func redactList(list []*api.GatheredResource, excludeAnnotKeys, excludeLabelKeys []*regexp.Regexp) error {
	for i := range list {
		if item, ok := list[i].Resource.(*unstructured.Unstructured); ok {
			// Determine the kind of items in case this is a generic 'mixed' list.
			gvks, _, err := scheme.Scheme.ObjectKinds(item)
			if err != nil {
				return err
			}

			resource := item

			// Redact item if it is a Secret or a Route.
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
				return err
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
	annots, ok := annotsRaw.(map[string]any)
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
	return slices.Contains(namespaces, namespace)
}

// matchesLabelFilter checks if the resource labels match the include/exclude filters.
// If includeLabels is set, all key-value pairs must match for the resource to be included.
// An empty string value means "match any value for this key" (key-only matching).
// If excludeLabels is set, any matching key-value pair will exclude the resource.
func matchesLabelFilter(resourceLabels, includeLabels, excludeLabels map[string]string) bool {
	// Check exclude labels first
	if len(excludeLabels) > 0 {
		for key, value := range excludeLabels {
			if resourceValue, exists := resourceLabels[key]; exists {
				// If exclude value is empty, exclude any resource with this key
				// Otherwise, only exclude if the value also matches
				if value == "" || resourceValue == value {
					return false
				}
			}
		}
	}

	// Check include labels
	if len(includeLabels) > 0 {
		for key, value := range includeLabels {
			resourceValue, exists := resourceLabels[key]
			if !exists {
				// Required label key is missing, filter it out
				return false
			}
			// If include value is empty, we only care that the key exists
			// Otherwise, the value must also match
			if value != "" && resourceValue != value {
				return false
			}
		}
	}

	return true
}

// matchesAnnotationFilter checks if the resource annotations match the include/exclude filters.
// If includeAnnotations is set, all key-value pairs must match for the resource to be included.
// An empty string value means "match any value for this key" (key-only matching).
// If excludeAnnotations is set, any matching key-value pair will exclude the resource.
func matchesAnnotationFilter(resourceAnnotations, includeAnnotations, excludeAnnotations map[string]string) bool {
	// Check exclude annotations first
	if len(excludeAnnotations) > 0 {
		for key, value := range excludeAnnotations {
			if resourceValue, exists := resourceAnnotations[key]; exists {
				// If exclude value is empty, exclude any resource with this key
				// Otherwise, only exclude if the value also matches
				if value == "" || resourceValue == value {
					return false
				}
			}
		}
	}

	// Check include annotations
	if len(includeAnnotations) > 0 {
		for key, value := range includeAnnotations {
			resourceValue, exists := resourceAnnotations[key]
			if !exists {
				// Required annotation key is missing, filter it out
				return false
			}
			// If include value is empty, we only care that the key exists
			// Otherwise, the value must also match
			if value != "" && resourceValue != value {
				return false
			}
		}
	}

	return true
}

func isNativeResource(gvr schema.GroupVersionResource) bool {
	_, ok := kubernetesNativeResources[gvr]
	return ok
}
