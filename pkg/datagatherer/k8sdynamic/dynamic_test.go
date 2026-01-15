package k8sdynamic

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/informers"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	k8scache "k8s.io/client-go/tools/cache"

	"github.com/jetstack/preflight/api"
)

func getObject(version, kind, name, namespace string, withManagedFields bool) *unstructured.Unstructured {
	metadata := map[string]any{
		"name":      name,
		"namespace": namespace,
		"uid":       fmt.Sprintf("%s1", name),
	}

	if withManagedFields {
		// []metav1.FieldsV1{} can't be deep copied by fake client so using
		// string as example value
		metadata["managedFields"] = "set"
	}

	object := map[string]any{
		"apiVersion": version,
		"kind":       kind,
		"metadata":   metadata,
	}

	return &unstructured.Unstructured{
		Object: object,
	}
}

func getObjectAnnot(version, kind, name, namespace string, annotations, labels map[string]any) *unstructured.Unstructured {
	obj := getObject(version, kind, name, namespace, false)

	metadata, _ := obj.Object["metadata"].(map[string]any)
	if annotations == nil {
		annotations = make(map[string]any)
	}
	metadata["annotations"] = annotations
	metadata["labels"] = labels

	return obj
}

func getSecret(name, namespace string, data map[string]any, isTLS bool, withLastApplied bool) *unstructured.Unstructured {
	object := getObject("v1", "Secret", name, namespace, false)

	if data != nil {
		object.Object["data"] = data
	}

	object.Object["type"] = "Opaque"
	if isTLS {
		object.Object["type"] = "kubernetes.io/tls"
	}

	metadata, _ := object.Object["metadata"].(map[string]any)
	annotations := make(map[string]any)

	// if we're creating a 'raw' secret as scraped that was applied by kubectl
	if withLastApplied {
		jsonData, _ := json.Marshal(data)
		annotations["kubectl.kubernetes.io/last-applied-configuration"] = string(jsonData)
	}

	metadata["annotations"] = annotations

	return object
}

func sortGatheredResources(list []*api.GatheredResource) {
	if len(list) > 1 {
		sort.SliceStable(list, func(i, j int) bool {
			var itemA, itemB string
			// unstructured
			if item, ok := list[i].Resource.(*unstructured.Unstructured); ok {
				itemA = item.GetName()
			}
			if item, ok := list[j].Resource.(*unstructured.Unstructured); ok {
				itemB = item.GetName()
			}

			// pods
			if item, ok := list[i].Resource.(*corev1.Pod); ok {
				itemA = item.GetName()
			}
			if item, ok := list[j].Resource.(*corev1.Pod); ok {
				itemB = item.GetName()
			}
			return itemA < itemB
		})
	}
}

func TestNewDataGathererWithClientAndDynamicInformer(t *testing.T) {
	ctx := t.Context()
	config := ConfigDynamic{
		ExcludeNamespaces:    []string{"kube-system"},
		GroupVersionResource: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
		FieldSelectors: []string{
			"type!=kubernetes.io/service-account-token",
			"type!=kubernetes.io/dockercfg",
		},
	}
	cl := fake.NewSimpleDynamicClient(runtime.NewScheme())
	dg, err := config.newDataGathererWithClient(ctx, cl, nil)

	if err != nil {
		t.Errorf("expected no error but got: %v", err)
	}

	expected := &DataGathererDynamic{
		groupVersionResource: config.GroupVersionResource,
		// it's important that the namespaces are set as the IncludeNamespaces
		// during initialization
		namespaces:    config.IncludeNamespaces,
		fieldSelector: "metadata.namespace!=kube-system,type!=kubernetes.io/service-account-token,type!=kubernetes.io/dockercfg",
	}

	gatherer := dg.(*DataGathererDynamic)
	// test gatherer's fields
	if !reflect.DeepEqual(gatherer.groupVersionResource, expected.groupVersionResource) {
		t.Errorf("expected %v, got %v", expected, dg)
	}
	if !reflect.DeepEqual(gatherer.namespaces, expected.namespaces) {
		t.Errorf("expected %v, got %v", expected, dg)
	}
	if gatherer.cache == nil {
		t.Errorf("unexpected cache value: %v", nil)
	}
	if gatherer.informer == nil {
		t.Errorf("unexpected resource informer value: %v", nil)
	}
	if gatherer.registration == nil {
		t.Errorf("unexpected resource event handler registration value: %v", nil)
	}
	if !reflect.DeepEqual(gatherer.fieldSelector, expected.fieldSelector) {
		t.Errorf("expected %v, got %v", expected.fieldSelector, gatherer.fieldSelector)
	}
}

