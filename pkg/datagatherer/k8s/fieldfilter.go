package k8s

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// SecretSelectedFields is the list of fields sent from Secret objects to the
// backend
var SecretSelectedFields = []FieldPath{
	{"kind"},
	{"apiVersion"},
	{"metadata", "annotations"},
	{"metadata", "labels"},
	{"metadata", "name"},
	{"metadata", "namespace"},
	{"metadata", "ownerReferences"},
	{"metadata", "selfLink"},
	{"metadata", "uid"},

	{"type"},
	{"data", "tls.crt"},
	{"data", "ca.crt"},
}

// RouteSelectedFields is the list of fields sent from OpenShift Route objects to the
// backend
var RouteSelectedFields = []FieldPath{
	{"kind"},
	{"apiVersion"},
	{"metadata", "annotations"},
	{"metadata", "name"},
	{"metadata", "namespace"},
	{"metadata", "ownerReferences"},
	{"metadata", "selfLink"},
	{"metadata", "uid"},

	{"spec", "host"},
	{"spec", "to", "kind"},
	{"spec", "to", "name"},
	{"spec", "to", "weight"},
	{"spec", "tls", "termination"},
	{"spec", "tls", "certificate"},
	{"spec", "tls", "caCertificate"},
	{"spec", "tls", "destinationCACertificate"},
	{"spec", "tls", "insecureEdgeTerminationPolicy"},
	{"spec", "wildcardPolicy"},
	{"status"},
}

// RedactFields are removed from all objects
var RedactFields = []FieldPath{
	{"metadata", "managedFields"},
	{"metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration"},
}

type FieldPath []string

// Select removes all but the supplied fields from the resource
func Select(fields []FieldPath, resource *unstructured.Unstructured) error {
	newResource := unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	for _, field := range fields {
		value, found, err := unstructured.NestedFieldNoCopy(resource.Object, field...)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		if err := unstructured.SetNestedField(newResource.Object, value, field...); err != nil {
			return err
		}
	}

	resource.Object = newResource.Object

	return nil
}

// Redact removes the supplied fields from the resource
func Redact(fields []FieldPath, resource *unstructured.Unstructured) error {
	for _, field := range fields {
		unstructured.RemoveNestedField(resource.Object, field...)
	}

	return nil
}
