package k8sdynamic

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
		map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":      "example",
				"namespace": "example",
				"annotations": map[string]any{
					"kubectl.kubernetes.io/last-applied-configuration": "secret",
				},
				"labels": map[string]any{
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
			"data": map[string]any{
				"tls.crt":    "cert data",
				"tls.key":    "secret",
				"extra":      "should be removed",
				"conjur-map": "should be kept",
			},
		},
		SecretSelectedFields,
		map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":      "example",
				"namespace": "example",
				"annotations": map[string]any{
					// The "last-applied-configuration" isn't ignored in
					// "Select". "Redact" removes it.
					"kubectl.kubernetes.io/last-applied-configuration": "secret",
				},
				"labels": map[string]any{
					"foo": "bar",
				},
				"resourceVersion":   "fake-resource-version",
				"creationTimestamp": "2025-08-15T00:00:01Z",
				"deletionTimestamp": "2025-08-15T00:00:02Z",
			},
			"type": "kubernetes.io/tls",
			"data": map[string]any{
				// The "tls.key" is ignored.
				"tls.crt":    "cert data",
				"conjur-map": "should be kept",
			},
		},
	))

	// Confirm select function preserves immutability
	t.Run("secret-immutable", run_TestSelect(
		map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"immutable":  true,
			"metadata": map[string]any{
				"name":      "with-immutable",
				"namespace": "example",
			},
			"type": "Opaque",
		},
		SecretSelectedFields,
		map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"immutable":  true,
			"metadata": map[string]any{
				"name":      "with-immutable",
				"namespace": "example",
			},
			"type": "Opaque",
		},
	))

	t.Run("secret-immutable-false", run_TestSelect(
		map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"immutable":  false,
			"metadata": map[string]any{
				"name":      "with-immutable-false",
				"namespace": "example",
			},
			"type": "Opaque",
		},
		SecretSelectedFields,
		map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"immutable":  false,
			"metadata": map[string]any{
				"name":      "with-immutable-false",
				"namespace": "example",
			},
			"type": "Opaque",
		},
	))

	t.Run("secret-immutable-absent", run_TestSelect(
		map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":      "immutable-absent",
				"namespace": "example",
			},
			"type": "Opaque",
		},
		SecretSelectedFields,
		map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":      "immutable-absent",
				"namespace": "example",
			},
			"type": "Opaque",
		},
	))

	t.Run("route", run_TestSelect(
		map[string]any{
			"apiVersion": "v1",
			"kind":       "Route",
			"metadata": map[string]any{
				"name": "example",
				"annotations": map[string]any{
					"kubectl.kubernetes.io/last-applied-configuration": "secret",
				},
				"labels": map[string]any{
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
			"spec": map[string]any{
				"host": "www.example.com",
				"to": map[string]any{
					"kind": "Service",
					"name": "frontend",
				},
				"tls": map[string]any{
					"termination":              "reencrypt",
					"key":                      "secret",
					"certificate":              "cert data",
					"caCertificate":            "caCert data",
					"destinationCACertificate": "destinationCaCert data",
				},
			},
		}, RouteSelectedFields,
		map[string]any{
			"apiVersion": "v1",
			"kind":       "Route",
			"metadata": map[string]any{
				"name": "example",
				"annotations": map[string]any{
					// The "last-applied-configuration" isn't ignored in
					// "Select". "Redact" removes it.
					"kubectl.kubernetes.io/last-applied-configuration": "secret",
				},
				"resourceVersion":   "fake-resource-version",
				"creationTimestamp": "2025-08-15T00:00:01Z",
				"deletionTimestamp": "2025-08-15T00:00:02Z",
			},
			"spec": map[string]any{
				"host": "www.example.com",
				"to": map[string]any{
					"kind": "Service",
					"name": "frontend",
				},
				"tls": map[string]any{
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

func run_TestSelect(given map[string]any, givenSelect []FieldPath, expect map[string]any) func(*testing.T) {
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
		Object: map[string]any{
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
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":      "example",
				"namespace": "example",
				"annotations": map[string]any{
					"kubectl.kubernetes.io/last-applied-configuration": "secret",
				},
				"managedFields": nil,
			},
			"type": "kubernetes.io/tls",
			"data": map[string]any{
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
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":          "example",
				"namespace":     "example",
				"managedFields": []any{},
			},
			"spec": map[string]any{
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
		Object: map[string]any{
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
