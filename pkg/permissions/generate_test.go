package permissions

import (
	"testing"

	"github.com/jetstack/preflight/pkg/agent"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGenerate(t *testing.T) {
	inputDatagatherers := []agent.DataGatherer{
		{
			Name: "k8s/pods",
			Kind: "k8s-dynamic",
			Config: &k8s.ConfigDynamic{
				GroupVersionResource: schema.GroupVersionResource{
					Version:  "v1",
					Resource: "pods",
				},
			},
		},
		{
			Name: "k8s/secrets",
			Kind: "k8s-dynamic",
			Config: &k8s.ConfigDynamic{
				GroupVersionResource: schema.GroupVersionResource{
					Version:  "v1",
					Resource: "secrets",
				},
			},
		},
	}

	expectedOutput := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: jetstack-secure-agent-pods-reader
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: jetstack-secure-agent-secrets-reader
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch"]
---`

	if output := Generate(inputDatagatherers); output != expectedOutput {
		t.Fatalf("unexpected output \n%s \n expected: \n%s", output, expectedOutput)
	}

}
