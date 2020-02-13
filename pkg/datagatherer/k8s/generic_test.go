package k8s

import (
	"reflect"
	"testing"

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
				"name": name,
				"namespace": namespace,
			},
		},
	}
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
	tests := map[string]struct{
		gvr schema.GroupVersionResource
		namespace string
		objects []runtime.Object
		expected interface{}
		err bool
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
			gvr: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
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
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cl := fake.NewSimpleDynamicClient(emptyScheme, test.objects...)
			g := genericGatherer{
				cl: cl,
				groupVersionResource: test.gvr,
				namespace: test.namespace,
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
