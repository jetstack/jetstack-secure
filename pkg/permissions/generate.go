package permissions

import (
	"fmt"
	"strings"

	"github.com/jetstack/preflight/pkg/agent"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Generate(dataGatherers []agent.DataGatherer) string {
	var accumulator string = ""

	for _, g := range dataGatherers {
		if g.Kind != "k8s-dynamic" {
			continue
		}

		genericConfig := g.Config
		dyConfig := genericConfig.(*k8s.ConfigDynamic)

		metaName := fmt.Sprint(dyConfig.GroupVersionResource.Resource)

		accumulator = fmt.Sprintf(`%s
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: jetstack-secure-agent-%s-reader
rules:
- apiGroups: ["%s"]
  resources: ["%s"]
  verbs: ["get", "list", "watch"]
---`, accumulator, metaName, dyConfig.GroupVersionResource.Group, dyConfig.GroupVersionResource.Resource)
	}

	s := strings.TrimPrefix(accumulator, "\n")
	ss := strings.TrimSuffix(s, "---")
	return strings.TrimSuffix(ss, "\n")
}

func GenerateClusterRoles(dataGatherer []agent.DataGatherer) []rbac.ClusterRole {
	out := []rbac.ClusterRole{}

	for _, g := range dataGatherer {
		if g.Kind != "k8s-dynamic" {
			continue
		}

		genericConfig := g.Config
		dyConfig := genericConfig.(*k8s.ConfigDynamic)

		metaName := dyConfig.GroupVersionResource.Resource

		out = append(out, rbac.ClusterRole{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterRole",
				APIVersion: "rbac.authorization.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("jetstack-secure-agent-%s-reader", metaName),
			},
			Rules: []rbac.PolicyRule{
				{
					Verbs:     []string{"get", "list", "watch"},
					APIGroups: []string{dyConfig.GroupVersionResource.Group},
					Resources: []string{metaName},
				},
			},
		})

	}
	return out
}

func GenerateClusterRoleBindings(clusterRoles []rbac.ClusterRole) []rbac.ClusterRoleBinding {
	out := []rbac.ClusterRoleBinding{}
	for _, cr := range clusterRoles {
		out = append(out, rbac.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterRoleBinding",
				APIVersion: "rbac.authorization.k8s.io/v1",
			},

			ObjectMeta: metav1.ObjectMeta{
				Name: cr.ObjectMeta.Name,
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
				Name:     cr.ObjectMeta.Name,
				APIGroup: "rbac.authorization.k8s.io",
			},
		})

	}
	return out
}
