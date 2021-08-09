package permissions

import (
	"testing"

	"github.com/jetstack/preflight/pkg/agent"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"github.com/maxatome/go-testdeep/td"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGenerateRBAC(t *testing.T) {
	// Use these test cases to check if Generate function is correct
	testCases := []struct {
		// expectedClusterRoles is the collection of ClusterRole
		expectedClusterRoleBindings []rbac.ClusterRoleBinding
		dataGatherers               []agent.DataGatherer
		description                 string
	}{
		{
			description: "Generate RBAC struct for pods datagatherer",
			dataGatherers: []agent.DataGatherer{

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
			},
			expectedClusterRoleBindings: []rbac.ClusterRoleBinding{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterRoleBinding",
						APIVersion: "rbac.authorization.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "jetstack-secure-agent-ClusterRole-reader",
					},
					Subjects: []rbac.Subject{
						{
							Kind:      "ServiceAccount",
							Name:      "agent",
							Namespace: "jetstack-secure",
						},
					},
					RoleRef: rbac.RoleRef{
						Kind:     "ClusterRole",
						Name:     "jetstack-secure-agent-secrets-reader",
						APIGroup: "rbac.authorization.k8s.io",
					},
				},
			},
		},
		// Try adding more test cases
	}

	for _, input := range testCases {
		got := GenerateRoles(input.dataGatherers)
		toBeTest := GenerateBindings(got)

		td.Cmp(t, input.expectedClusterRoleBindings, toBeTest)
	}
}
