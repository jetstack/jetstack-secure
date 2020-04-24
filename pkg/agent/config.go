package agent

import (
	"fmt"
	"net/url"

	"github.com/hashicorp/go-multierror"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/datagatherer/aks"
	"github.com/jetstack/preflight/pkg/datagatherer/eks"
	"github.com/jetstack/preflight/pkg/datagatherer/gke"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"github.com/jetstack/preflight/pkg/datagatherer/local"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// Config wraps the options for a run of the agent.
type Config struct {
	Schedule string `yaml:"schedule"`
	// Token is the agent token if using basic authentication.
	// If not provided it will assume OAuth2 authentication.
	Token string `yaml:"token"`
	// Deprecated: Endpoint is being replaced with Server.
	Endpoint Endpoint `yaml:"endpoint"`
	// Server is the base url for the Preflight server.
	// It defaults to https://preflight.jetstack.io.
	Server string `yaml:"server"`
	// OrganizationID within Preflight that will receive the data.
	OrganizationID string `yaml:"organization_id"`
	// ClusterID is the cluster that the agent is scanning.
	ClusterID     string         `yaml:"cluster_id"`
	DataGatherers []dataGatherer `yaml:"data-gatherers"`
}

type Endpoint struct {
	Protocol string `yaml:"protocol"`
	Host     string `yaml:"host"`
	Path     string `yaml:"path"`
}

type dataGatherer struct {
	Kind     string
	Name     string
	DataPath string
	Config   datagatherer.Config
}

func reMarshal(rawConfig interface{}, config datagatherer.Config) error {
	bb, err := yaml.Marshal(rawConfig)
	if err != nil {
		return nil
	}

	err = yaml.Unmarshal(bb, config)
	if err != nil {
		return nil
	}

	return nil
}

// UnmarshalYAML unmarshals a dataGatherer resolving the type according to Kind.
func (dg *dataGatherer) UnmarshalYAML(unmarshal func(interface{}) error) error {
	aux := struct {
		Kind      string      `yaml:"kind"`
		Name      string      `yaml:"name"`
		DataPath  string      `yaml:"data-path,omitempty"`
		RawConfig interface{} `yaml:"config"`
	}{}
	err := unmarshal(&aux)
	if err != nil {
		return err
	}

	dg.Kind = aux.Kind
	dg.Name = aux.Name
	dg.DataPath = aux.DataPath

	var cfg datagatherer.Config

	switch dg.Kind {
	case "gke":
		cfg = &gke.Config{}
	case "eks":
		cfg = &eks.Config{}
	case "aks":
		cfg = &aks.Config{}
	case "k8s":
		cfg = &k8s.Config{}
	case "local":
		cfg = &local.Config{}
	// dummy dataGatherer is just used for testing
	case "dummy":
		cfg = &dummyConfig{}
	default:
		return fmt.Errorf("cannot parse data-gatherer configuration, kind %q is not supported", dg.Kind)
	}

	// we encode aux.RawConfig, which is just a map of reflect.Values, into yaml and decode it again to the right type.
	err = reMarshal(aux.RawConfig, cfg)
	if err != nil {
		return err
	}

	dg.Config = cfg

	return nil
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
		if c.OrganizationID == "" {
			result = multierror.Append(result, fmt.Errorf("organization_id is required"))
		}
		if c.ClusterID == "" {
			result = multierror.Append(result, fmt.Errorf("cluster_id is required"))
		}
	}

	if c.Server != "" {
		if url, err := url.Parse(c.Server); err != nil || url.Hostname() == "" {
			result = multierror.Append(result, fmt.Errorf("server is not a valid URL"))
		}
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

	if config.Server == "" && config.Endpoint.Host == "" && config.Endpoint.Path == "" {
		config.Server = "https://preflight.jetstack.io"
	}

	if config.Endpoint.Protocol == "" && config.Server == "" {
		config.Endpoint.Protocol = "http"
	}

	if err = config.validate(); err != nil {
		return config, err
	}

	return config, nil
}