func TestNewDataGathererWithClientAndSharedIndexInformer(t *testing.T) {
	ctx := t.Context()
	config := ConfigDynamic{
		IncludeNamespaces:    []string{"a"},
		GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}
	clientset := fakeclientset.NewSimpleClientset()
	dg, err := config.newDataGathererWithClient(ctx, nil, clientset)
	if err != nil {
		t.Errorf("expected no error but got: %v", err)
	}

	expected := &DataGathererDynamic{
		groupVersionResource: config.GroupVersionResource,
		// it's important that the namespaces are set as the IncludeNamespaces
		// during initialization
		namespaces: config.IncludeNamespaces,
	}

	gatherer := dg.(*DataGathererDynamic)
	// test gatherer's fields
	if !reflect.DeepEqual(gatherer.groupVersionResource, expected.groupVersionResource) {
		t.Errorf("expected %v, got %v", expected, dg)
	}
	if !reflect.DeepEqual(gatherer.namespaces, expected.namespaces) {
		t.Errorf("expected %v, got %v", expected, dg)
	}
	if gatherer.cache == nil {
		t.Errorf("unexpected cache value: %v", nil)
	}
	if gatherer.informer == nil {
		t.Errorf("unexpected resource informer value: %v", nil)
	}
	if gatherer.registration == nil {
		t.Errorf("unexpected event handler registration value: %v", nil)
	}
}

func TestUnmarshalDynamicConfig(t *testing.T) {
	textCfg := `
kubeconfig: "/home/someone/.kube/config"
resource-type:
  group: "g"
  version: "v"
  resource: "r"
exclude-namespaces:
- kube-system
- my-namespace
# this config is invalid, but the validation is tested elsewhere
# include-namespaces is here just to ensure that they are loaded
# from the config file
include-namespaces:
- default
field-selectors:
- type!=kubernetes.io/service-account-token
`

	expectedGVR := schema.GroupVersionResource{
		Group:    "g",
		Version:  "v",
		Resource: "r",
	}

	expectedExcludeNamespaces := []string{
		"kube-system",
		"my-namespace",
	}

	expectedIncludeNamespaces := []string{"default"}

	expectedFieldSelectors := []string{
		"type!=kubernetes.io/service-account-token",
	}

	cfg := ConfigDynamic{}
	err := yaml.Unmarshal([]byte(textCfg), &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}

	if got, want := cfg.KubeConfigPath, "/home/someone/.kube/config"; got != want {
		t.Errorf("KubeConfigPath does not match: got=%q; want=%q", got, want)
	}

	if got, want := cfg.GroupVersionResource, expectedGVR; !reflect.DeepEqual(got, want) {
		t.Errorf("GroupVersionResource does not match: got=%+v want=%+v", got, want)
	}

	if got, want := cfg.ExcludeNamespaces, expectedExcludeNamespaces; !reflect.DeepEqual(got, want) {
		t.Errorf("ExcludeNamespaces does not match: got=%+v want=%+v", got, want)
	}
	if got, want := cfg.IncludeNamespaces, expectedIncludeNamespaces; !reflect.DeepEqual(got, want) {
		t.Errorf("IncludeNamespaces does not match: got=%+v want=%+v", got, want)
	}
	if got, want := cfg.FieldSelectors, expectedFieldSelectors; !reflect.DeepEqual(got, want) {
		t.Errorf("FieldSelectors does not match: got=%+v want=%+v", got, want)
	}
}

func TestConfigDynamicValidate(t *testing.T) {
	tests := []struct {
		Config        ConfigDynamic
		ExpectedError string
	}{
		{
			Config: ConfigDynamic{
				GroupVersionResource: schema.GroupVersionResource{
					Group:    "",
					Version:  "",
					Resource: "",
				},
			},
			ExpectedError: "invalid configuration: GroupVersionResource.Resource cannot be empty",
		},
		{
			Config: ConfigDynamic{
				IncludeNamespaces: []string{"a"},
				ExcludeNamespaces: []string{"b"},
			},
			ExpectedError: "cannot set excluded and included namespaces",
		},
		{
			Config: ConfigDynamic{
				GroupVersionResource: schema.GroupVersionResource{
					Group:    "",
					Version:  "v1",
					Resource: "secrets",
				},
				FieldSelectors: []string{""},
			},
			ExpectedError: "invalid field selector 0: must not be empty",
		},
		{
			Config: ConfigDynamic{
				GroupVersionResource: schema.GroupVersionResource{
					Group:    "",
					Version:  "v1",
					Resource: "secrets",
				},
				FieldSelectors: []string{"foo"},
			},
			ExpectedError: "invalid field selector 0: invalid selector: 'foo'; can't understand 'foo'",
		},
	}

	for _, test := range tests {
		err := test.Config.validate()
		if err == nil && test.ExpectedError != "" {
			t.Errorf("expected error: %q, got: nil", test.ExpectedError)
		}
		if err != nil && !strings.Contains(err.Error(), test.ExpectedError) {
			t.Errorf("expected %s, got %s", test.ExpectedError, err.Error())
		}
	}
}

func TestGenerateExcludedNamespacesFieldSelector(t *testing.T) {
	tests := []struct {
		ExcludeNamespaces     []string
		ExpectedFieldSelector string
	}{
		{
			ExcludeNamespaces: []string{
				"",
			},
			ExpectedFieldSelector: "",
		},
		{
			ExcludeNamespaces: []string{
				"kube-system",
			},
			ExpectedFieldSelector: "metadata.namespace!=kube-system",
		},
		{
			ExcludeNamespaces: []string{
				"kube-system",
				"my-namespace",
			},
			ExpectedFieldSelector: "metadata.namespace!=kube-system,metadata.namespace!=my-namespace",
		},
	}

	for _, test := range tests {
		fieldSelector := generateExcludedNamespacesFieldSelector(test.ExcludeNamespaces).String()
		if fieldSelector != test.ExpectedFieldSelector {
			t.Errorf("ExpectedFieldSelector does not match: got=%+v want=%+v", fieldSelector, test.ExpectedFieldSelector)
		}
	}
}

