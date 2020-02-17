package agent

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// Config wraps the options for a run of the agent.
type Config struct {
	Schedule      string         `yaml:"schedule"`
	Token         string         `yaml:"token"`
	Endpoint      Endpoint       `yaml:"endpoint"`
	DataGatherers []dataGatherer `yaml:"data-gatherers"`
}

// Endpoint is the configuration of the server where to post the data.
type Endpoint struct {
	Protocol string `yaml:"protocol"`
	Host     string `yaml:"host"`
	Path     string `yaml:"path"`
}

type dataGatherer struct {
	Kind     string `yaml:"kind"`
	Name     string `yaml:"name"`
	DataPath string `yaml:"data-path,omitempty"`
	Config   []byte `yaml:"config"`
}

// Dump generates a YAML string of the Config object
func (c *Config) Dump() (string, error) {
	d, err := yaml.Marshal(&c)

	if err != nil {
		return "", errors.Wrap(err, "failed to generate YAML dump of config")
	}

	return string(d), nil
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

	if config.Endpoint.Protocol == "" {
		config.Endpoint.Protocol = "http"
	}

	if err = config.validate(); err != nil {
		return config, err
	}

	if !strings.HasPrefix(config.Endpoint.Path, "/") {
		config.Endpoint.Path = fmt.Sprintf("/%s", config.Endpoint.Path)
	}

	return config, nil
}
