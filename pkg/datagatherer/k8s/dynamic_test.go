package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/d4l3k/messagediff"
	"github.com/jetstack/preflight/api"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/dynamic/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	k8scache "k8s.io/client-go/tools/cache"
)

func getObject(version, kind, name, namespace string, withManagedFields bool) *unstructured.Unstructured {
	metadata := map[string]interface{}{
		"name":      name,
		"namespace": namespace,
		"uid":       fmt.Sprintf("%s1", name),
	}

	if withManagedFields {
		// []metav1.FieldsV1{} can't be deep copied by fake client so using
		// string as example value
		metadata["managedFields"] = "set"
	}

	object := map[string]interface{}{
		"apiVersion": version,
		"kind":       kind,
		"metadata":   metadata,
	}

	return &unstructured.Unstructured{
		Object: object,
	}
}

func getSecret(name, namespace string, data map[string]interface{}, isTLS bool, withLastApplied bool) *unstructured.Unstructured {
	object := getObject("v1", "Secret", name, namespace, false)

	if data != nil {
		object.Object["data"] = data
	}

	object.Object["type"] = "Opaque"
	if isTLS {
		object.Object["type"] = "kubernetes.io/tls"
	}

	metadata, _ := object.Object["metadata"].(map[string]interface{})
	annotations := make(map[string]interface{})

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
	ctx := context.Background()
	config := ConfigDynamic{
		IncludeNamespaces:    []string{"a"},
		GroupVersionResource: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
	}
	cl := fake.NewSimpleDynamicClient(runtime.NewScheme())
	dg, err := config.newDataGathererWithClient(ctx, cl, nil)

	if err != nil {
		t.Errorf("expected no error but got: %v", err)
	}

	expected := &DataGathererDynamic{
		ctx:                  ctx,
		cl:                   cl,
		groupVersionResource: config.GroupVersionResource,
		// it's important that the namespaces are set as the IncludeNamespaces
		// during initialization
		namespaces: config.IncludeNamespaces,
	}

	gatherer := dg.(*DataGathererDynamic)
	// test gatherer's fields
	if !reflect.DeepEqual(gatherer.ctx, expected.ctx) {
		t.Errorf("expected %v, got %v", expected, dg)
	}
	if !reflect.DeepEqual(gatherer.cl, expected.cl) {
		t.Errorf("expected %v, got %v", expected, dg)
	}
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
	if gatherer.dynamicSharedInformer == nil {
		t.Errorf("unexpected dynamicSharedInformer value: %v", nil)
	}
	if gatherer.nativeSharedInformer != nil {
		t.Errorf("unexpected nativeSharedInformer value: %v. should be nil", gatherer.nativeSharedInformer)
	}
}

func TestNewDataGathererWithClientAndSharedIndexInformer(t *testing.T) {
	ctx := context.Background()
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
		ctx:                  ctx,
		k8sClientSet:         clientset,
		groupVersionResource: config.GroupVersionResource,
		// it's important that the namespaces are set as the IncludeNamespaces
		// during initialization
		namespaces: config.IncludeNamespaces,
	}

	gatherer := dg.(*DataGathererDynamic)
	// test gatherer's fields
	if !reflect.DeepEqual(gatherer.ctx, expected.ctx) {
		t.Errorf("expected %v, got %v", expected, dg)
	}
	if !reflect.DeepEqual(gatherer.k8sClientSet, expected.k8sClientSet) {
		t.Errorf("expected %v, got %v", expected, dg)
	}
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
	if gatherer.nativeSharedInformer == nil {
		t.Errorf("unexpected nativeSharedInformer value: %v", nil)
	}
	if gatherer.dynamicSharedInformer != nil {
		t.Errorf("unexpected dynamicSharedInformer value: %v. should be nil", gatherer.dynamicSharedInformer)
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
	}

	for _, test := range tests {
		err := test.Config.validate()
		if !strings.Contains(err.Error(), test.ExpectedError) {
			t.Errorf("expected %s, got %s", test.ExpectedError, err.Error())
		}
	}
}

