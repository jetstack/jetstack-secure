package permissions

import (
	"fmt"
	"strings"

	"github.com/jetstack/preflight/pkg/agent"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
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

	return strings.TrimPrefix(accumulator, "\n")
}
