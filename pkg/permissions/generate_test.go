package permissions

import (
	"testing"

	"github.com/d4l3k/messagediff"
	"github.com/jetstack/preflight/pkg/agent"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGenerateRBAC(t *testing.T) {
	// Use these test cases to check if Generate function is correct
	testCases := []struct {
		// expectedClusterRoles is the collection of ClusterRole
		expectedClusterRoles []rbac.ClusterRole
		dataGatherers        []agent.DataGatherer
		description          string
	}{
		{
			description: "Generate RBAC struct for pods datagatherer",
			dataGatherers: []agent.DataGatherer{
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
				{
					Name: "k8s/awspcaissuer",
					Kind: "k8s-dynamic",
					Config: &k8s.ConfigDynamic{
						GroupVersionResource: schema.GroupVersionResource{
							Group:    "awspca.cert-manager.io",
							Version:  "v1",
							Resource: "awspcaissuers",
						},
					},
				},
			},
			expectedClusterRoles: []rbac.ClusterRole{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterRole",
						APIVersion: "rbac.authorization.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "jetstack-secure-agent-pods-reader",
					},
					Rules: []rbac.PolicyRule{
						{
							Verbs:     []string{"get", "list", "watch"},
							APIGroups: []string{""},
							Resources: []string{"pods"},
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterRole",
						APIVersion: "rbac.authorization.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "jetstack-secure-agent-secrets-reader",
					},
					Rules: []rbac.PolicyRule{
						{
							Verbs:     []string{"get", "list", "watch"},
							APIGroups: []string{""},
							Resources: []string{"secrets"},
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterRole",
						APIVersion: "rbac.authorization.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "jetstack-secure-agent-awspcaissuers-reader",
					},
					Rules: []rbac.PolicyRule{
						{
							Verbs:     []string{"get", "list", "watch"},
							APIGroups: []string{"awspca.cert-manager.io"},
							Resources: []string{"awspcaissuers"},
						},
					},
				},
			},
		},
		// Try adding more test cases
	}

	for _, input := range testCases {
		got := GenerateRoles(input.dataGatherers)
		if diff, equal := messagediff.PrettyDiff(input.expectedClusterRoles, got); !equal {
			t.Errorf("%s:\n%s", input.description, diff)
			t.Fatalf("unexpected difference in RBAC cluster role: \ngot \n%v\nwant\n%v", got, input.expectedClusterRoles)
		}
	}
}
