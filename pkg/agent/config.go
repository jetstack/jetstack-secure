package agent

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"github.com/jetstack/preflight/pkg/datagatherer/local"
	"github.com/jetstack/preflight/pkg/kubeconfig"
	"github.com/jetstack/preflight/pkg/version"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
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
	ClusterID          string         `yaml:"cluster_id"`
	ClusterDescription string         `yaml:"cluster_description"`
	DataGatherers      []DataGatherer `yaml:"data-gatherers"`
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

type AgentCmdFlags struct {
	// ConfigFilePath (--config-file, -c) is the path to the agent configuration
	// YAML file.
	ConfigFilePath string

	// Period (--period, -p) is the time waited between scans.
	Period time.Duration

	// OneShot (--one-shot) flag causes agent to run once.
	OneShot bool

	// VenafiCloudMode (--venafi-cloud) determines which format to load for
	// config and credential type.
	VenafiCloudMode bool

	// ClientID (--client-id) is the clientID in case of Venafi Cloud mode.
	ClientID string

	// PrivateKeyPath (--private-key-path) is the path for the service account
	// private key in case of Venafi Cloud mode.
	PrivateKeyPath string

	// CredentialsPath (--credentials-file, -k) is the path to the credentials )
	// is where the agent will try to loads the credentials (Experimental).
	CredentialsPath string

	// OutputPath (--output-path) is where the agent will write data to instead
	// of uploading to server.
	OutputPath string

	// InputPath (--input-path) is where the agent will read data from instead
	// of gathering data from clusters.
	InputPath string

	// BackoffMaxTime (--backoff-max-time) is the maximum time for which data
	// gatherers will retry after a failure.
	BackoffMaxTime time.Duration

	// StrictMode (--strict) causes the agent to fail at the first attempt.
	StrictMode bool

	// APIToken (--api-token) is an authentication token used for the backend
	// API as an alternative to OAuth flows.
	APIToken string

	// VenConnName (--venafi-connection) is the name of the VenafiConnection
	// resource to use. Using this flag will enable Venafi Connection mode.
	VenConnName string

	// VenConnNS (--venafi-connection-namespace) is the namespace of the
	// VenafiConnection resource to use. It is only useful when the
	// VenafiConnection isn't in the same namespace as the agent.
	//
	// May be left empty to use the same namespace as the agent.
	VenConnNS string

	// InstallNS (--install-namespace) is the namespace in which the agent is
	// running in. Only needed when running the agent outside of Kubernetes.
	//
	// May be left empty when running in Kubernetes. In this case, the namespace
	// is read from the file
	// /var/run/secrets/kubernetes.io/serviceaccount/namespace.
	InstallNS string

	// Profiling (--enable-pprof) enables the pprof server.
	Profiling bool

	// Prometheus (--enable-metrics) enables the Prometheus metrics server.
	Prometheus bool
}

func InitAgentCmdFlags(c *cobra.Command, cfg *AgentCmdFlags) {
	c.PersistentFlags().StringVarP(
		&cfg.ConfigFilePath,
		"agent-config-file",
		"c",
		"./agent.yaml",
		"Config file location, default is `agent.yaml` in the current working directory.",
	)
	c.PersistentFlags().DurationVarP(
		&cfg.Period,
		"period",
		"p",
		0,
		"Override time between scans in the configuration file (given as XhYmZs).",
	)
	c.PersistentFlags().StringVarP(
		&cfg.CredentialsPath,
		"credentials-file",
		"k",
		"",
		"Location of the credentials file. For OAuth2 based authentication.",
	)
	c.PersistentFlags().BoolVarP(
		&cfg.VenafiCloudMode,
		"venafi-cloud",
		"",
		false,
		"Runs agent with parsing config (and credentials file if provided) in Venafi Cloud format if true.",
	)
	c.PersistentFlags().StringVarP(
		&cfg.ClientID,
		"client-id",
		"",
		"",
		"Venafi Cloud Service Account client ID. If you use this flag you don't need to use --venafi-cloud as it will assume you are authenticating against Venafi Cloud. Using this removes the need to use a credentials file with Venafi Cloud mode.",
	)
	c.PersistentFlags().StringVarP(
		&cfg.PrivateKeyPath,
		"private-key-path",
		"",
		"/etc/venafi/agent/key/privatekey.pem",
		"Venafi Cloud Service Account private key path.",
	)
	c.PersistentFlags().BoolVarP(
		&cfg.OneShot,
		"one-shot",
		"",
		false,
		"Runs agent a single time if true, or continously if false",
	)
	c.PersistentFlags().StringVarP(
		&cfg.OutputPath,
		"output-path",
		"",
		"",
		"Output file path, if used, it will write data to a local file instead of uploading to the preflight server",
	)
	c.PersistentFlags().StringVarP(
		&cfg.InputPath,
		"input-path",
		"",
		"",
		"Input file path, if used, it will read data from a local file instead of gathering data from clusters",
	)
	c.PersistentFlags().DurationVarP(
		&cfg.BackoffMaxTime,
		"backoff-max-time",
		"",
		10*time.Minute,
		"Max time for retrying failed data gatherers (given as XhYmZs).",
	)
	c.PersistentFlags().BoolVarP(
		&cfg.StrictMode,
		"strict",
		"",
		false,
		"Runs agent in strict mode. No retry attempts will be made for a missing data gatherer's data.",
	)
	c.PersistentFlags().StringVar(
		&cfg.APIToken,
		"api-token",
		os.Getenv("API_TOKEN"),
		"Token used for authentication when API tokens are in use on the backend",
	)
	c.PersistentFlags().StringVar(
		&cfg.VenConnName,
		"venafi-connection",
		"",
		"Name of the VenafiConnection to be used. Using this flag will enable the VenafiConnection mode.",
	)
	c.PersistentFlags().StringVar(
		&cfg.VenConnNS,
		"venafi-connection-namespace",
		"",
		"Namespace of the VenafiConnection to be used. It is only useful when the VenafiConnection isn't in the same namespace as the agent. The field `allowReferencesFrom` must be present on the cross-namespace VenafiConnection for the agent to use it.",
	)
	c.PersistentFlags().StringVar(
		&cfg.InstallNS,
		"install-namespace",
		"",
		"Namespace in which the agent is running. Only needed when running the agent outside of Kubernetes. Used for testing purposes.",
	)
	c.PersistentFlags().BoolVarP(
		&cfg.Profiling,
		"enable-pprof",
		"",
		false,
		"Enables the pprof profiling server on the agent (port: 6060).",
	)
	c.PersistentFlags().BoolVarP(
		&cfg.Prometheus,
		"enable-metrics",
		"",
		false,
		"Enables Prometheus metrics server on the agent (port: 8081).",
	)
}

