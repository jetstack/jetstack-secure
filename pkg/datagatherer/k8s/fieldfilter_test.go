package k8s

// import (
// 	"encoding/json"
// 	"testing"

// 	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
// )

// func TestSelect(t *testing.T) {
// 	// secret objects
// 	secretResource := &unstructured.Unstructured{
// 		Object: map[string]interface{}{
// 			"apiVersion": "v1",
// 			"kind":       "Secret",
// 			"metadata": map[string]interface{}{
// 				"name":      "example",
// 				"namespace": "example",
// 				"annotations": map[string]string{
// 					"kubectl.kubernetes.io/last-applied-configuration": "secret",
// 				},
// 			},
// 			"type": "kubernetes.io/tls",
// 			"data": map[string]interface{}{
// 				"tls.crt": "cert data",
// 				"tls.key": "secret",
// 			},
// 		},
// 	}

// 	secretFieldsToSelect := []string{
// 		"apiVersion",
// 		"kind",
// 		"metadata.name",
// 		"metadata.namespace",
// 		"type",
// 		"/data/tls.crt",
// 	}

// 	secretExpectedJSON := `{
//     "apiVersion": "v1",
//     "data": {
//         "tls.crt": "cert data"
//     },
//     "kind": "Secret",
//     "metadata": {
//         "name": "example",
//         "namespace": "example"
//     },
//     "type": "kubernetes.io/tls"
// }`
// 	// route objects
// 	routeResource := &unstructured.Unstructured{
// 		Object: map[string]interface{}{
// 			"apiVersion": "v1",
// 			"kind":       "Route",
// 			"metadata": map[string]interface{}{
// 				"name": "example",
// 				"annotations": map[string]string{
// 					"kubectl.kubernetes.io/last-applied-configuration": "secret",
// 				},
// 			},
// 			"spec": map[string]interface{}{
// 				"host": "www.example.com",
// 				"to": map[string]string{
// 					"kind": "Service",
// 					"name": "frontend",
// 				},
// 				"tls": map[string]interface{}{
// 					"termination":              "reencrypt",
// 					"key":                      "secret",
// 					"certificate":              "cert data",
// 					"caCertificate":            "caCert data",
// 					"destinationCACertificate": "destinationCaCert data",
// 				},
// 			},
// 		},
// 	}

// 	routeFieldsToSelect := []string{
// 		"apiVersion",
// 		"kind",
// 		"metadata.name",
// 		"spec.host",
// 		"spec.to.kind",
// 		"spec.to.name",
// 		"spec.tls.termination",
// 		"spec.tls.certificate",
// 		"spec.tls.caCertificate",
// 		"spec.tls.destinationCACertificate",
// 	}

// 	routeExpectedJSON := `{
//     "apiVersion": "v1",
//     "kind": "Route",
//     "metadata": {
//         "name": "example"
//     },
//     "spec": {
//         "host": "www.example.com",
//         "tls": {
//             "caCertificate": "caCert data",
//             "certificate": "cert data",
//             "destinationCACertificate": "destinationCaCert data",
//             "termination": "reencrypt"
//         },
//         "to": {
//             "kind": "Service",
//             "name": "frontend"
//         }
//     }
// }`

// 	tests := map[string]struct {
// 		resource       *unstructured.Unstructured
// 		fieldsToSelect []string
// 		expectedJSON   string
// 	}{
// 		"secret": {secretResource, secretFieldsToSelect, secretExpectedJSON},
// 		"route":  {routeResource, routeFieldsToSelect, routeExpectedJSON},
// 	}

// 	for name, test := range tests {
// 		err := Select(test.fieldsToSelect, test.resource)
// 		if err != nil {
// 			t.Fatalf("unexpected error: %s", err)
// 		}

// 		bytes, err := json.MarshalIndent(test.resource, "", "    ")
// 		if err != nil {
// 			t.Fatalf("unexpected error: %s", err)
// 		}

// 		t.Run(name, func(t *testing.T) {
// 			if string(bytes) != test.expectedJSON {
// 				t.Fatalf("unexpected JSON: \ngot \n%s\nwant\n%s", string(bytes), test.expectedJSON)
// 			}
// 		})
// 	}
// }

// func TestSelectMissingSelectedField(t *testing.T) {
// 	resource := &unstructured.Unstructured{
// 		Object: map[string]interface{}{
// 			"kind": "Secret",
// 		},
// 	}

// 	fieldsToSelect := []string{
// 		"kind", // required for unstructured unmarshal
// 		"missing",
// 		"/missing",
// 	}

