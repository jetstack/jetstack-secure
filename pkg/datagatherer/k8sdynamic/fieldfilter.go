package k8sdynamic

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// SecretSelectedFields is the list of fields sent from Secret objects to the
// backend.
// The `data` is redacted, to prevent private keys or sensitive data being
// collected. Only the following none-sensitive keys are retained: tls.crt,
// ca.crt. These keys are assumed to always contain public TLS certificates.
// The `conjur-map` key is also retained, as it is used to map Secrets to
// Conjur variables, and is not considered sensitive.
// See https://docs.cyberark.com/conjur-open-source/latest/en/content/integrations/k8s-ocp/cjr-secrets-provider-lp.htm
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
	{"metadata", "creationTimestamp"},
	{"metadata", "deletionTimestamp"},
	{"metadata", "resourceVersion"},
	{"immutable"},
	{"type"},
	{"data", "tls.crt"},
	{"data", "ca.crt"},
	{"data", "conjur-map"},
}

// RouteSelectedFields is the list of fields sent from OpenShift Route objects to the
// backend.
// The Route resource is redacted because it may contain private keys for TLS.
//
// TODO(wallrj): Find out if the `.tls.key` field is the only one that may
// contain sensitive data and if so, that field could be redacted instead
// selecting everything else, for consistency with Ingress or any of the other
// resources that are collected. Or alternatively add an comment to explain why
// for Route, the set of fields is allow-listed while for Ingress, all fields
// are collected.
// https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/network_apis/route-route-openshift-io-v1#spec-tls-3
var RouteSelectedFields = []FieldPath{
	{"kind"},
	{"apiVersion"},
	{"metadata", "annotations"},
	{"metadata", "name"},
	{"metadata", "namespace"},
	{"metadata", "ownerReferences"},
	{"metadata", "selfLink"},
	{"metadata", "uid"},
	{"metadata", "creationTimestamp"},
	{"metadata", "deletionTimestamp"},
	{"metadata", "resourceVersion"},

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
		Object: map[string]any{},
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
func Redact(fields []FieldPath, resource *unstructured.Unstructured) {
	for _, field := range fields {
		unstructured.RemoveNestedField(resource.Object, field...)
	}
}
