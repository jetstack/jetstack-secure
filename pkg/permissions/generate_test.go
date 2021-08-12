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

func TestGenerateAgentRBACManifestsString(t *testing.T) {
	testCases := []struct {
		description           string
		dataGatherers         []agent.DataGatherer
		expectedRBACManifests string
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
			expectedRBACManifests: `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
	name: jetstack-secure-agent-pods-reader
rules:
- apiGroups: [""]
	resources: ["pods"]
	verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: jetstack-secure-agent-pods-reader
roleRef:
  kind: ClusterRole
  name: jetstack-secure-agent-pods-reader
  apiGroup: rbac.authorization.k8s.io
subjects:
- kind: ServiceAccount
  name: agent
  namespace: jetstack-secure
`,
		},
		{
			description: "Generate ClusterRole and RoleBinding for simple pod dg with include namespace \"foobar\"",
			dataGatherers: []agent.DataGatherer{
				{
					Name: "k8s/pods",
					Kind: "k8s-dynamic",
					Config: &k8s.ConfigDynamic{
						IncludeNamespaces: []string{"foobar"},
						GroupVersionResource: schema.GroupVersionResource{
							Version:  "v1",
							Resource: "pods",
						},
					},
				},
			},
			expectedRBACManifests: `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
	name: jetstack-secure-agent-pods-reader
rules:
- apiGroups: [""]
	resources: ["pods"]
	verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: jetstack-secure-agent-pods-reader
  namespace: foobar
roleRef:
  kind: ClusterRole
  name: jetstack-secure-agent-pods-reader
  apiGroup: rbac.authorization.k8s.io
subjects:
- kind: ServiceAccount
  name: agent
  namespace: jetstack-secure
`,
		},
	}

	for _, input := range testCases {
		got := generateFullManifest(input.dataGatherers)

		td.Cmp(t, input.expectedRBACManifests, got)
	}
}

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
			description: "Generate RBAC config for simple pod dg use case where only two namespace are included",
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
