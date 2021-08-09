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

func TestGenerateAgentRBACManifests(t *testing.T) {
	testCases := []struct {
		description                string
		dataGatherers              []agent.DataGatherer
		expectedAgentRBACManifests AgentRBACManifests
	}{
		{
			description: "Generate ClusterRole and ClusterRoleBinding for simple pod dg use case",
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
			},
			expectedAgentRBACManifests: AgentRBACManifests{
				ClusterRoles: []rbac.ClusterRole{
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
				},
				ClusterRoleBindings: []rbac.ClusterRoleBinding{
					{
						TypeMeta: metav1.TypeMeta{
							Kind:       "ClusterRoleBinding",
							APIVersion: "rbac.authorization.k8s.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "jetstack-secure-agent-pods-reader",
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
							Name:     "jetstack-secure-agent-pods-reader",
							APIGroup: "rbac.authorization.k8s.io",
						},
					},
				},
			},
		},
		{
			description: "Generate RBAC config for simple pod dg use case where single namespace is set",
			dataGatherers: []agent.DataGatherer{
				{
					Name: "k8s/pods",
					Kind: "k8s-dynamic",
					Config: &k8s.ConfigDynamic{
						GroupVersionResource: schema.GroupVersionResource{
							Version:  "v1",
							Resource: "pods",
						},
						IncludeNamespaces: []string{"example", "foobar"},
					},
				},
			},
			expectedAgentRBACManifests: AgentRBACManifests{
				ClusterRoles: []rbac.ClusterRole{
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
				},
				RoleBindings: []rbac.RoleBinding{
					{
						TypeMeta: metav1.TypeMeta{
							Kind:       "RoleBinding",
							APIVersion: "rbac.authorization.k8s.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "jetstack-secure-agent-pods-reader",
							Namespace: "example",
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
							Name:     "jetstack-secure-agent-pods-reader",
							APIGroup: "rbac.authorization.k8s.io",
						},
					},
					{
						TypeMeta: metav1.TypeMeta{
							Kind:       "RoleBinding",
							APIVersion: "rbac.authorization.k8s.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "jetstack-secure-agent-pods-reader",
							Namespace: "foobar",
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
							Name:     "jetstack-secure-agent-pods-reader",
							APIGroup: "rbac.authorization.k8s.io",
						},
					},
				},
			},
		},
	}

	for _, input := range testCases {
		got := GenerateAgentRBACManifests(input.dataGatherers)

		td.Cmp(t, input.expectedAgentRBACManifests, got)
	}
}
