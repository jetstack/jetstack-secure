package k8s

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/utils/diff"
)

func getObject(version, kind, name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": version,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

func getSecret(name, namespace string, data map[string]interface{}) *unstructured.Unstructured {
	object := getObject("v1", "Secret", name, namespace)
	object.Object["data"] = data
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

func TestGenericGatherer_Fetch(t *testing.T) {
	emptyScheme := runtime.NewScheme()
	tests := map[string]struct {
		gvr       schema.GroupVersionResource
		namespace string
		objects   []runtime.Object
		expected  interface{}
		err       bool
	}{
		"an error should be returned if 'resource' is missing": {
			err: true,
			gvr: schema.GroupVersionResource{Group: "test", Version: "v1"},
		},
		"only a Foo should be returned if GVR selects foos": {
			gvr: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
			objects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns"),
				getObject("v1", "Service", "testservice", "testns"),
				getObject("foobar/v1", "NotFoo", "notfoo", "testns"),
			},
			expected: asUnstructuredList(
				getObject("foobar/v1", "Foo", "testfoo", "testns"),
			),
		},
		"only Foos in the specified namespace should be returned": {
			gvr:       schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
			namespace: "testns",
			objects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns"),
				getObject("foobar/v1", "Foo", "testfoo", "nottestns"),
			},
			expected: asUnstructuredList(
				getObject("foobar/v1", "Foo", "testfoo", "testns"),
			),
		},
		"Foos in different namespaces should be returned if no namespace field is set": {
			gvr: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
			objects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns"),
				getObject("foobar/v1", "Foo", "testfoo", "nottestns"),
			},
			expected: asUnstructuredList(
				getObject("foobar/v1", "Foo", "testfoo", "testns"),
				getObject("foobar/v1", "Foo", "testfoo", "nottestns"),
			),
		},
		"Secret resources should have data removed": {
			gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
			objects: []runtime.Object{
				getSecret("testsecret", "testns", map[string]interface{}{
					"secretKey": "secretValue",
				}),
				getSecret("anothertestsecret", "differentns", map[string]interface{}{
					"secretNumber": "12345",
				}),
			},
			expected: asUnstructuredList(
				getSecret("testsecret", "testns", map[string]interface{}{}),
				getSecret("anothertestsecret", "differentns", map[string]interface{}{}),
			),
		},
		// Note that we can't test use of fieldSelector to exclude namespaces
		// here as the as the fake client does not implement it.
		// See go/pkg/mod/k8s.io/client-go@v0.17.0/dynamic/fake/simple.go:291
		// TODO: Add a custom reactor to allow testing of fieldSelector.
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cl := fake.NewSimpleDynamicClient(emptyScheme, test.objects...)
			g := DataGatherer{
				cl:                   cl,
				groupVersionResource: test.gvr,
				namespace:            test.namespace,
			}

			res, err := g.Fetch()
			if err != nil && !test.err {
				t.Errorf("expected no error but got: %v", err)
			}
			if err == nil && test.err {
				t.Errorf("expected to get an error but didn't get one")
			}
			if !reflect.DeepEqual(res, test.expected) {
				t.Errorf("unexpected difference: %v", diff.ObjectDiff(res, test.expected))
			}
		})
	}
}

func TestUnmarshalConfig(t *testing.T) {
	textCfg := `
kubeconfig: "/home/someone/.kube/config"
resource-type:
  group: "g"
  version: "v"
  resource: "r"
exclude-namespaces:
- kube-system
- my-namespace
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

	cfg := Config{}
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