// fake time for testing
type fakeTime struct {
}

func (f *fakeTime) now() time.Time {
	//2021-03-16T18:22:15+00:00
	return time.Unix(1615918935, 0)
}

func init() {
	clock = &fakeTime{}
}

func TestDynamicGatherer_Fetch(t *testing.T) {
	// start a k8s client
	// init the datagatherer's informer with the client
	// add/delete resources watched by the data gatherer
	// check the expected result
	tests := map[string]struct {
		config            ConfigDynamic
		excludeAnnotsKeys []string
		excludeLabelKeys  []string
		addObjects        []runtime.Object
		deleteObjects     map[string]string
		updateObjects     map[string]runtime.Object
		expected          []*api.GatheredResource
		err               bool
	}{
		"fetches the default namespace": {
			addObjects: []runtime.Object{
				getObject("v1", "Namespace", "default", "", false),
			},
			config: ConfigDynamic{
				IncludeNamespaces:    []string{""},
				GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			},
			expected: []*api.GatheredResource{
				{
					Resource: &unstructured.Unstructured{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "Namespace",
							"metadata": map[string]any{
								"name": "default",
								"uid":  "default1",
							},
						},
					},
				},
			},
		},
		"only a Foo should be returned if GVR selects foos": {
			addObjects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				getObject("v1", "Service", "testservice", "testns", false),
				getObject("foobar/v1", "NotFoo", "notfoo", "testns", false),
			},
			config: ConfigDynamic{
				IncludeNamespaces:    []string{"testns"},
				GroupVersionResource: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
			},
			expected: []*api.GatheredResource{
				{
					Resource: getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				},
			},
		},
		"delete a Foo resource from the testns, the cache should have a Foo with deletedAt set to now()": {
			addObjects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				getObject("v1", "Service", "testservice", "testns", false),
				getObject("foobar/v1", "NotFoo", "notfoo", "testns", false),
			},
			deleteObjects: map[string]string{
				"testns": "testfoo",
			},
			config: ConfigDynamic{
				IncludeNamespaces:    []string{"testns"},
				GroupVersionResource: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
			},
			expected: []*api.GatheredResource{
				{
					Resource:  getObject("foobar/v1", "Foo", "testfoo", "testns", false),
					DeletedAt: api.Time{Time: clock.now()},
				},
			},
		},
		"only Foos in the specified namespace should be returned": {
			config: ConfigDynamic{
				IncludeNamespaces:    []string{"testns"},
				GroupVersionResource: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
			},
			addObjects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				getObject("foobar/v1", "Foo", "testfoo", "nottestns", false),
			},
			expected: []*api.GatheredResource{
				{
					Resource: getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				},
			},
		},
		"Foos in different namespaces should be returned if no namespace field is set": {
			config: ConfigDynamic{
				IncludeNamespaces:    []string{""},
				GroupVersionResource: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
			},
			addObjects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo1", "testns1", false),
				getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
			},
			expected: []*api.GatheredResource{
				{
					Resource: getObject("foobar/v1", "Foo", "testfoo1", "testns1", false),
				},
				{
					Resource: getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
				},
			},
		},
		"DeleteFoos in different namespaces should be returned if no namespace field is set": {
			config: ConfigDynamic{
				IncludeNamespaces:    []string{""},
				GroupVersionResource: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
			},
			addObjects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo1", "testns1", false),
				getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
			},
			expected: []*api.GatheredResource{
				{
					Resource: getObject("foobar/v1", "Foo", "testfoo1", "testns1", false),
				},
				{
					Resource: getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
				},
			},
		},
		"Delete all Foo resources, all the fetched resources should have a deletedAt field set to now()": {
			config: ConfigDynamic{
				IncludeNamespaces:    []string{""},
				GroupVersionResource: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
			},
			deleteObjects: map[string]string{
				"testns1": "testfoo1",
				"testns2": "testfoo2",
			},
			addObjects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo1", "testns1", false),
				getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
			},
			expected: []*api.GatheredResource{
				{
					Resource:  getObject("foobar/v1", "Foo", "testfoo1", "testns1", false),
					DeletedAt: api.Time{Time: clock.now()},
				},
				{
					Resource:  getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
					DeletedAt: api.Time{Time: clock.now()},
				},
			},
		},
		"Update all Foo resources, all the fetched resources should have been updated": {
			config: ConfigDynamic{
				IncludeNamespaces:    []string{""},
				GroupVersionResource: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
			},
			updateObjects: map[string]runtime.Object{
				"testns1": getObject("foobar/v1", "Foo", "testfoo1", "testns1", false),
				"testns2": getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
			},
			addObjects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo1", "testns1", false),
				getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
			},
			expected: []*api.GatheredResource{
				{
					Resource: getObject("foobar/v1", "Foo", "testfoo1", "testns1", false),
				},
				{
					Resource: getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
				},
			},
		},
		"Secret resources should have data removed": {
			config: ConfigDynamic{
				IncludeNamespaces:    []string{""},
				GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
			},
			addObjects: []runtime.Object{
				getSecret("testsecret", "testns1", map[string]any{
					"secretKey": "secretValue",
				}, false, true),
				getSecret("anothertestsecret", "testns2", map[string]any{
					"secretNumber": "12345",
				}, false, true),
			},
			expected: []*api.GatheredResource{
				{
					Resource: getSecret("testsecret", "testns1", nil, false, false),
				},
				{
					Resource: getSecret("anothertestsecret", "testns2", nil, false, false),
				},
			},
		},
		"Secret of type kubernetes.io/tls should have crts and not keys": {
			config: ConfigDynamic{
				IncludeNamespaces:    []string{""},
				GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
			},
			addObjects: []runtime.Object{
				getSecret("testsecret", "testns1", map[string]any{
					"tls.key": "secretValue",
					"tls.crt": "value",
					"ca.crt":  "value",
				}, true, true),
				getSecret("anothertestsecret", "testns2", map[string]any{
					"example.key": "secretValue",
					"example.crt": "value",
				}, true, true),
			},
			expected: []*api.GatheredResource{
				{
					// only tls.crt and ca.cert remain
					Resource: getSecret("testsecret", "testns1", map[string]any{
						"tls.crt": "value",
						"ca.crt":  "value",
					}, true, false),
				},
				{
					// all other keys removed
					Resource: getSecret("anothertestsecret", "testns2", nil, true, false),
				},
			},
		},
		"excluded annotations are removed for unstructured-based gatherers such as secrets": {
			config: ConfigDynamic{GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}},

			// To give a realistic regex in this test case, let's use the
			// example of the Kapp project that uses four annotations that all
			// start with `kapp.k14s.io/original*`. These annotations are
			// similar to `kubectl.kubernetes.io/last-applied-configuration` in
			// that they may contain sensitive information. From [1], they may
			// look like this:
			//
			//  kapp.k14s.io/original: |
			//    {"apiVersion":"v1","kind":"Secret","spec":{"data": {"password": "cGFzc3dvcmQ=","username": "bXl1c2VybmFtZQ=="}}}
			//  kapp.k14s.io/original-diff: |
			//    - type: test
			//      path: /data
			//      value:
			//      password: cygpcGVyUzNjcmV0UEBhc3N3b3JkIQ==
			//      username: bXl1c2VybmFtZQ==
			//
			//  [1]: https://github.com/carvel-dev/kapp/issues/90#issuecomment-602074356
			//
			// The regular expression could be:
			excludeAnnotsKeys: []string{`^kapp\.k14s\.io/original.*`},

			// A somewhat realistic example of labels that would need to be
			// excluded would be when a company declares ownership using
			// sensitive identifiers (e.g., employee IDs), and the company
			// doesn't want these IDs to be exposed. Let's imagine these
			// employee IDs look like this:
			//
			//  company.com/employee-id: 12345
			//
			// The regular expression would then be:
			excludeLabelKeys: []string{`^company\.com/employee-id$`},

			addObjects: []runtime.Object{getObjectAnnot("v1", "Secret", "s0", "n1",
				map[string]any{"kapp.k14s.io/original": "foo", "kapp.k14s.io/original-diff": "bar", "normal": "true"},
				map[string]any{`company.com/employee-id`: "12345", "prod": "true"},
			)},
			expected: []*api.GatheredResource{{Resource: getObjectAnnot("v1", "Secret", "s0", "n1",
				map[string]any{"normal": "true"},
				map[string]any{"prod": "true"},
			)}},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var wg sync.WaitGroup
			ctx := t.Context()
			gvrToListKind := map[schema.GroupVersionResource]string{
				{Group: "foobar", Version: "v1", Resource: "foos"}:      "UnstructuredList",
				{Group: "apps", Version: "v1", Resource: "deployments"}: "UnstructuredList",
				{Group: "", Version: "v1", Resource: "secrets"}:         "UnstructuredList",
				{Group: "", Version: "v1", Resource: "namespaces"}:      "UnstructuredList",
			}
			cl := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, tc.addObjects...)
			// init the datagatherer's informer with the client
			dg, err := tc.config.newDataGathererWithClient(ctx, cl, nil)
			if err != nil {
				t.Fatalf("unexpected error: %+v", err)
			}

			// initializing test informer, this informer will update the waitGroup making sure all the
			// update and delete events have all been capture by the informers, the 100 mills sleep is
			// just to make sure dg informer is caught up. This allows us to wait until the waitGroup is
			// done before doing the dg.Fetch.
			factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(cl, 10*time.Minute, metav1.NamespaceAll, nil)
			resourceInformer := factory.ForResource(tc.config.GroupVersionResource)
			testInformer := resourceInformer.Informer()
			_, err = testInformer.AddEventHandler(k8scache.ResourceEventHandlerFuncs{
				DeleteFunc: func(obj any) {
					defer wg.Done()
					time.Sleep(100 * time.Millisecond)
				},
				UpdateFunc: func(oldObj, newObj any) {
					defer wg.Done()
					time.Sleep(100 * time.Millisecond)
				},
			})
			require.NoError(t, err)
			// start test Informer
			factory.Start(ctx.Done())
			k8scache.WaitForCacheSync(ctx.Done(), testInformer.HasSynced)

			dgd := dg.(*DataGathererDynamic)
			for _, key := range tc.excludeAnnotsKeys {
				dgd.ExcludeAnnotKeys = append(dgd.ExcludeAnnotKeys, regexp.MustCompile(key))
			}
			for _, key := range tc.excludeLabelKeys {
				dgd.ExcludeLabelKeys = append(dgd.ExcludeLabelKeys, regexp.MustCompile(key))
			}

			// start data gatherer informer
			dynamiDg := dg
			go func() {
				if err = dynamiDg.Run(ctx); err != nil {
					t.Errorf("unexpected client error: %+v", err)
				}
			}()
			err = dynamiDg.WaitForCacheSync(ctx)
			if err != nil {
				t.Fatalf("unexpected client error: %+v", err)
			}

			// deletes all the objects set to be deleted, to trigger
			// a delete event in the informers. Add 1 to wg making "sure" (https://github.com/kubernetes/kubernetes/issues/95372)
			// the informers cache are sync
			for ns, delete := range tc.deleteObjects {
				wg.Add(1)
				deletePolicy := metav1.DeletePropagationForeground
				deleteOptions := metav1.DeleteOptions{
					PropagationPolicy: &deletePolicy,
				}
				err := cl.Resource(tc.config.GroupVersionResource).Namespace(ns).Delete(ctx, delete, deleteOptions)
				if err != nil {
					t.Fatalf("unexpected client delete error: %+v", err)
				}
			}

			for ns, update := range tc.updateObjects {
				wg.Add(1)
				newObj := update.(*unstructured.Unstructured)
				_, err := cl.Resource(tc.config.GroupVersionResource).Namespace(ns).Update(ctx, newObj, metav1.UpdateOptions{})
				if err != nil {
					t.Fatalf("unexpected client update error: %+v", err)
				}
			}

			// wait for all the events to occur, else timeut in 30 seconds
			if waitTimeout(&wg, 30*time.Second) {
				t.Fatalf("unexpected timeout")
			}
			res, expectCount, err := dynamiDg.Fetch()
			if err != nil && !tc.err {
				t.Errorf("expected no error but got: %v", err)
			}
			if err == nil && tc.err {
				t.Errorf("expected to get an error but didn't get one")
			}

			if tc.expected != nil {
				data, ok := res.(*api.DynamicData)
				if !ok {
					t.Errorf("expected result be *api.DynamicData but wasn't")
				}

				list := data.Items
				// sorting list of results by name
				sortGatheredResources(list)
				// sorting list of expected results by name
				sortGatheredResources(tc.expected)

				assert.Equal(t, tc.expected, list)
				assert.Len(t, list, expectCount, "unexpected number of resources returned")
			}
		})
	}
}

