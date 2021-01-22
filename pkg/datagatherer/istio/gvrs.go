package isito

import "k8s.io/apimachinery/pkg/runtime/schema"

// List of resources used by Istio analysis.
var gvrs = []schema.GroupVersionResource{
	{
		Group:    "apiextensions.k8s.io",
		Version:  "v1beta1",
		Resource: "customresourcedefinitions",
	},
	{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	},
	{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	},
	{
		Group:    "",
		Version:  "v1",
		Resource: "endpoints",
	},
	{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	},
	{
		Group:    "",
		Version:  "v1",
		Resource: "nodes",
	},
	{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	},
	{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	},
	{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	},
	{
		Group:    "extensions",
		Version:  "v1beta1",
		Resource: "ingresses",
	},
	{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "destinationrules",
	},
	{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "envoyfilters",
	},
	{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "gateways",
	},
	{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "serviceentries",
	},
	{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "sidecars",
	},
	{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "virtualservices",
	},
	{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "eorkloadentries",
	},
	{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "workloadgroups",
	},
	{
		Group:    "networking.x-k8s.io",
		Version:  "v1alpha1",
		Resource: "backendpolicies",
	},
	{
		Group:    "networking.x-k8s.io",
		Version:  "v1alpha1",
		Resource: "gateways",
	},
	{
		Group:    "networking.x-k8s.io",
		Version:  "v1alpha1",
		Resource: "gatewayclasses",
	},
	{
		Group:    "networking.x-k8s.io",
		Version:  "v1alpha1",
		Resource: "httproutes",
	},
	{
		Group:    "networking.x-k8s.io",
		Version:  "v1alpha1",
		Resource: "tcproutes",
	},
	{
		Group:    "networking.x-k8s.io",
		Version:  "v1alpha1",
		Resource: "tlsroutes",
	},
	{
		Group:    "security.istio.io",
		Version:  "v1beta1",
		Resource: "authorizationpolicies",
	},
	{
		Group:    "security.istio.io",
		Version:  "v1beta1",
		Resource: "peerauthentications",
	},
	{
		Group:    "security.istio.io",
		Version:  "v1beta1",
		Resource: "requestauthentications",
	},
}
