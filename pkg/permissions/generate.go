package permissions

import (
	"fmt"
	"strings"

	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/jetstack/preflight/pkg/agent"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
)

// AgentRBACManifests is a wrapper around the various RBAC structs needed to grant the agent fine-grained permissions as per its dg configs
type AgentRBACManifests struct {
	// ClusterRoles is a list of roles for resources the agent will collect
	ClusterRoles []rbac.ClusterRole
	// ClusterRoleBindings is a list of crbs for resources which have no include/exclude ns configured
	ClusterRoleBindings []rbac.ClusterRoleBinding
	// RoleBindings is a list of namespaced bindings to grant permissions when include/exclude ns set
	RoleBindings []rbac.RoleBinding
}

const agentNamespace = "jetstack-secure"
const agentSubjectName = "agent"

func GenerateAgentRBACManifests(dataGatherers []agent.DataGatherer) AgentRBACManifests {
	// create a new AgentRBACManifest struct
	var AgentRBACManifests AgentRBACManifests

	for _, dg := range dataGatherers {
		if dg.Kind != "k8s-dynamic" {
			continue
		}

		dyConfig := dg.Config.(*k8s.ConfigDynamic)
		metadataName := fmt.Sprintf("%s-agent-%s-reader", agentNamespace, dyConfig.GroupVersionResource.Resource)

		AgentRBACManifests.ClusterRoles = append(AgentRBACManifests.ClusterRoles, rbac.ClusterRole{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterRole",
				APIVersion: "rbac.authorization.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: metadataName,
			},
			Rules: []rbac.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch"},
					APIGroups: []string{dyConfig.GroupVersionResource.Group},
					Resources: []string{dyConfig.GroupVersionResource.Resource},
				},
			},
		})

		// if dyConfig.IncludeNamespaces has more than 0 items in it
		//   then, for each namespace create a rbac.RoleBinding in that namespace
		if len(dyConfig.IncludeNamespaces) != 0 {
			for _, ns := range dyConfig.IncludeNamespaces {
				AgentRBACManifests.RoleBindings = append(AgentRBACManifests.RoleBindings, rbac.RoleBinding{
					TypeMeta: metav1.TypeMeta{
						Kind:       "RoleBinding",
						APIVersion: "rbac.authorization.k8s.io/v1",
					},

					ObjectMeta: metav1.ObjectMeta{
						Name:      metadataName,
						Namespace: ns,
					},

					Subjects: []rbac.Subject{
						{
							Kind:      "ServiceAccount",
							Name:      agentSubjectName,
							Namespace: agentNamespace,
						},
					},

					RoleRef: rbac.RoleRef{
						Kind:     "ClusterRole",
						Name:     metadataName,
						APIGroup: "rbac.authorization.k8s.io",
					},
				})
			}
		} else {
			// only do this if the dg does not have IncludeNamespaces set
			AgentRBACManifests.ClusterRoleBindings = append(AgentRBACManifests.ClusterRoleBindings, rbac.ClusterRoleBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterRoleBinding",
					APIVersion: "rbac.authorization.k8s.io/v1",
				},

				ObjectMeta: metav1.ObjectMeta{
					Name: metadataName,
				},

				Subjects: []rbac.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      agentSubjectName,
						Namespace: agentNamespace,
					},
				},

				RoleRef: rbac.RoleRef{
					Kind:     "ClusterRole",
					Name:     metadataName,
					APIGroup: "rbac.authorization.k8s.io",
				},
			})
		}

	}

	return AgentRBACManifests
}

func createClusterRoleString(clusterRoles []rbac.ClusterRole) string {
	var builder strings.Builder
	for _, cb := range clusterRoles {
		data, err := yaml.Marshal(cb)
		if err != nil {
			fmt.Print("Cluster Role fails to marshal")
		}

		builder.WriteString("\n")
		builder.Write(data)
		builder.WriteString("---")
	}

	return builder.String()
}
func createRoleBindingString(roleBindings []rbac.RoleBinding) string {
	var builder strings.Builder
	for _, cb := range roleBindings {
		data, err := yaml.Marshal(cb)
		if err != nil {
			fmt.Print("Role Binding fails to marshal")
		}

		builder.WriteString("\n")
		builder.Write(data)
		builder.WriteString("---")
	}

	return builder.String()
}
func createClusterRoleBindingString(clusterRoleBindings []rbac.ClusterRoleBinding) string {
	var builder strings.Builder
	for _, cb := range clusterRoleBindings {
		data, err := yaml.Marshal(cb)
		if err != nil {
			fmt.Print("Cluster Role Binding fails to marshal")
		}

		builder.WriteString("\n")
		builder.Write(data)
		builder.WriteString("---")
	}

	return builder.String()
}

func GenerateFullManifest(dataGatherers []agent.DataGatherer) string {
	agentRBACManifestsStruct := GenerateAgentRBACManifests(dataGatherers)
	agentCLR := createClusterRoleString(agentRBACManifestsStruct.ClusterRoles)
	agentCLRB := createClusterRoleBindingString(agentRBACManifestsStruct.ClusterRoleBindings)
	agentRB := createRoleBindingString(agentRBACManifestsStruct.RoleBindings)

	out := fmt.Sprintf(`%s%s%s`, agentCLR, agentCLRB, agentRB)
	out = strings.TrimPrefix(out, "\n")
	out = strings.TrimSpace(out)
	out = strings.ReplaceAll(out, "\n  creationTimestamp: null", "")

	return out

}
