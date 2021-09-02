package configs

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/jetstack/preflight/pkg/agent"
	"gopkg.in/yaml.v2"
)

func GetDefaultDataGatherers() ([]agent.DataGatherer, error) {
	// This will read the default.yaml and call the getDataGatherers()
	filename, err := filepath.Abs("./default.yaml")
	if err != nil {
		fmt.Print("fail to read the path")
	}

	defaultYAML, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Print("fail to read the file")
	}

	var dataGatherer []agent.DataGatherer
	err = yaml.Unmarshal(defaultYAML, &dataGatherer)

	return dataGatherer, err
}

func getDataGatherers(yamlFile []byte) ([]agent.DataGatherer, error) {
	// this will unmarshal the data gatherer

	// defaultYAML, err := ioutil.ReadFile(yamlFile)
	// if err != nil {
	// 	fmt.Print("fail to read the file")
	// }

	type ConfigAgentRBACManifests struct {
		ClusterRoles []rbac.ClusterRole
		// ClusterRoleBindings is a list of crbs for resources which have no include/exclude ns configured
		ClusterRoleBindings []rbac.ClusterRoleBinding
		// RoleBindings is a list of namespaced bindings to grant permissions when include/exclude ns set
		RoleBindings []rbac.RoleBinding
	}

	var configAgentRBACManifests ConfigAgentRBACManifests
	yaml.Unmarshal(yamlFile, configAgentRBACManifests)
}