// 	err := Select(fieldsToSelect, resource)
// 	if err != nil {
// 		t.Fatalf("unexpected error: %s", err)
// 	}

// 	bytes, err := json.MarshalIndent(resource, "", "    ")
// 	if err != nil {
// 		t.Fatalf("unexpected error: %s", err)
// 	}

// 	expectedJSON := `{
//     "kind": "Secret"
// }`
// 	if string(bytes) != expectedJSON {
// 		t.Fatalf("unexpected JSON: \ngot \n%s\nwant\n%s", string(bytes), expectedJSON)
// 	}
// }

// func TestRedactSecret(t *testing.T) {
// 	resource := &unstructured.Unstructured{
// 		Object: map[string]interface{}{
// 			"apiVersion": "v1",
// 			"kind":       "Secret",
// 			"metadata": map[string]interface{}{
// 				"name":      "example",
// 				"namespace": "example",
// 				"annotations": map[string]string{
// 					"kubectl.kubernetes.io/last-applied-configuration": "secret",
// 				},
// 				"managedFields": nil,
// 			},
// 			"type": "kubernetes.io/tls",
// 			"data": map[string]interface{}{
// 				"tls.crt": "cert data",
// 				"tls.key": "secret",
// 			},
// 		},
// 	}

// 	fieldsToRedact := []string{
// 		"metadata.managedFields",
// 		"/metadata/annotations/kubectl.kubernetes.io~1last-applied-configuration",
// 		"/data/tls.key",
// 	}

// 	err := Redact(fieldsToRedact, resource)
// 	if err != nil {
// 		t.Fatalf("unexpected error: %s", err)
// 	}

// 	bytes, err := json.MarshalIndent(resource, "", "    ")
// 	if err != nil {
// 		t.Fatalf("unexpected error: %s", err)
// 	}
// 	expectedJSON := `{
//     "apiVersion": "v1",
//     "data": {
//         "tls.crt": "cert data"
//     },
//     "kind": "Secret",
//     "metadata": {
//         "annotations": {},
//         "name": "example",
//         "namespace": "example"
//     },
//     "type": "kubernetes.io/tls"
// }`
// 	if string(bytes) != expectedJSON {
// 		t.Fatalf("unexpected JSON: \ngot \n%s\nwant\n%s", string(bytes), expectedJSON)
// 	}
// }

// func TestRedactPod(t *testing.T) {
// 	resource := &unstructured.Unstructured{
// 		Object: map[string]interface{}{
// 			"apiVersion": "v1",
// 			"kind":       "Pod",
// 			"metadata": map[string]interface{}{
// 				"name":          "example",
// 				"namespace":     "example",
// 				"managedFields": []interface{}{},
// 			},
// 			"spec": map[string]interface{}{
// 				"serviceAccountName": "example",
// 			},
// 		},
// 	}

// 	fieldsToRedact := []string{
// 		"metadata.managedFields",
// 	}

// 	err := Redact(fieldsToRedact, resource)
// 	if err != nil {
// 		t.Fatalf("unexpected error: %s", err)
// 	}

// 	bytes, err := json.MarshalIndent(resource, "", "    ")
// 	if err != nil {
// 		t.Fatalf("unexpected error: %s", err)
// 	}
// 	expectedJSON := `{
//     "apiVersion": "v1",
//     "kind": "Pod",
//     "metadata": {
//         "name": "example",
//         "namespace": "example"
//     },
//     "spec": {
//         "serviceAccountName": "example"
//     }
// }`
// 	if string(bytes) != expectedJSON {
// 		t.Fatalf("unexpected JSON: \ngot \n%s\nwant\n%s", string(bytes), expectedJSON)
// 	}
// }

// func TestRedactMissingField(t *testing.T) {
// 	resource := &unstructured.Unstructured{
// 		Object: map[string]interface{}{
// 			"kind": "Secret",
// 		},
// 	}

// 	fieldsToRedact := []string{
// 		"missing",
// 		"/missing",
// 	}

// 	err := Redact(fieldsToRedact, resource)
// 	if err != nil {
// 		t.Fatalf("unexpected error: %s", err)
// 	}

// 	bytes, err := json.MarshalIndent(resource, "", "    ")
// 	if err != nil {
// 		t.Fatalf("unexpected error: %s", err)
// 	}

// 	expectedJSON := `{
//     "kind": "Secret"
// }`
// 	if string(bytes) != expectedJSON {
// 		t.Fatalf("unexpected JSON: \ngot \n%s\nwant\n%s", string(bytes), expectedJSON)
// 	}
// }
