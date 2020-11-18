package k8s

import (
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFieldSelector(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":                       "example",
				"namespace":                  "example",
				"last-applied-configuration": "secret",
			},
			"type": "kubernetes.io/tls",
			"data": map[string]interface{}{
				"tls.crt": "cert data",
				"tls.key": "secret",
			},
		},
	}

	fieldsToSelect := []string{
		"apiVersion",
		"kind",
		"metadata.name",
		"metadata.namespace",
		"type",
		"/data/tls.crt",
	}

	err := Select(fieldsToSelect, resource)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	bytes, err := json.MarshalIndent(resource, "", "    ")
	expectedJSON := `{
    "apiVersion": "v1",
    "data": {
        "tls.crt": "cert data"
    },
    "kind": "Secret",
    "metadata": {
        "name": "example",
        "namespace": "example"
    },
    "type": "kubernetes.io/tls"
}`
	if string(bytes) != expectedJSON {
		t.Fatalf("unexpected JSON: \ngot \n%s\nwant\n%s", string(bytes), expectedJSON)
	}
}