// getConfiguration combines the input configuration with the flags passed to
// the agent and returns the final configuration as well as the Venafi client to
// be used to upload data.
func getConfiguration(log *log.Logger, cfg Config, flags AgentCmdFlags) (Config, client.Client, error) {
	// If the ClientID of the service account is specified, then assume we are in Venafi Cloud mode.
	if flags.ClientID != "" || flags.VenConnName != "" {
		flags.VenafiCloudMode = true
	}

	baseURL := cfg.Server
	if baseURL == "" {
		log.Printf("Using deprecated Endpoint configuration. User Server instead.")
		baseURL = fmt.Sprintf("%s://%s", cfg.Endpoint.Protocol, cfg.Endpoint.Host)
		_, err := url.Parse(baseURL)
		if err != nil {
			return Config{}, nil, fmt.Errorf("failed to parse server URL: %w", err)
		}
	}

	if flags.Period == 0 && cfg.Period == 0 && !flags.OneShot {
		return Config{}, nil, fmt.Errorf("period must be set as a flag or in config")
	}

	var credentials client.Credentials
	var err error
	if flags.ClientID != "" {
		credentials = &client.VenafiSvcAccountCredentials{
			ClientID:       flags.ClientID,
			PrivateKeyFile: flags.PrivateKeyPath,
		}
	} else if flags.CredentialsPath != "" {
		file, err := os.Open(flags.CredentialsPath)
		if err != nil {
			return Config{}, nil, fmt.Errorf("failed to load credentials from file %s: %w", flags.CredentialsPath, err)
		}
		defer file.Close()

		b, err := io.ReadAll(file)
		if err != nil {
			return Config{}, nil, fmt.Errorf("failed to read credentials file: %w", err)
		}
		if flags.VenafiCloudMode {
			credentials, err = client.ParseVenafiCredentials(b)
		} else {
			credentials, err = client.ParseOAuthCredentials(b)
		}
		if err != nil {
			return Config{}, nil, fmt.Errorf("failed to parse credentials file: %w", err)
		}
	}

	venConnMode := flags.VenConnName != ""

	if venConnMode && flags.InstallNS == "" {
		flags.InstallNS, err = getInClusterNamespace()
		if err != nil {
			return Config{}, nil, fmt.Errorf("could not guess which namespace the agent is running in: %w", err)
		}
	}
	if venConnMode && flags.VenConnNS == "" {
		flags.VenConnNS = flags.InstallNS
	}

	agentMetadata := &api.AgentMetadata{
		Version:   version.PreflightVersion,
		ClusterID: cfg.ClusterID,
	}

	var preflightClient client.Client
	switch {
	case credentials != nil:
		preflightClient, err = createCredentialClient(log, credentials, cfg, agentMetadata, baseURL)
	case flags.VenConnName != "":
		// Why wasn't this added to the createCredentialClient instead? Because
		// the --venafi-connection mode of authentication doesn't need any
		// secrets (or any other information for that matter) to be loaded from
		// disk (using --credentials-path). Everything is passed as flags.
		log.Println("Venafi Connection mode was specified, using Venafi Connection authentication.")

		// The venafi-cloud.upload_path was initially meant to let users
		// configure HTTP proxies, but it has never been used since HTTP proxies
		// don't rewrite paths. Thus, we've disabled the ability to change this
		// value with the new --venafi-connection flag, and this field is simply
		// ignored.
		if cfg.VenafiCloud != nil && cfg.VenafiCloud.UploadPath != "" {
			log.Printf(`ignoring venafi-cloud.upload_path. In Venafi Connection mode, this field is not needed.`)
		}

		// Regarding venafi-cloud.uploader_id, we found that it doesn't do
		// anything in the backend. Since the backend requires it for historical
		// reasons (but cannot be empty), we just ignore whatever the user has
		// set in the config file, and set it to an arbitrary value in the
		// client since it doesn't matter.
		if cfg.VenafiCloud != nil && cfg.VenafiCloud.UploaderID != "" {
			log.Printf(`ignoring venafi-cloud.uploader_id. In Venafi Connection mode, this field is not needed.`)
		}

		restCfg, err := kubeconfig.LoadRESTConfig("")
		if err != nil {
			return Config{}, nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}

		preflightClient, err = client.NewVenConnClient(restCfg, agentMetadata, flags.InstallNS, flags.VenConnName, flags.VenConnNS, nil)
	case flags.APIToken != "":
		log.Println("An API token was specified, using API token authentication.")
		preflightClient, err = client.NewAPITokenClient(agentMetadata, flags.APIToken, baseURL)
	default:
		log.Println("No credentials were specified, using with no authentication.")
		preflightClient, err = client.NewUnauthenticatedClient(agentMetadata, baseURL)
	}

	if err != nil {
		return Config{}, nil, fmt.Errorf("failed to create client: %w", err)
	}

	return cfg, preflightClient, nil
}