func TestDynamicGathererNativeResources_Fetch(t *testing.T) {
	// start a k8s client
	// init the datagatherer's informer with the client
	// add/delete resources watched by the data gatherer
	// check the expected result
	podGVR := schema.GroupVersionResource{Group: corev1.SchemeGroupVersion.Group, Version: corev1.SchemeGroupVersion.Version, Resource: "pods"}
	tests := map[string]struct {
		config            ConfigDynamic
		excludeAnnotsKeys []string
		excludeLabelKeys  []string
		addObjects        []runtime.Object
		deleteObjects     map[string]string
		updateObjects     map[string]runtime.Object
		expected          []*api.GatheredResource
		err               bool
	}{
		"only a Pod should be returned if GVR selects pods": {
			addObjects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				getObject("v1", "Service", "testservice", "testns", false),
				getObject("foobar/v1", "NotFoo", "notfoo", "testns", false),
				&corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod1", Namespace: "testns", UID: "uid-testpod1"}},
			},
			config: ConfigDynamic{
				IncludeNamespaces:    []string{"testns"},
				GroupVersionResource: podGVR,
			},
			expected: []*api.GatheredResource{
				{
					Resource: &corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod1", Namespace: "testns", UID: "uid-testpod1"}},
				},
			},
		},
		"delete a Pod resource from the testns, the cache should have a Pod with deletedAt set to now()": {
			addObjects: []runtime.Object{
				&corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testfoo", Namespace: "testns", UID: "uid-testfoo1"}},
				getObject("v1", "Service", "testservice", "testns", false),
				getObject("foobar/v1", "NotFoo", "notfoo", "testns", false),
			},
			deleteObjects: map[string]string{
				"testns": "testfoo",
			},
			config: ConfigDynamic{
				IncludeNamespaces:    []string{"testns"},
				GroupVersionResource: podGVR,
			},
			expected: []*api.GatheredResource{
				{
					Resource:  &corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testfoo", Namespace: "testns", UID: "uid-testfoo1"}},
					DeletedAt: api.Time{Time: clock.now()},
				},
			},
		},
		"Pods in different namespaces should be returned if no namespace field is set": {
			config: ConfigDynamic{
				IncludeNamespaces:    []string{""},
				GroupVersionResource: podGVR,
			},
			addObjects: []runtime.Object{
				&corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod1", Namespace: "testns", UID: "uid-testpod1"}},
				&corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod2", Namespace: "testns2", UID: "uid-testpod2"}},
			},
			expected: []*api.GatheredResource{
				{
					Resource: &corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod1", Namespace: "testns", UID: "uid-testpod1"}},
				},
				{
					Resource: &corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod2", Namespace: "testns2", UID: "uid-testpod2"}},
				},
			},
		},
		"Delete Pods in different namespaces should be returned if no namespace field is set": {
			config: ConfigDynamic{
				IncludeNamespaces:    []string{""},
				GroupVersionResource: podGVR,
			},
			addObjects: []runtime.Object{
				&corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod1", Namespace: "testns", UID: "uid-testpod1"}},
				&corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod2", Namespace: "testns2", UID: "uid-testpod2"}},
			},
			expected: []*api.GatheredResource{
				{
					Resource: &corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod1", Namespace: "testns", UID: "uid-testpod1"}},
				},
				{
					Resource: &corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod2", Namespace: "testns2", UID: "uid-testpod2"}},
				},
			},
		},
		"Delete all Pod resources, all the fetched resources should have a deletedAt field set to now()": {
			config: ConfigDynamic{
				IncludeNamespaces:    []string{""},
				GroupVersionResource: podGVR,
			},
			deleteObjects: map[string]string{
				"testns1": "testpod1",
				"testns2": "testpod2",
			},
			addObjects: []runtime.Object{
				&corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod1", Namespace: "testns1", UID: "uid-testpod1"}},
				&corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod2", Namespace: "testns2", UID: "uid-testpod2"}},
			},
			expected: []*api.GatheredResource{
				{
					Resource:  &corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod1", Namespace: "testns1", UID: "uid-testpod1"}},
					DeletedAt: api.Time{Time: clock.now()},
				},
				{
					Resource:  &corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod2", Namespace: "testns2", UID: "uid-testpod2"}},
					DeletedAt: api.Time{Time: clock.now()},
				},
			},
		},
		"Update all Pods resources, all the fetched resources should have been updated": {
			config: ConfigDynamic{
				IncludeNamespaces:    []string{""},
				GroupVersionResource: podGVR,
			},
			updateObjects: map[string]runtime.Object{
				"testns1": &corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod1", Namespace: "testns1", UID: "uid-testpod1", Labels: map[string]string{"foo": "newlabel"}}},
				"testns2": &corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod2", Namespace: "testns2", UID: "uid-testpod2", Labels: map[string]string{"foo": "newlabel"}}},
			},
			addObjects: []runtime.Object{
				&corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod1", Namespace: "testns1", UID: "uid-testpod1"}},
				&corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod2", Namespace: "testns2", UID: "uid-testpod2"}},
			},
			expected: []*api.GatheredResource{
				{
					Resource: &corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod1", Namespace: "testns1", UID: "uid-testpod1", Labels: map[string]string{"foo": "newlabel"}}},
				},
				{
					Resource: &corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}, ObjectMeta: metav1.ObjectMeta{Name: "testpod2", Namespace: "testns2", UID: "uid-testpod2", Labels: map[string]string{"foo": "newlabel"}}},
				},
			},
		},
		"only Pods in the specified namespace should be returned": {
			config: ConfigDynamic{
				IncludeNamespaces:    []string{"testns"},
				GroupVersionResource: podGVR,
			},
			addObjects: []runtime.Object{
				&corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testfoo1",
						Namespace: "testns",
						UID:       "uid-testfoo1",
					},
				},
				&corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testfoo1",
						Namespace: "nottestns",
						UID:       "uid-testfoo2",
					},
				},
			},
			expected: []*api.GatheredResource{
				{
					Resource: &corev1.Pod{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Pod",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "testfoo1",
							Namespace: "testns",
							UID:       "uid-testfoo1",
						},
					},
				},
			},
		},
		// Pod is the only native resource that we test out of lack of time
		// (would require a lot of changes to the testing func). Ideally we
		// should test all native resources such as Service, Deployment,
		// Ingress, Namespace, and so on.
		"excluded annotations are removed for typed resources gatherers such as pods": {
			config:            ConfigDynamic{GroupVersionResource: podGVR},
			excludeAnnotsKeys: []string{"secret"},
			excludeLabelKeys:  []string{"secret"},
			addObjects: []runtime.Object{
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p0", UID: "p0", Namespace: "n1", Annotations: map[string]string{"normal-annot": "bar"}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p1", UID: "p1", Namespace: "n1", Labels: map[string]string{"normal-label": "bar"}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p2", UID: "p2", Namespace: "n1", Annotations: map[string]string{"super-secret-annot": "bar"}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p3", UID: "p3", Namespace: "n1", Labels: map[string]string{"super-secret-label": "bar"}}},
			},
			expected: []*api.GatheredResource{
				{Resource: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p0", UID: "p0", Namespace: "n1", Annotations: map[string]string{"normal-annot": "bar"}}, TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}}},
				{Resource: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p1", UID: "p1", Namespace: "n1", Labels: map[string]string{"normal-label": "bar"}}, TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}}},
				{Resource: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p2", UID: "p2", Namespace: "n1", Annotations: map[string]string{}}, TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}}},
				{Resource: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p3", UID: "p3", Namespace: "n1", Labels: map[string]string{}}, TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}}},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var wg sync.WaitGroup
			ctx := t.Context()

			clientset := fakeclientset.NewSimpleClientset(tc.addObjects...)

			// init the datagatherer's informer with the client
			dg, err := tc.config.newDataGathererWithClient(ctx, nil, clientset)
			if err != nil {
				t.Fatalf("unexpected error: %+v", err)
			}

			// initializing test informer, this informer will capture all the events
			// that occur in the test case and only allow the dg.Fetch to be performed
			// after all the events have been triggered
			factory := informers.NewSharedInformerFactoryWithOptions(clientset,
				10*time.Minute,
				informers.WithNamespace(metav1.NamespaceAll),
				informers.WithTweakListOptions(func(options *metav1.ListOptions) {}))
			testInformer := factory.Core().V1().Pods().Informer()
			_, err = testInformer.AddEventHandler(k8scache.ResourceEventHandlerFuncs{
				DeleteFunc: func(obj any) {
					defer wg.Done()
					time.Sleep(100 * time.Millisecond)
				},
				UpdateFunc: func(oldObj, newObj any) {
					defer wg.Done()
					time.Sleep(100 * time.Millisecond)
				},
			})
			require.NoError(t, err)

			// start test Informer
			factory.Start(ctx.Done())
			k8scache.WaitForCacheSync(ctx.Done(), testInformer.HasSynced)
			dgd := dg.(*DataGathererDynamic)
			for _, key := range tc.excludeAnnotsKeys {
				dgd.ExcludeAnnotKeys = append(dgd.ExcludeAnnotKeys, regexp.MustCompile(key))
			}
			for _, key := range tc.excludeLabelKeys {
				dgd.ExcludeLabelKeys = append(dgd.ExcludeLabelKeys, regexp.MustCompile(key))
			}

			// start data gatherer informer
			dynamiDg := dg
			go func() {
				if err = dynamiDg.Run(ctx); err != nil {
					t.Errorf("unexpected client error: %+v", err)
				}
			}()
			err = dynamiDg.WaitForCacheSync(ctx)
			if err != nil {
				t.Fatalf("unexpected client error: %+v", err)
			}

			// deletes all the objects set to be deleted, to trigger
			// a delete event in the informers. Add 1 to wg
			for ns, delete := range tc.deleteObjects {
				wg.Add(1)
				deletePolicy := metav1.DeletePropagationForeground
				deleteOptions := metav1.DeleteOptions{
					PropagationPolicy: &deletePolicy,
				}
				err := clientset.CoreV1().Pods(ns).Delete(ctx, delete, deleteOptions)
				if err != nil {
					t.Fatalf("unexpected client delete error: %+v", err)
				}
			}

			for ns, update := range tc.updateObjects {
				wg.Add(1)
				newObj := update.(*corev1.Pod)
				_, err := clientset.CoreV1().Pods(ns).Update(ctx, newObj, metav1.UpdateOptions{})
				if err != nil {
					t.Fatalf("unexpected client update error: %+v", err)
				}
			}

			// wait for all the events to occur, else timeout in 30 seconds
			if waitTimeout(&wg, 5*time.Second) {
				t.Fatalf("unexpected timeout")
			}
			rawRes, count, err := dynamiDg.Fetch()
			if tc.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tc.expected != nil {
				res, ok := rawRes.(*api.DynamicData)
				require.Truef(t, ok, "expected result be an *api.DynamicData but wasn't")
				actual := res.Items

				// sorting list of results by name
				sortGatheredResources(actual)
				// sorting list of expected results by name
				sortGatheredResources(tc.expected)

				assert.Equal(t, tc.expected, actual)
				assert.Len(t, actual, count)
			}
		})
	}
}

// waitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false
	case <-time.After(timeout):
		return true
	}
}

