package k8s

import (
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSelect(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "example",
				"namespace": "example",
				"annotations": map[string]string{
					"kubectl.kubernetes.io/last-applied-configuration": "secret",
				},
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

func TestSelectMissingSelectedField(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind": "Secret",
		},
	}

	fieldsToSelect := []string{
		"kind", // required for unstructured unmarshal
		"missing",
		"/missing",
	}

	err := Select(fieldsToSelect, resource)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	bytes, err := json.MarshalIndent(resource, "", "    ")
	expectedJSON := `{
    "kind": "Secret"
}`
	if string(bytes) != expectedJSON {
		t.Fatalf("unexpected JSON: \ngot \n%s\nwant\n%s", string(bytes), expectedJSON)
	}
}

func TestRedactSecret(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "example",
				"namespace": "example",
				"annotations": map[string]string{
					"kubectl.kubernetes.io/last-applied-configuration": "secret",
				},
				"managedFields": nil,
			},
			"type": "kubernetes.io/tls",
			"data": map[string]interface{}{
				"tls.crt": "cert data",
				"tls.key": "secret",
			},
		},
	}

	fieldsToRedact := []string{
		"metadata.managedFields",
		"/metadata/annotations/kubectl.kubernetes.io~1last-applied-configuration",
		"/data/tls.key",
	}

	err := Redact(fieldsToRedact, resource)
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
        "annotations": {},
        "name": "example",
        "namespace": "example"
    },
    "type": "kubernetes.io/tls"
}`
	if string(bytes) != expectedJSON {
		t.Fatalf("unexpected JSON: \ngot \n%s\nwant\n%s", string(bytes), expectedJSON)
	}
}

func TestRedactPod(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":          "example",
				"namespace":     "example",
				"managedFields": []interface{}{},
			},
			"spec": map[string]interface{}{
				"serviceAccountName": "example",
			},
		},
	}

	fieldsToRedact := []string{
		"metadata.managedFields",
	}

	err := Redact(fieldsToRedact, resource)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	bytes, err := json.MarshalIndent(resource, "", "    ")
	expectedJSON := `{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "name": "example",
        "namespace": "example"
    },
    "spec": {
        "serviceAccountName": "example"
    }
}`
	if string(bytes) != expectedJSON {
		t.Fatalf("unexpected JSON: \ngot \n%s\nwant\n%s", string(bytes), expectedJSON)
	}
}

func TestRedactMissingField(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind": "Secret",
		},
	}

	fieldsToRedact := []string{
		"missing",
		"/missing",
	}

	err := Redact(fieldsToRedact, resource)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	bytes, err := json.MarshalIndent(resource, "", "    ")
	expectedJSON := `{
    "kind": "Secret"
}`
	if string(bytes) != expectedJSON {
		t.Fatalf("unexpected JSON: \ngot \n%s\nwant\n%s", string(bytes), expectedJSON)
	}
}