func TestGenerateFieldSelector(t *testing.T) {
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
			ExpectedFieldSelector: "metadata.namespace!=kube-system,",
		},
		{
			ExcludeNamespaces: []string{
				"kube-system",
				"my-namespace",
			},
			ExpectedFieldSelector: "metadata.namespace!=my-namespace,metadata.namespace!=kube-system,",
		},
	}

	for _, test := range tests {
		fieldSelector := generateFieldSelector(test.ExcludeNamespaces)
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
	emptyScheme := runtime.NewScheme()
	tests := map[string]struct {
		config        ConfigDynamic
		addObjects    []runtime.Object
		deleteObjects map[string]string
		updateObjects map[string]runtime.Object
		expected      []*api.GatheredResource
		err           bool
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
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Namespace",
							"metadata": map[string]interface{}{
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
				getSecret("testsecret", "testns1", map[string]interface{}{
					"secretKey": "secretValue",
				}, false, true),
				getSecret("anothertestsecret", "testns2", map[string]interface{}{
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
				getSecret("testsecret", "testns1", map[string]interface{}{
					"tls.key": "secretValue",
					"tls.crt": "value",
					"ca.crt":  "value",
				}, true, true),
				getSecret("anothertestsecret", "testns2", map[string]interface{}{
					"example.key": "secretValue",
					"example.crt": "value",
				}, true, true),
			},
			expected: []*api.GatheredResource{
				{
					// only tls.crt and ca.cert remain
					Resource: getSecret("testsecret", "testns1", map[string]interface{}{
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
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var wg sync.WaitGroup
			ctx := context.Background()
			gvrToListKind := map[schema.GroupVersionResource]string{
				schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"}:      "UnstructuredList",
				schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}: "UnstructuredList",
				schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}:         "UnstructuredList",
				schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}:      "UnstructuredList",
			}
			cl := fake.NewSimpleDynamicClientWithCustomListKinds(emptyScheme, gvrToListKind, tc.addObjects...)
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
			testInformer.AddEventHandler(k8scache.ResourceEventHandlerFuncs{
				DeleteFunc: func(obj interface{}) {
					defer wg.Done()
					time.Sleep(100 * time.Millisecond)
				},
				UpdateFunc: func(old, new interface{}) {
					defer wg.Done()
					time.Sleep(100 * time.Millisecond)
				},
			})
			//start test Informer
			factory.Start(ctx.Done())
			k8scache.WaitForCacheSync(ctx.Done(), testInformer.HasSynced)

			// start data gatherer informer
			dynamiDg := dg
			err = dynamiDg.Run(ctx.Done())
			if err != nil {
				t.Fatalf("unexpected client error: %+v", err)
			}
			err = dynamiDg.WaitForCacheSync(ctx.Done())
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
				new := update.(*unstructured.Unstructured)
				_, err := cl.Resource(tc.config.GroupVersionResource).Namespace(ns).Update(ctx, new, metav1.UpdateOptions{})
				if err != nil {
					t.Fatalf("unexpected client update error: %+v", err)
				}
			}

			// wait for all the events to occur, else timeut in 30 seconds
			if waitTimeout(&wg, 30*time.Second) {
				t.Fatalf("unexpected timeout")
			}
			res, err := dynamiDg.Fetch()
			if err != nil && !tc.err {
				t.Errorf("expected no error but got: %v", err)
			}
			if err == nil && tc.err {
				t.Errorf("expected to get an error but didn't get one")
			}

			if tc.expected != nil {
				items, ok := res.(map[string]interface{})
				if !ok {
					t.Errorf("expected result be an map[string]interface{} but wasn't")
				}

				list, ok := items["items"].([]*api.GatheredResource)
				if !ok {
					t.Errorf("expected result be an []*api.GatheredResource but wasn't")
				}
				// sorting list of results by name
				sortGatheredResources(list)
				// sorting list of expected results by name
				sortGatheredResources(tc.expected)

				if diff, equal := messagediff.PrettyDiff(tc.expected, list); !equal {
					t.Errorf("\n%s", diff)
					expectedJSON, _ := json.MarshalIndent(tc.expected, "", "  ")
					gotJSON, _ := json.MarshalIndent(list, "", "  ")
					t.Fatalf("unexpected JSON: \ngot \n%s\nwant\n%s", string(gotJSON), expectedJSON)
				}
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
		config        ConfigDynamic
		addObjects    []runtime.Object
		deleteObjects map[string]string
		updateObjects map[string]runtime.Object
		expected      []*api.GatheredResource
		err           bool
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
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var wg sync.WaitGroup
			ctx := context.Background()

			var clientset kubernetes.Interface
			clientset = fakeclientset.NewSimpleClientset(tc.addObjects...)

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
			testInformer.AddEventHandler(k8scache.ResourceEventHandlerFuncs{
				DeleteFunc: func(obj interface{}) {
					defer wg.Done()
					time.Sleep(100 * time.Millisecond)
				},
				UpdateFunc: func(old, new interface{}) {
					defer wg.Done()
					time.Sleep(100 * time.Millisecond)
				},
			})

			//start test Informer
			factory.Start(ctx.Done())
			k8scache.WaitForCacheSync(ctx.Done(), testInformer.HasSynced)

			// start data gatherer informer
			dynamiDg := dg
			err = dynamiDg.Run(ctx.Done())
			if err != nil {
				t.Fatalf("unexpected client error: %+v", err)
			}
			err = dynamiDg.WaitForCacheSync(ctx.Done())
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
				new := update.(*corev1.Pod)
				_, err := clientset.CoreV1().Pods(ns).Update(ctx, new, metav1.UpdateOptions{})
				if err != nil {
					t.Fatalf("unexpected client update error: %+v", err)
				}
			}

			// wait for all the events to occur, else timeout in 30 seconds
			if waitTimeout(&wg, 5*time.Second) {
				t.Fatalf("unexpected timeout")
			}
			res, err := dynamiDg.Fetch()
			if err != nil && !tc.err {
				t.Errorf("expected no error but got: %v", err)
			}
			if err == nil && tc.err {
				t.Errorf("expected to get an error but didn't get one")
			}

			if tc.expected != nil {
				items, ok := res.(map[string]interface{})
				if !ok {
					t.Errorf("expected result be an map[string]interface{} but wasn't")
				}

				list, ok := items["items"].([]*api.GatheredResource)
				if !ok {
					t.Errorf("expected result be an []*api.GatheredResource but wasn't")
				}
				// sorting list of results by name
				sortGatheredResources(list)
				// sorting list of expected results by name
				sortGatheredResources(tc.expected)

				if diff, equal := messagediff.PrettyDiff(tc.expected, list); !equal {
					t.Errorf("\n%s", diff)
					expectedJSON, _ := json.MarshalIndent(tc.expected, "", "  ")
					gotJSON, _ := json.MarshalIndent(list, "", "  ")
					t.Fatalf("unexpected JSON: \ngot \n%s\nwant\n%s", string(gotJSON), expectedJSON)
				}
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