func TestRemoveUnstructuredKeys(t *testing.T) {
	t.Run("remove single key", run_TestRemoveUnstructuredKeys(tc_RemoveUnstructuredKeys{
		givenPath:    []string{"metadata", "annotations"},
		givenExclude: []string{"^toexclude$"},
		givenObj: map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					"toexclude": "foo",
					"tokeep":    "bar",
				},
			},
		},
		expectObj: map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					"tokeep": "bar",
				},
			},
		},
	}))

	t.Run("remove keys using multiple regexes", run_TestRemoveUnstructuredKeys(tc_RemoveUnstructuredKeys{
		givenPath:    []string{"metadata", "annotations"},
		givenExclude: []string{"^toexclude1$", "^toexclude2$"},
		givenObj: map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					"toexclude1": "foo",
					"toexclude2": "bar",
				},
			},
		},
		expectObj: map[string]any{
			"metadata": map[string]any{"annotations": map[string]any{}},
		},
	}))

	t.Run("remove multiple keys with a single regex", run_TestRemoveUnstructuredKeys(tc_RemoveUnstructuredKeys{
		givenPath:    []string{"metadata", "annotations"},
		givenExclude: []string{"toexclude.*"},
		givenObj: map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					"toexclude1": "foo",
					"toexclude2": "bar",
					"tokeep":     "baz",
				},
			},
		},
		expectObj: map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					"tokeep": "baz",
				},
			},
		},
	}))

	t.Run("with no regex, the object is untouched", run_TestRemoveUnstructuredKeys(tc_RemoveUnstructuredKeys{
		givenPath:    []string{"metadata", "annotations"},
		givenExclude: []string{},
		givenObj: map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					"tokeep1": "foo",
				},
			},
		},
		expectObj: map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					"tokeep1": "foo",
				},
			},
		},
	}))

	// The "leaf" field is the field that is at the end of the path. For
	// example, "annotations" is the leaf field in metadata.annotations.
	t.Run("works when the leaf field is not found", run_TestRemoveUnstructuredKeys(tc_RemoveUnstructuredKeys{
		givenPath:    []string{"metadata", "annotations"},
		givenExclude: []string{},

		givenObj:  map[string]any{"metadata": map[string]any{}},
		expectObj: map[string]any{"metadata": map[string]any{}},
	}))

	t.Run("works when the leaf field is nil", run_TestRemoveUnstructuredKeys(tc_RemoveUnstructuredKeys{
		givenPath:    []string{"metadata", "annotations"},
		givenExclude: []string{},
		givenObj:     map[string]any{"metadata": map[string]any{"annotations": nil}},
		expectObj:    map[string]any{"metadata": map[string]any{"annotations": nil}},
	}))

	t.Run("works when leaf field is unexpectedly not nil and not a known map", run_TestRemoveUnstructuredKeys(tc_RemoveUnstructuredKeys{
		givenPath: []string{"metadata", "annotations"},
		givenObj:  map[string]any{"metadata": map[string]any{"annotations": 42}},
		expectObj: map[string]any{"metadata": map[string]any{"annotations": 42}},
	}))

	// The "intermediate" field is the field that is not at the end of the path.
	// For example, "metadata" is the intermediate field in
	// metadata.annotations.
	t.Run("works when the intermediate field doesn't exist", run_TestRemoveUnstructuredKeys(tc_RemoveUnstructuredKeys{
		givenPath: []string{"metadata", "annotations"},
		givenObj:  map[string]any{},
		expectObj: map[string]any{},
	}))

	t.Run("works when the intermediate field is nil", run_TestRemoveUnstructuredKeys(tc_RemoveUnstructuredKeys{
		givenPath: []string{"metadata", "annotations"},
		givenObj:  map[string]any{"metadata": nil},
		expectObj: map[string]any{"metadata": nil},
	}))

	t.Run("works when the intermediate field is unexpectedly not nil and not a map", run_TestRemoveUnstructuredKeys(tc_RemoveUnstructuredKeys{
		givenPath: []string{"metadata", "annotations"},
		givenObj:  map[string]any{"metadata": 42},
		expectObj: map[string]any{"metadata": 42},
	}))
}

