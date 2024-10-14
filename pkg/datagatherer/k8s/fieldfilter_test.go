package k8s

import (
	"encoding/json"
	"testing"

	"github.com/jetstack/preflight/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterSecret(t *testing.T) {
	t.Run("full object", func(t *testing.T) {
		given := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "example",
				"namespace": "example",
				"annotations": map[string]interface{}{
					"kubectl.kubernetes.io/last-applied-configuration": "secret",
				},
				"labels": map[string]string{
					"foo": "bar",
				},
			},
			"type": "kubernetes.io/tls",
			"data": map[string]interface{}{
				"tls.crt": "cert data",
				"tls.key": "secret",
			},
		}
		expect := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "example",
				"namespace": "example",
				"annotations": map[string]string{
					// The "last-applied-configuration" isn't ignored in
					// "Select". "Redact" removes it.
					"kubectl.kubernetes.io/last-applied-configuration": "secret",
				},
				"labels": map[string]interface{}{
					"foo": "bar",
				},
			},
			"type": "kubernetes.io/tls",
			"data": map[string]interface{}{
				// The "tls.key" is ignored.
				"tls.crt": "cert data",
			},
		}

		got := FilterSecret(given)
		assert.Equal(t, expect, got)
	})

	t.Run("works even when some fields are missing", func(t *testing.T) {
		given := map[string]interface{}{
			"kind": "Secret",
		}
		expect := map[string]interface{}{
			"kind": "Secret",
		}
		got := FilterSecret(given)
		assert.Equal(t, expect, got)
	})
}

func TestFilterRoute(t *testing.T) {
	given := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Route",
		"metadata": map[string]interface{}{
			"name": "example",
			"annotations": map[string]string{
				"kubectl.kubernetes.io/last-applied-configuration": "secret",
			},
			"labels": map[string]string{
				"foo": "bar",
			},
		},
		"spec": map[string]interface{}{
			"host": "www.example.com",
			"to": map[string]string{
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
	}
	expect := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Route",
		"metadata": map[string]interface{}{
			"name": "example",
			"annotations": map[string]interface{}{
				// The "last-applied-configuration" isn't ignored in
				// "Select". "Redact" removes it.
				"kubectl.kubernetes.io/last-applied-configuration": "secret",
			},
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
	}

	got := FilterRoute(given)
	assert.Equal(t, expect, got)

}

func Test_DropNoisyFieldsObject(t *testing.T) {
	t.Run("secret", func(t *testing.T) {
		given := map[string]interface{}{
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
		}
		var expect map[string]interface{}
		err := json.Unmarshal([]byte(testutil.Undent(`
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
			}
		`)), &expect)
		require.NoError(t, err)

		DropNoisyFieldsObject(given)
		assert.Equal(t, expect, given)
	})

	t.Run("pod", func(t *testing.T) {
		given := map[string]interface{}{
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
		}
		var expect map[string]interface{}
		err := json.Unmarshal([]byte(testutil.Undent(`
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
			}
		`)), &expect)
		require.NoError(t, err)

		DropNoisyFieldsObject(given)
		assert.Equal(t, expect, given)
	})

	t.Run("still works when fields are missing", func(t *testing.T) {
		given := map[string]interface{}{
			"kind": "Secret",
		}
		expect := map[string]interface{}{
			"kind": "Secret",
		}
		DropNoisyFieldsObject(given)
		assert.Equal(t, expect, given)
	})
}
