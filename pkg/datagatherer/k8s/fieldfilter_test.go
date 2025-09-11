package k8s

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jetstack/preflight/pkg/testutil"
)

func TestSelect(t *testing.T) {
	t.Run("secret", run_TestSelect(
		map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "example",
				"namespace": "example",
				"annotations": map[string]interface{}{
					"kubectl.kubernetes.io/last-applied-configuration": "secret",
				},
				"labels": map[string]interface{}{
					"foo": "bar",
				},
				"resourceVersion":   "fake-resource-version",
				"creationTimestamp": "2025-08-15T00:00:01Z",
				"deletionTimestamp": "2025-08-15T00:00:02Z",
				// Examples of fields which are dropped
				"deletionGracePeriodSeconds": 10,
				"finalizers":                 []string{"example.com/fake-finalizer"},
				"generation":                 11,
			},
			"type": "kubernetes.io/tls",
			"data": map[string]interface{}{
				"tls.crt":    "cert data",
				"tls.key":    "secret",
				"extra":      "should be removed",
				"conjur-map": "should be kept",
			},
		},
		SecretSelectedFields,
		map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "example",
				"namespace": "example",
				"annotations": map[string]interface{}{
					// The "last-applied-configuration" isn't ignored in
					// "Select". "Redact" removes it.
					"kubectl.kubernetes.io/last-applied-configuration": "secret",
				},
				"labels": map[string]interface{}{
					"foo": "bar",
				},
				"resourceVersion":   "fake-resource-version",
				"creationTimestamp": "2025-08-15T00:00:01Z",
				"deletionTimestamp": "2025-08-15T00:00:02Z",
			},
			"type": "kubernetes.io/tls",
			"data": map[string]interface{}{
				// The "tls.key" is ignored.
				"tls.crt":    "cert data",
				"conjur-map": "should be kept",
			},
		},
	))

	t.Run("route", run_TestSelect(
		map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Route",
			"metadata": map[string]interface{}{
				"name": "example",
				"annotations": map[string]interface{}{
					"kubectl.kubernetes.io/last-applied-configuration": "secret",
				},
				"labels": map[string]interface{}{
					"foo": "bar",
				},
				"resourceVersion":   "fake-resource-version",
				"creationTimestamp": "2025-08-15T00:00:01Z",
				"deletionTimestamp": "2025-08-15T00:00:02Z",
				// Examples of fields which are dropped
				"deletionGracePeriodSeconds": 10,
				"finalizers":                 []string{"example.com/fake-finalizer"},
				"generation":                 11,
			},
			"spec": map[string]interface{}{
				"host": "www.example.com",
				"to": map[string]interface{}{
					"kind": "Service",
					"name": "frontend",
				},
				"tls": map[string]interface{}{
					"termination":              "reencrypt",
					"key":                      "secret",
					"certificate":              "cert data",
					"caCertificate":            "caCert data",
					"destinationCACertificate": "destinationCaCert data",
				},
			},
		}, RouteSelectedFields,
		map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Route",
			"metadata": map[string]interface{}{
				"name": "example",
				"annotations": map[string]interface{}{
					// The "last-applied-configuration" isn't ignored in
					// "Select". "Redact" removes it.
					"kubectl.kubernetes.io/last-applied-configuration": "secret",
				},
				"resourceVersion":   "fake-resource-version",
				"creationTimestamp": "2025-08-15T00:00:01Z",
				"deletionTimestamp": "2025-08-15T00:00:02Z",
			},
			"spec": map[string]interface{}{
				"host": "www.example.com",
				"to": map[string]interface{}{
					"kind": "Service",
					"name": "frontend",
				},
				"tls": map[string]interface{}{
					"termination": "reencrypt",
					// The "key" field is ignored.
					"certificate":              "cert data",
					"caCertificate":            "caCert data",
					"destinationCACertificate": "destinationCaCert data",
				},
			},
		},
	))
}

func run_TestSelect(given map[string]interface{}, givenSelect []FieldPath, expect map[string]interface{}) func(*testing.T) {
	return func(t *testing.T) {
		t.Helper()
		givenPtr := unstructured.Unstructured{Object: given}
		err := Select(givenSelect, &givenPtr)
		require.NoError(t, err)

		assert.Equal(t, expect, givenPtr.Object)
	}
}

func TestSelectMissingSelectedField(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind": "Secret",
		},
	}

	fieldsToSelect := []FieldPath{
		{"kind"}, // required for unstructured unmarshal
		{"missing"},
	}

	err := Select(fieldsToSelect, resource)
	require.NoError(t, err)
	bytes, err := json.MarshalIndent(resource, "", "    ")
	require.NoError(t, err)

	expectedJSON := testutil.Undent(`
		{
		    "kind": "Secret"
		}`)
	assert.Equal(t, expectedJSON, string(bytes))
}

func TestRedactSecret(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "example",
				"namespace": "example",
				"annotations": map[string]interface{}{
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

	fieldsToRedact := []FieldPath{
		{"metadata", "managedFields"},
		{"metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration"},
		{"data", "tls.key"},
	}

	Redact(fieldsToRedact, resource)

	bytes, err := json.MarshalIndent(resource, "", "    ")
	require.NoError(t, err)
	expectedJSON := testutil.Undent(`
		{
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
		}`)
	assert.Equal(t, expectedJSON, string(bytes))
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

	fieldsToRedact := []FieldPath{
		{"metadata", "managedFields"},
	}

	Redact(fieldsToRedact, resource)

	bytes, err := json.MarshalIndent(resource, "", "    ")
	require.NoError(t, err)
	expectedJSON := testutil.Undent(`
		{
		    "apiVersion": "v1",
		    "kind": "Pod",
		    "metadata": {
		        "name": "example",
		        "namespace": "example"
		    },
		    "spec": {
		        "serviceAccountName": "example"
		    }
		}`)
	assert.Equal(t, expectedJSON, string(bytes))
}

func TestRedactMissingField(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind": "Secret",
		},
	}

	fieldsToRedact := []FieldPath{
		{"missing"},
	}

	Redact(fieldsToRedact, resource)
	bytes, err := json.MarshalIndent(resource, "", "    ")
	require.NoError(t, err)

	expectedJSON := testutil.Undent(`
		{
		    "kind": "Secret"
		}`)
	assert.Equal(t, expectedJSON, string(bytes))
}