type tc_RemoveUnstructuredKeys struct {
	givenExclude []string
	givenObj     map[string]any
	givenPath    []string
	expectObj    map[string]any
}

func run_TestRemoveUnstructuredKeys(tc tc_RemoveUnstructuredKeys) func(*testing.T) {
	return func(t *testing.T) {
		t.Helper()
		RemoveUnstructuredKeys(toRegexps(tc.givenExclude), &unstructured.Unstructured{Object: tc.givenObj}, tc.givenPath...)
		assert.Equal(t, tc.expectObj, tc.givenObj)
	}
}

func TestRemoveTypedKeys(t *testing.T) {
	t.Run("remove single key", run_TestRemoveTypedKeys(tc_TestRemoveTypedKeys{
		givenExclude: []string{"^toexclude$"},
		given:        map[string]string{"toexclude": "foo", "tokeep": "bar"},
		expected:     map[string]string{"tokeep": "bar"},
	}))

	t.Run("remove keys using multiple regexes", run_TestRemoveTypedKeys(tc_TestRemoveTypedKeys{
		givenExclude: []string{"^toexclude1$", "^toexclude2$"},
		given:        map[string]string{"toexclude1": "foo", "toexclude2": "bar", "tokeep": "baz"},
		expected:     map[string]string{"tokeep": "baz"},
	}))

	t.Run("remove multiple keys with a single regex", run_TestRemoveTypedKeys(tc_TestRemoveTypedKeys{
		givenExclude: []string{"^toexclude.*"},
		given:        map[string]string{"toexclude1": "foo", "toexclude2": "bar", "tokeep": "baz"},
		expected:     map[string]string{"tokeep": "baz"},
	}))

	t.Run("with no regex, the object is untouched", run_TestRemoveTypedKeys(tc_TestRemoveTypedKeys{
		givenExclude: []string{},
		given:        map[string]string{"tokeep1": "foo", "tokeep2": "bar"},
		expected:     map[string]string{"tokeep1": "foo", "tokeep2": "bar"},
	}))

	t.Run("works when the map is nil", run_TestRemoveTypedKeys(tc_TestRemoveTypedKeys{
		givenExclude: []string{"^toexclude$"},
		given:        nil,
		expected:     nil,
	}))
}

type tc_TestRemoveTypedKeys struct {
	givenExclude []string
	given        map[string]string
	expected     map[string]string
}

func run_TestRemoveTypedKeys(tc tc_TestRemoveTypedKeys) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()
		RemoveTypedKeys(toRegexps(tc.givenExclude), tc.given)
		assert.Equal(t, tc.expected, tc.given)
	}
}

func toRegexps(keys []string) []*regexp.Regexp {
	var regexps []*regexp.Regexp
	for _, key := range keys {
		regexps = append(regexps, regexp.MustCompile(key))
	}
	return regexps
}
