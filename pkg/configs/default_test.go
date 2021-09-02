package configs

import (
	"testing"

	"github.com/jetstack/preflight/pkg/agent"
	"github.com/maxatome/go-testdeep/td"
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
# gather services for pod readiness probe rules
- kind: "k8s-dynamic"
  name: "k8s/services"
  config:
	resource-type:
	resource: services
	version: v1`,
			expectedAgentDataGatherers: []agent.DataGatherer{
				{
					Kind:     "k8s-dynamic",
					Name:     "k8s/pods",
					DataPath: "",
					Config:   nil,
				},
				{
					Kind:     "k8s-dynamic",
					Name:     "k8s/services",
					DataPath: "",
					Config:   nil,
				},
			},
		},
	}

	for _, input := range testCases {
		got, err := getDataGatherers(([]byte(input.inputYaml)))
		if err != nil {

		}

		td.Cmp(t, input.expectedAgentDataGatherers, got)
	}
}
