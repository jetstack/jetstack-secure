package configs

import (
	"testing"

	"github.com/jetstack/preflight/pkg/agent"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"github.com/maxatome/go-testdeep/td"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestParseDatagatherers(t *testing.T) {
	testCases := []struct {
		description                string
		expectedAgentDataGatherers []agent.DataGatherer
		inputYaml                  string
	}{
		{
			description: "simple data gatherer unmarshal",
			inputYaml: `
- kind: "k8s-dynamic"
  name: "k8s/pods"
  config:
    resource-type:
      resource: pods
      version: v1
- kind: "k8s-dynamic"
  name: "k8s/services"
  config:
    resource-type:
      resource: services
      version: v1
`,
			expectedAgentDataGatherers: []agent.DataGatherer{
				{
					Kind:     "k8s-dynamic",
					Name:     "k8s/pods",
					DataPath: "",
					Config: &k8s.ConfigDynamic{
						GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
					},
				},
				{
					Kind:     "k8s-dynamic",
					Name:     "k8s/services",
					DataPath: "",
					Config: &k8s.ConfigDynamic{
						GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
					},
				},
			},
		},
	}

	for _, input := range testCases {
		got, err := getDataGatherers(([]byte(input.inputYaml)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		td.Cmp(t, input.expectedAgentDataGatherers, got)
	}
}
