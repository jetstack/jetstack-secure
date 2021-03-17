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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/dynamic/fake"
	k8scache "k8s.io/client-go/tools/cache"
	"k8s.io/utils/diff"
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

	// if we're creating a 'raw' secret as scraped that was applied by kubectl
	if withLastApplied {
		jsonData, _ := json.Marshal(data)
		metadata["annotations"] = map[string]interface{}{
			"kubectl.kubernetes.io/last-applied-configuration": string(jsonData),
		}
	}

	return object
}

func asUnstructuredList(items ...*unstructured.Unstructured) *unstructured.UnstructuredList {
	itemsNonPtr := make([]unstructured.Unstructured, len(items))
	for i, u := range items {
		itemsNonPtr[i] = *u
	}
	return &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"resourceVersion": "",
			},
		},
		Items: itemsNonPtr,
	}
}

func sortGatheredResources(list []*api.GatheredResource) {
	if len(list) > 1 {
		sort.SliceStable(list, func(i, j int) bool {
			itemA := list[i].Resource.(*unstructured.Unstructured).GetName()
			itemB := list[j].Resource.(*unstructured.Unstructured).GetName()
			return itemA < itemB
		})
	}
}

func TestNewDataGathererWithClient(t *testing.T) {
	ctx := context.Background()
	config := ConfigDynamic{
		IncludeNamespaces:    []string{"a"},
		GroupVersionResource: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
	}
	cl := fake.NewSimpleDynamicClient(runtime.NewScheme())
	dg, err := config.newDataGathererWithClient(ctx, cl)

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

	if !reflect.DeepEqual(dg, expected) {
		t.Errorf("unexpected difference: %v", diff.ObjectDiff(dg, expected))
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
					Resource:   getObject("foobar/v1", "Foo", "testfoo", "testns", false),
					Properties: &api.GatheredResourceMetadata{},
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
					Resource: getObject("foobar/v1", "Foo", "testfoo", "testns", false),
					Properties: &api.GatheredResourceMetadata{
						DeletedAt: &api.Time{Time: clock.now()},
					},
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
					Resource:   getObject("foobar/v1", "Foo", "testfoo", "testns", false),
					Properties: &api.GatheredResourceMetadata{},
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
					Resource:   getObject("foobar/v1", "Foo", "testfoo1", "testns1", false),
					Properties: &api.GatheredResourceMetadata{},
				},
				{
					Resource:   getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
					Properties: &api.GatheredResourceMetadata{},
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
					Resource:   getObject("foobar/v1", "Foo", "testfoo1", "testns1", false),
					Properties: &api.GatheredResourceMetadata{},
				},
				{
					Resource:   getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
					Properties: &api.GatheredResourceMetadata{},
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
					Resource: getObject("foobar/v1", "Foo", "testfoo1", "testns1", false),
					Properties: &api.GatheredResourceMetadata{
						DeletedAt: &api.Time{Time: clock.now()},
					},
				},
				{
					Resource: getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
					Properties: &api.GatheredResourceMetadata{
						DeletedAt: &api.Time{Time: clock.now()},
					},
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
					Resource:   getObject("foobar/v1", "Foo", "testfoo1", "testns1", false),
					Properties: &api.GatheredResourceMetadata{},
				},
				{
					Resource:   getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
					Properties: &api.GatheredResourceMetadata{},
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
					Resource:   getSecret("testsecret", "testns1", nil, false, false),
					Properties: &api.GatheredResourceMetadata{},
				},
				{
					Resource:   getSecret("anothertestsecret", "testns2", nil, false, false),
					Properties: &api.GatheredResourceMetadata{},
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
					Properties: &api.GatheredResourceMetadata{},
				},
				{
					// all other keys removed
					Resource:   getSecret("anothertestsecret", "testns2", nil, true, false),
					Properties: &api.GatheredResourceMetadata{},
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
			}
			cl := fake.NewSimpleDynamicClientWithCustomListKinds(emptyScheme, gvrToListKind, tc.addObjects...)
			// init the datagatherer's informer with the client
			dg, err := tc.config.newDataGathererWithClient(ctx, cl)
			if err != nil {
				t.Fatalf("unexpected error: %+v", err)
			}

			// initializing test informer, this informer will capture all the events
			// that occur in the test case and only allow the dg.Fetch to be perfomed
			// after all the events have been triggered
			factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(cl, 30*time.Second, metav1.NamespaceAll, nil)
			resourceInformer := factory.ForResource(tc.config.GroupVersionResource)
			testInformer := resourceInformer.Informer()
			testInformer.AddEventHandler(k8scache.ResourceEventHandlerFuncs{
				DeleteFunc: func(obj interface{}) {
					wg.Done()
				},
				UpdateFunc: func(old, new interface{}) {
					wg.Done()
				},
			})
			//start test Informer
			factory.Start(ctx.Done())
			k8scache.WaitForCacheSync(ctx.Done(), testInformer.HasSynced)

			// start data gatherer informer
			dynamiDg := dg.(*DataGathererDynamic)
			dynamiDg.Run(ctx.Done())
			err = dynamiDg.WaitForCacheSync(ctx.Done())
			if err != nil {
				t.Fatalf("unexpected client error: %+v", err)
			}

			// send resource events for the informer
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

			wg.Wait()
			res, err := dynamiDg.Fetch()
			if err != nil && !tc.err {
				t.Errorf("expected no error but got: %v", err)
			}
			if err == nil && tc.err {
				t.Errorf("expected to get an error but didn't get one")
			}

			if tc.expected != nil {
				list, ok := res.([]*api.GatheredResource)
				if !ok {
					t.Errorf("expected result be an *api.GatheredResource but wasn't")
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
