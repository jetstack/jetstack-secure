package k8s

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/d4l3k/messagediff"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/utils/diff"
)

func getObject(version, kind, name, namespace string, withManagedFields bool) *unstructured.Unstructured {
	metadata := map[string]interface{}{
		"name":      name,
		"namespace": namespace,
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

func TestDynamicGatherer_Fetch(t *testing.T) {
	emptyScheme := runtime.NewScheme()
	tests := map[string]struct {
		gvr        schema.GroupVersionResource
		namespaces []string
		objects    []runtime.Object
		expected   interface{}
		err        bool
	}{
		"an error should be returned if 'resource' is missing": {
			err: true,
			gvr: schema.GroupVersionResource{Group: "test", Version: "v1"},
		},
		"only a Foo should be returned if GVR selects foos": {
			gvr: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
			objects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				getObject("v1", "Service", "testservice", "testns", false),
				getObject("foobar/v1", "NotFoo", "notfoo", "testns", false),
			},
			expected: asUnstructuredList(
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
			),
		},
		"only Foos in the specified namespace should be returned": {
			gvr:        schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
			namespaces: []string{"testns"},
			objects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				getObject("foobar/v1", "Foo", "testfoo", "nottestns", false),
			},
			expected: asUnstructuredList(
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
			),
		},
		"Foos in different namespaces should be returned if no namespace field is set": {
			gvr: schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
			objects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns1", false),
				getObject("foobar/v1", "Foo", "testfoo", "testns2", false),
			},
			expected: asUnstructuredList(
				getObject("foobar/v1", "Foo", "testfoo", "testns1", false),
				getObject("foobar/v1", "Foo", "testfoo", "testns2", false),
			),
		},
		"Secret resources should have data removed": {
			gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
			objects: []runtime.Object{
				getSecret("testsecret", "testns1", map[string]interface{}{
					"secretKey": "secretValue",
				}, false, true),
				getSecret("anothertestsecret", "testns2", map[string]interface{}{
					"secretNumber": "12345",
				}, false, true),
			},
			expected: asUnstructuredList(
				getSecret("testsecret", "testns1", nil, false, false),
				getSecret("anothertestsecret", "testns2", nil, false, false),
			),
		},
		"Secret of type kubernetes.io/tls should have crts and not keys": {
			gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
			objects: []runtime.Object{
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
			expected: asUnstructuredList(
				// only tls.crt and ca.cert remain
				getSecret("testsecret", "testns1", map[string]interface{}{
					"tls.crt": "value",
					"ca.crt":  "value",
				}, true, false),
				// all other keys removed
				getSecret("anothertestsecret", "testns2", nil, true, false),
			),
		},
		"Foos in different namespaces should be returned if they are in the namespace list for the gatherer": {
			gvr:        schema.GroupVersionResource{Group: "foobar", Version: "v1", Resource: "foos"},
			namespaces: []string{"testns", "testns2"},
			objects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
				getObject("foobar/v1", "Foo", "testfoo3", "nottestns", false),
			},
			expected: asUnstructuredList(
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				getObject("foobar/v1", "Foo", "testfoo2", "testns2", false),
			),
		},
		"Resources should have managed fields removed": {
			gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			objects: []runtime.Object{
				getObject("apps/v1", "Deployment", "foo1", "testns", false),
				getObject("apps/v1", "Deployment", "foo2", "testns", true),
			},
			expected: asUnstructuredList(
				getObject("apps/v1", "Deployment", "foo1", "testns", false),
				getObject("apps/v1", "Deployment", "foo2", "testns", false),
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
			g := DataGathererDynamic{
				cl:                   cl,
				groupVersionResource: test.gvr,
				// if empty, namespaces will default to []string{""} during
				// fetch to get all ns
				namespaces: test.namespaces,
			}

			res, err := g.Fetch()
			if err != nil && !test.err {
				t.Errorf("expected no error but got: %v", err)
			}
			if err == nil && test.err {
				t.Errorf("expected to get an error but didn't get one")
			}
			if diff, equal := messagediff.PrettyDiff(test.expected, res); !equal {
				t.Errorf("\n%s", diff)
				expectedJSON, _ := json.MarshalIndent(test.expected, "", "  ")
				gotJSON, _ := json.MarshalIndent(res, "", "  ")
				t.Fatalf("unexpected JSON: \ngot \n%s\nwant\n%s", string(gotJSON), expectedJSON)
			}
		})
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
