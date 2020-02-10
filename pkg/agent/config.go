package agent

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"gopkg.in/yaml.v2"
)

// Config wraps the options for a run of the agent
type Config struct {
	Schedule      string         `yaml:"schedule"`
	Token         string         `yaml:"token"`
	Endpoint      endpoint       `yaml:"endpoint"`
	DataGatherers []dataGatherer `yaml:"data-gatherers"`
}

type endpoint struct {
	Host string `yaml:"host"`
	Path string `yaml:"path"`
}
type dataGatherer struct {
	Kind string            `yaml:"kind"`
	Name string            `yaml:"name"`
	Data map[string]string `yaml:"data"`
}

func (c *Config) validate() error {
	var result *multierror.Error

	if c.Token == "" {
		result = multierror.Append(result, fmt.Errorf("token is required"))
	}

	if c.Schedule == "" {
		result = multierror.Append(result, fmt.Errorf("schedule is required"))
	}

	if c.Endpoint.Host == "" {
		result = multierror.Append(result, fmt.Errorf("endpoint host is required"))
	}

	if c.Endpoint.Path == "" {
		result = multierror.Append(result, fmt.Errorf("endpoint path is required"))
	}

	for i, v := range c.DataGatherers {
		if v.Kind == "" {
			result = multierror.Append(result, fmt.Errorf("datagatherer %d/%d is missing a kind", i+1, len(c.DataGatherers)))
		}
		if v.Name == "" {
			result = multierror.Append(result, fmt.Errorf("datagatherer %d/%d is missing a name", i+1, len(c.DataGatherers)))
		}
	}

	return result.ErrorOrNil()
}

// ParseConfig reads config into a struct used to configure running agents
func ParseConfig(data []byte) (Config, error) {
	var config Config

	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}

	return config, config.validate()
}
