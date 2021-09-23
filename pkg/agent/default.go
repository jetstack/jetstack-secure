package agent

import (
	_ "embed"
	"io/ioutil"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

//go:embed default.yaml
var defaultYAML []byte

// GetDefaultDataGatherers
func GetDefaultDataGatherers() ([]DataGatherer, error) {
	return getDataGatherers(defaultYAML)
}

// LoadDataGatherers
func LoadDataGatherers(path string) ([]DataGatherer, error) {
	filename, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	defaultYAML, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return getDataGatherers([]byte(defaultYAML))
}

// getDataGatherers this will unmarshal the data gatherers
func getDataGatherers(yamlFile []byte) ([]DataGatherer, error) {
	var dataGatherer []DataGatherer
	err := yaml.Unmarshal(yamlFile, &dataGatherer)

	return dataGatherer, err
}
