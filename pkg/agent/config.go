package agent

import (
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"github.com/jetstack/preflight/pkg/datagatherer/local"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Config wraps the options for a run of the agent.
type Config struct {
	Schedule string        `yaml:"schedule"`
	Period   time.Duration `yaml:"period"`
	// Deprecated: Endpoint is being replaced with Server.
	Endpoint Endpoint `yaml:"endpoint"`
	// Server is the base url for the Preflight server.
	// It defaults to https://preflight.jetstack.io.
	Server string `yaml:"server"`
	// OrganizationID within Preflight that will receive the data.
	OrganizationID string `yaml:"organization_id"`
	// ClusterID is the cluster that the agent is scanning.
	ClusterID     string         `yaml:"cluster_id"`
	DataGatherers []DataGatherer `yaml:"data-gatherers"`
	// InputPath replaces DataGatherers with input data file
	InputPath string `yaml:"input-path"`
	// OutputPath replaces Server with output data file
	OutputPath  string             `yaml:"output-path"`
	VenafiCloud *VenafiCloudConfig `yaml:"venafi-cloud,omitempty"`
}

type Endpoint struct {
	Protocol string `yaml:"protocol"`
	Host     string `yaml:"host"`
	Path     string `yaml:"path"`
}

type DataGatherer struct {
	Kind     string `yaml:"kind"`
	Name     string `yaml:"name"`
	DataPath string `yaml:"data_path"`
	Config   datagatherer.Config
}

type VenafiCloudConfig struct {
	// UploaderID is the upload ID that will be used when
	// creating a cluster connection
	UploaderID string `yaml:"uploader_id,omitempty"`
	// UploadPath is the endpoint path for the upload API.
	UploadPath string `yaml:"upload_path,omitempty"`
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
func (dg *DataGatherer) UnmarshalYAML(unmarshal func(interface{}) error) error {
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
	case "k8s":
		cfg = &k8s.ConfigDynamic{}
	case "k8s-dynamic":
		cfg = &k8s.ConfigDynamic{}
	case "k8s-discovery":
		cfg = &k8s.ConfigDiscovery{}
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

func (c *Config) validate(isVenafi bool) error {
	var result *multierror.Error

	// configured for Venafi Cloud
	if c.VenafiCloud != nil {
		if c.VenafiCloud.UploadPath == "" {
			result = multierror.Append(result, fmt.Errorf("upload_path is required in Venafi Cloud mode"))
		}

		if _, err := url.Parse(c.VenafiCloud.UploadPath); err != nil {
			result = multierror.Append(result, fmt.Errorf("upload_path is not a valid URL"))
		}
	} else if !isVenafi {
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
func ParseConfig(data []byte, isVenafi bool) (Config, error) {
	var config Config

	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}

	if config.Server == "" && config.Endpoint.Host == "" && config.Endpoint.Path == "" {
		config.Server = "https://preflight.jetstack.io"
		if config.VenafiCloud != nil {
			config.Server = client.VenafiCloudProdURL
		}
	}

	if config.Endpoint.Protocol == "" && config.Server == "" {
		config.Endpoint.Protocol = "http"
	}

	err = config.validate(isVenafi)
	if err != nil {
		return config, err
	}

	return config, nil
}
