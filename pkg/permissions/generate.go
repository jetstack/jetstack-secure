package permissions

import (
	"fmt"
	"strings"

	"github.com/jetstack/preflight/pkg/agent"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

//*******************************
func AgentCLR(clusterRoles []rbac.ClusterRole) string {
	var got []string
	for _, clr := range clusterRoles {
		var apiGroups, resources, verbs string
		for _, rule := range clr.Rules {
			apiGroups = strings.Join(rule.APIGroups, "\", \"")
			resources = strings.Join(rule.Resources, "\", \"")
			verbs = strings.Join(rule.Verbs, "\", \"")
		}

		got = append(got, fmt.Sprintf(
			`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
	name: jetstack-secure-agent-pods-reader
rules:
- apiGroups: ["%s"]
	resources: ["%s"]
	verbs: ["%s"]`, apiGroups, resources, verbs))
	}
	out := strings.Join(got, "\n")
	return out
}

//*******************************
func AgentCLRB(clusterRoleBindings []rbac.ClusterRoleBinding) string {
	var got []string
	for _, clrb := range clusterRoleBindings {
		got = append(got, fmt.Sprintf(
			`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %s
roleRef:
  kind: ClusterRole
  name: %s
  apiGroup: %s
subjects:
- kind: %s
  name: %s
  namespace: %s`, clrb.ObjectMeta.Name, clrb.RoleRef.Name, clrb.RoleRef.APIGroup, clrb.Subjects[0].Kind, clrb.Subjects[0].Name, clrb.Subjects[0].Namespace))
	}
	out := strings.Join(got, "\n")
	return out
}

//*******************************
func AgentRB(RoleBindings []rbac.RoleBinding) string {
	var got []string
	for _, rb := range RoleBindings {
		got = append(got, fmt.Sprintf(
			`---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: %s
  namespace: %s
roleRef:
  kind: ClusterRole
  name: %s
  apiGroup: %s
subjects:
- kind: %s
  name: %s
  namespace: %s`, rb.ObjectMeta.Name, rb.ObjectMeta.Namespace, rb.RoleRef.Name, rb.RoleRef.APIGroup, rb.Subjects[0].Kind, rb.Subjects[0].Name, rb.Subjects[0].Namespace))
	}
	out := strings.Join(got, "\n")
	return out
}

//*******************************

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

func GenerateFullManifest(dataGatherers []agent.DataGatherer) string {
	agentRBACManifestsStruct := GenerateAgentRBACManifests(dataGatherers)
	agentCLR := AgentCLR(agentRBACManifestsStruct.ClusterRoles)
	agentCLRB := AgentCLRB(agentRBACManifestsStruct.ClusterRoleBindings)
	agentRB := AgentRB(agentRBACManifestsStruct.RoleBindings)

	out := fmt.Sprintf(
		`%s
%s%s`, agentCLR, agentCLRB, agentRB)

	return out
}