func createCredentialClient(log *log.Logger, credentials client.Credentials, config Config, agentMetadata *api.AgentMetadata, baseURL string) (client.Client, error) {
	switch creds := credentials.(type) {
	case *client.VenafiSvcAccountCredentials:
		log.Println("Venafi Cloud mode was specified, using Venafi Service Account authentication.")
		// check if config has Venafi Cloud data, use config data if it's present
		uploaderID := creds.ClientID
		uploadPath := ""
		if config.VenafiCloud != nil {
			log.Println("Loading uploader_id and upload_path from \"venafi-cloud\" configuration.")
			uploaderID = config.VenafiCloud.UploaderID
			uploadPath = config.VenafiCloud.UploadPath
		}
		return client.NewVenafiCloudClient(agentMetadata, creds, baseURL, uploaderID, uploadPath)

	case *client.OAuthCredentials:
		log.Println("A credentials file was specified, using oauth authentication.")
		return client.NewOAuthClient(agentMetadata, creds, baseURL)
	default:
		return nil, errors.New("credentials file is in unknown format")
	}
}

// Inspired by the controller-runtime project.
func getInClusterNamespace() (string, error) {
	// Check whether the namespace file exists.
	// If not, we are not running in cluster so can't guess the namespace.
	_, err := os.Stat(inClusterNamespacePath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("not running in cluster, please use --install-namespace to specify the namespace in which the agent is running")
	}
	if err != nil {
		return "", fmt.Errorf("error checking namespace file: %w", err)
	}

	namespace, err := os.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %w", err)
	}
	return string(namespace), nil
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

func (c *Config) validate(isVenafiCloudMode bool) error {
	var result *multierror.Error

	// configured for Venafi Cloud
	if c.VenafiCloud != nil {
		if c.VenafiCloud.UploadPath == "" {
			result = multierror.Append(result, fmt.Errorf("upload_path is required in Venafi Cloud mode"))
		}

		if _, err := url.Parse(c.VenafiCloud.UploadPath); err != nil {
			result = multierror.Append(result, fmt.Errorf("upload_path is not a valid URL"))
		}
	} else if !isVenafiCloudMode {
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
func ParseConfig(data []byte, isVenafiCloudMode bool) (Config, error) {
	var config Config

	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}

	if config.Server == "" && config.Endpoint.Host == "" && config.Endpoint.Path == "" {
		config.Server = "https://preflight.jetstack.io"
		if config.VenafiCloud != nil || isVenafiCloudMode {
			config.Server = client.VenafiCloudProdURL
		}
	}

	if config.Endpoint.Protocol == "" && config.Server == "" {
		config.Endpoint.Protocol = "http"
	}

	err = config.validate(isVenafiCloudMode)
	if err != nil {
		return config, err
	}

	return config, nil
}
