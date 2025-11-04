package agent

import (
	"crypto/x509"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	"github.com/jetstack/venafi-connection-lib/http_client"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/rest"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"github.com/jetstack/preflight/pkg/datagatherer/local"
	"github.com/jetstack/preflight/pkg/kubeconfig"
	"github.com/jetstack/preflight/pkg/logs"
	"github.com/jetstack/preflight/pkg/version"
)

// Config defines the YAML configuration file that you can pass using
// `--config-file` or `-c`.
type Config struct {
	// Deprecated: Schedule doesn't do anything. Use `period` instead.
	Schedule string        `yaml:"schedule"`
	Period   time.Duration `yaml:"period"`

	// Deprecated: Use `server` instead.
	Endpoint Endpoint `yaml:"endpoint"`

	// Server is the base URL for the Preflight server. It defaults to
	// https://preflight.jetstack.io in Jetstack Secure OAuth and Jetstack
	// Secure API Token modes, and https://api.venafi.cloud in Venafi Cloud Key
	// Pair Service Account mode. It is ignored in Venafi Cloud VenafiConnection
	// mode and in MachineHub mode.
	Server string `yaml:"server"`

	// OrganizationID is only used in Jetstack Secure OAuth and Jetstack Secure
	// API Token modes.
	OrganizationID string `yaml:"organization_id"`

	// ClusterID is the cluster that the agent is scanning. Only used in Jetstack Secure modes.
	ClusterID string `yaml:"cluster_id"`
	// ClusterName is the name of the Kubernetes cluster where the agent is running.
	ClusterName string `yaml:"cluster_name"`
	// ClusterDescription is a short description of the Kubernetes cluster where the
	// agent is running.
	ClusterDescription string             `yaml:"cluster_description"`
	DataGatherers      []DataGatherer     `yaml:"data-gatherers"`
	VenafiCloud        *VenafiCloudConfig `yaml:"venafi-cloud,omitempty"`

	// For testing purposes.
	InputPath string `yaml:"input-path"`
	// For testing purposes.
	OutputPath string `yaml:"output-path"`

	// Skips annotation keys that match the given set of regular expressions.
	// Example: ".*someprivateannotation.*".
	ExcludeAnnotationKeysRegex []string `yaml:"exclude-annotation-keys-regex"`
	// Skips label keys that match the given set of regular expressions.
	ExcludeLabelKeysRegex []string `yaml:"exclude-label-keys-regex"`
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
	// Deprecated: UploaderID is ignored by the backend and is not needed.
	// UploaderID is the upload ID that will be used when creating a cluster
	// connection. This field is ignored by the backend and is often arbitrarily
	// set to "no".
	UploaderID string `yaml:"uploader_id,omitempty"`

	// UploadPath is the endpoint path for the upload API. Only used in Venafi
	// Cloud Key Pair Service Account mode.
	UploadPath string `yaml:"upload_path,omitempty"`
}

type AgentCmdFlags struct {
	// ConfigFilePath (--config-file, -c) is the path to the agent configuration
	// YAML file.
	ConfigFilePath string

	// Period (--period, -p) is the time waited between scans. It takes
	// precedence over the config field `period`.
	Period time.Duration

	// VenafiCloudMode (--venafi-cloud) turns on the Venafi Cloud Key Pair
	// Service Account mode. Must be used in conjunction with
	// --credentials-file.
	VenafiCloudMode bool

	// MachineHubMode configures the agent to send data to CyberArk Machine Hub.
	MachineHubMode bool

	// ClientID (--client-id) is the clientID in case of Venafi Cloud Key Pair
	// Service Account mode.
	ClientID string

	// PrivateKeyPath (--private-key-path) is the path for the service account
	// private key in case of Venafi Cloud Key Pair Service Account mode.
	PrivateKeyPath string

	// CredentialsPath (--credentials-file, -k) lets you specify the location of
	// the credentials file. This is used for the Jetstack Secure OAuth and
	// Venafi Cloud Key Pair Service Account modes. In Venafi Cloud Key Pair
	// Service Account mode, you also need to pass --venafi-cloud.
	CredentialsPath string

	// OneShot (--one-shot) is used for testing purposes. The agent will run
	// once and exit. It is often used in conjunction with --output-path and/or
	// --input-path.
	OneShot bool

	// OutputPath (--output-path) is used for testing purposes. In conjunction
	// with --one-shot, it allows you to write the data readings to a file
	// instead uploading them to the Venafi Cloud API.
	OutputPath string

	// InputPath (--input-path) is used for testing purposes. In conjunction
	// with --one-shot, it allows you to push manually crafted data readings (in
	// JSON format) to the Venafi Cloud API without the need to connect to a
	// Kubernetes cluster. See the jscp-testing-cli's README for more info:
	// https://gitlab.com/venafi/vaas/applications/tls-protect-for-k8s/cloud-services/-/tree/master/jscp-testing-cli
	InputPath string

	// BackoffMaxTime (--backoff-max-time) is the maximum time for which data
	// gatherers will retry after a failure.
	BackoffMaxTime time.Duration

	// StrictMode (--strict) causes the agent to fail at the first attempt.
	StrictMode bool

	// APIToken (--api-token) allows you to use the Jetstack Secure API Token
	// mode. Defaults to the value of the env var API_TOKEN.
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
	// May be left empty when running in Kubernetes. In Kubernetes, the
	// namespace is read from the environment variable `POD_NAMESPACE`.
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
		fmt.Sprintf("Location of the credentials file. For the %s and %s modes.", JetstackSecureOAuth, VenafiCloudKeypair),
	)
	c.PersistentFlags().BoolVarP(
		&cfg.VenafiCloudMode,
		"venafi-cloud",
		"",
		false,
		fmt.Sprintf("Turns on the %s mode. The flag --credentials-file must also be passed.", JetstackSecureOAuth),
	)
	if err := c.PersistentFlags().MarkHidden("venafi-cloud"); err != nil {
		panic(err)
	}
	c.PersistentFlags().StringVarP(
		&cfg.ClientID,
		"client-id",
		"",
		"",
		fmt.Sprintf("Turns on the %s mode. If you use this flag you don't need to use --venafi-cloud "+
			"as it will assume you are authenticating with Venafi Cloud. Using this removes the need to use a "+
			"credentials file.", VenafiCloudKeypair),
	)
	c.PersistentFlags().StringVarP(
		&cfg.PrivateKeyPath,
		"private-key-path",
		"",
		"",
		"To be used in conjunction with --client-id. The path to the private key file for the service account.",
	)
	c.PersistentFlags().BoolVarP(
		&cfg.OneShot,
		"one-shot",
		"",
		false,
		"For testing purposes. The agent will run once and exit. It is often used in conjunction with --output-path and/or --input-path.",
	)
	c.PersistentFlags().StringVarP(
		&cfg.OutputPath,
		"output-path",
		"",
		"",
		"For testing purposes. In conjunction with --one-shot, it allows you to write the data readings to a file instead of uploading to the server.",
	)
	c.PersistentFlags().StringVarP(
		&cfg.InputPath,
		"input-path",
		"",
		"",
		"For testing purposes. In conjunction with --one-shot, it allows you to push manually crafted data readings (in JSON format) to the Venafi Cloud API without the need to connect to a Kubernetes cluster.",
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
		"Turns on the "+string(JetstackSecureAPIToken)+" mode. Defaults to the value of the env var API_TOKEN.",
	)
	c.PersistentFlags().StringVar(
		&cfg.VenConnName,
		"venafi-connection",
		"",
		"Turns on the "+string(VenafiCloudVenafiConnection)+" mode. "+
			"This flag configures the name of the VenafiConnection to be used.",
	)
	c.PersistentFlags().StringVar(
		&cfg.VenConnNS,
		"venafi-connection-namespace",
		"",
		"Namespace of the VenafiConnection to be used. It is only useful when the "+
			"VenafiConnection isn't in the same namespace as the agent. The field `allowReferencesFrom` "+
			"must be present on the cross-namespace VenafiConnection for the agent to use it.",
	)
	c.PersistentFlags().StringVar(
		&cfg.InstallNS,
		"install-namespace",
		"",
		"For testing purposes. Namespace in which the agent is running. "+
			"Only needed when running the agent outside of Kubernetes.",
	)
	c.PersistentFlags().BoolVarP(
		&cfg.Profiling,
		"enable-pprof",
		"",
		false,
		"Enables the pprof profiling endpoints on the agent server (port: 8081).",
	)
	c.PersistentFlags().BoolVarP(
		&cfg.Prometheus,
		"enable-metrics",
		"",
		false,
		"Enables Prometheus metrics server on the agent (port: 8081).",
	)

	var dummy bool
	c.PersistentFlags().BoolVar(
		&dummy,
		"disable-compression",
		false,
		"Deprecated. No longer has an effect.",
	)
	if err := c.PersistentFlags().MarkDeprecated("disable-compression", "no longer has an effect"); err != nil {
		panic(err)
	}

	// This is a hidden feature flag we use to build the "Machine Hub" feature
	// gradually without impacting customers. Once the feature is GA, we will
	// turn this flag "on" by default.
	c.PersistentFlags().BoolVar(
		&cfg.MachineHubMode,
		"machine-hub",
		false,
		"Enables the MachineHub mode. The agent will push data to CyberArk MachineHub.",
	)
	if err := c.PersistentFlags().MarkHidden("machine-hub"); err != nil {
		panic(err)
	}

}

// OutputMode controls how the collected data is published.
// Only one OutputMode may be provided.
type OutputMode string

const (
	JetstackSecureOAuth         OutputMode = "Jetstack Secure OAuth"
	JetstackSecureAPIToken      OutputMode = "Jetstack Secure API Token"
	VenafiCloudKeypair          OutputMode = "Venafi Cloud Key Pair Service Account"
	VenafiCloudVenafiConnection OutputMode = "Venafi Cloud VenafiConnection"
	LocalFile                   OutputMode = "Local File"
	MachineHub                  OutputMode = "MachineHub"
)

// The command-line flags and the config file and some environment variables are
// combined into this struct by ValidateAndCombineConfig.
type CombinedConfig struct {
	DataGatherers  []DataGatherer
	Period         time.Duration
	BackoffMaxTime time.Duration
	InstallNS      string
	StrictMode     bool
	OneShot        bool

	OutputMode OutputMode

	// Only used in JetstackSecure modes.
	ClusterID string

	// Used by JetstackSecureOAuth, JetstackSecureAPIToken, and
	// VenafiCloudKeypair. Ignored in VenafiCloudVenafiConnection mode.
	Server string

	// JetstackSecureOAuth and JetstackSecureAPIToken modes only.
	OrganizationID string
	EndpointPath   string // Deprecated.

	// VenafiCloudKeypair mode only.
	UploadPath string

	// ClusterName is the name of the Kubernetes cluster where the agent is
	// running.
	ClusterName string

	// ClusterDescription is a short description of the Kubernetes cluster where
	// the agent is running.
	ClusterDescription string

	// VenafiCloudVenafiConnection mode only.
	VenConnName string
	VenConnNS   string

	// VenafiCloudKeypair and VenafiCloudVenafiConnection modes only.
	ExcludeAnnotationKeysRegex []*regexp.Regexp
	ExcludeLabelKeysRegex      []*regexp.Regexp

	// Only used for testing purposes.
	OutputPath string
	InputPath  string
}

// ValidateAndCombineConfig combines and validates the input configuration with
// the flags passed to the agent and returns the final configuration as well as
// the Venafi client to be used to upload data. Does not do any network call.
// The logger can be changed for testing purposes. You do not need to call
// ValidateDataGatherers as ValidateAndCombineConfig already does that.
//
// The error returned may be a multierror.Error. Use multierror.Prefix(err,
// "context:") rather than fmt.Errorf("context: %w", err) when wrapping the
// error.
func ValidateAndCombineConfig(log logr.Logger, cfg Config, flags AgentCmdFlags) (CombinedConfig, client.Client, error) {
	res := CombinedConfig{}

	{
		var (
			mode          OutputMode
			reason        string
			keysAndValues []any
		)
		switch {
		case flags.VenafiCloudMode && flags.CredentialsPath != "":
			mode = VenafiCloudKeypair
			reason = "--venafi-cloud and --credentials-path were specified"
			keysAndValues = []any{"credentialsPath", flags.CredentialsPath}
		case flags.ClientID != "" || flags.PrivateKeyPath != "":
			if flags.PrivateKeyPath == "" {
				return CombinedConfig{}, nil, fmt.Errorf("if --client-id is specified, --private-key-path must also be specified")
			}
			if flags.ClientID == "" {
				return CombinedConfig{}, nil, fmt.Errorf("--private-key-path is specified, --client-id must also be specified")
			}

			mode = VenafiCloudKeypair
			reason = "--client-id and --private-key-path were specified"
			keysAndValues = []any{"clientID", flags.ClientID, "privateKeyPath", flags.PrivateKeyPath}
		case flags.VenConnName != "":
			mode = VenafiCloudVenafiConnection
			reason = "--venafi-connection was specified"
			keysAndValues = []any{"venConnName", flags.VenConnName}
		case flags.APIToken != "":
			mode = JetstackSecureAPIToken
			reason = "--api-token was specified"
		case !flags.VenafiCloudMode && flags.CredentialsPath != "":
			mode = JetstackSecureOAuth
			reason = "--credentials-file was specified without --venafi-cloud"
		case flags.MachineHubMode:
			mode = MachineHub
			reason = "--machine-hub was specified"
		case flags.OutputPath != "":
			mode = LocalFile
			reason = "--output-path was specified"
		case cfg.OutputPath != "":
			mode = LocalFile
			reason = "output-path was specified in the config file"
		default:
			return CombinedConfig{}, nil, fmt.Errorf("no output mode specified. " +
				"To enable one of the output modes, you can:\n" +
				" - Use (--venafi-cloud with --credentials-file) or (--client-id with --private-key-path) to use the " + string(VenafiCloudKeypair) + " mode.\n" +
				" - Use --venafi-connection for the " + string(VenafiCloudVenafiConnection) + " mode.\n" +
				" - Use --credentials-file alone if you want to use the " + string(JetstackSecureOAuth) + " mode.\n" +
				" - Use --api-token if you want to use the " + string(JetstackSecureAPIToken) + " mode.\n" +
				" - Use --machine-hub if you want to use the " + string(MachineHub) + " mode.\n" +
				" - Use --output-path or output-path in the config file for " + string(LocalFile) + " mode.")
		}

		keysAndValues = append(keysAndValues, "mode", mode, "reason", reason)
		log.V(logs.Debug).Info("Output mode selected", keysAndValues...)
		res.OutputMode = mode
	}

	var errs error

	// Validation and defaulting of `server` and the deprecated `endpoint.path`.
	{
		// Only relevant if using TLSPK backends
		hasEndpointField := cfg.Endpoint.Host != "" && cfg.Endpoint.Path != ""
		hasServerField := cfg.Server != ""
		var server string
		var endpointPath string // Deprecated. Only used when the `endpoint` field is set.
		switch {
		case hasServerField && !hasEndpointField:
			server = cfg.Server
		case hasServerField && hasEndpointField:
			// The `server` field takes precedence over the deprecated
			// `endpoint` field.
			log.Info("The `server` and `endpoint` fields are both set in the config; using the `server` field.")
			server = cfg.Server
		case !hasServerField && hasEndpointField:
			log.Info("Using deprecated Endpoint configuration. User Server instead.")
			if cfg.Endpoint.Protocol == "" && cfg.Server == "" {
				cfg.Endpoint.Protocol = "http"
			}
			server = fmt.Sprintf("%s://%s", cfg.Endpoint.Protocol, cfg.Endpoint.Host)
			endpointPath = cfg.Endpoint.Path
		case !hasServerField && !hasEndpointField:
			server = "https://preflight.jetstack.io"
			if res.OutputMode == VenafiCloudKeypair {
				// The VenafiCloudVenafiConnection mode doesn't need a server.
				server = client.VenafiCloudProdURL
			}
		}
		url, urlErr := url.Parse(server)
		if urlErr != nil || url.Hostname() == "" {
			errs = multierror.Append(errs, fmt.Errorf("server %q is not a valid URL", server))
		}
		if res.OutputMode == VenafiCloudVenafiConnection && server != "" {
			log.Info(fmt.Sprintf("ignoring the server field specified in the config file. In %s mode, this field is not needed.", VenafiCloudVenafiConnection))
			server = ""
		}
		res.Server = server
		res.EndpointPath = endpointPath
	}

	// Validation of `venafi-cloud.upload_path`.
	{
		var uploadPath string
		switch res.OutputMode { // nolint:exhaustive
		case VenafiCloudKeypair:
			if cfg.VenafiCloud == nil || cfg.VenafiCloud.UploadPath == "" {
				errs = multierror.Append(errs, fmt.Errorf("the venafi-cloud.upload_path field is required when using the %s mode", res.OutputMode))
				break // Skip to the end of the switch statement.
			}
			_, urlErr := url.Parse(cfg.VenafiCloud.UploadPath)
			if urlErr != nil {
				errs = multierror.Append(errs, fmt.Errorf("upload_path is not a valid URL"))
				break // Skip to the end of the switch statement.
			}

			uploadPath = cfg.VenafiCloud.UploadPath
		case VenafiCloudVenafiConnection:
			// The venafi-cloud.upload_path was initially meant to let users
			// configure HTTP proxies, but it has never been used since HTTP
			// proxies don't rewrite paths. Thus, we've disabled the ability to
			// change this value with the new --venafi-connection flag, and this
			// field is simply ignored.
			if cfg.VenafiCloud != nil && cfg.VenafiCloud.UploadPath != "" {
				log.Info(fmt.Sprintf(`ignoring the venafi-cloud.upload_path field in the config file. In %s mode, this field is not needed.`, res.OutputMode))
			}
			uploadPath = ""
		}
		res.UploadPath = uploadPath
	}

	// Validation of `uploader_id`.
	//
	// We found that `venafi-cloud.uploader_id` doesn't do anything in the
	// backend. Since the backend requires it for historical reasons (but cannot
	// be empty), we just ignore whatever the user has set in the config file,
	// and set it to an arbitrary value in the client since it doesn't matter.
	//
	// TODO(mael): Remove the arbitrary `/no` path parameter from the Agent once
	// https://venafi.atlassian.net/browse/VC-35385 is done.
	{
		if cfg.VenafiCloud != nil && cfg.VenafiCloud.UploaderID != "" {
			log.Info(fmt.Sprintf(`ignoring the venafi-cloud.uploader_id field in the config file. This field is not needed in %s mode.`, res.OutputMode))
		}
	}

	// Validation of `cluster_name`, `cluster_id` and `organization_id`.
	{
		var clusterName string    // Required by venafi cloud modes. Optional for MachineHub mode.
		var clusterID string      // Required by the old jetstack-secure mode deprecated for venafi cloud modes.
		var organizationID string // Only used by the old jetstack-secure mode.
		switch res.OutputMode {   // nolint:exhaustive
		case VenafiCloudKeypair, VenafiCloudVenafiConnection:
			// For backwards compatibility, use the agent config's `cluster_id` as
			// ClusterName if `cluster_name` is not set.
			if cfg.ClusterName == "" && cfg.ClusterID == "" {
				errs = multierror.Append(errs, fmt.Errorf("cluster_name or cluster_id is required in %s mode", res.OutputMode))
			}
			if cfg.ClusterName != "" && cfg.ClusterID != "" {
				log.Info(fmt.Sprintf(`Ignoring the cluster_id field in the config file. This field is not needed in %s mode.`, res.OutputMode))
			}
			clusterName = cfg.ClusterName
			if clusterName == "" {
				log.Info("Using cluster_id as cluster_name for backwards compatibility", "clusterID", cfg.ClusterID)
				clusterName = cfg.ClusterID
			}
			if cfg.OrganizationID != "" {
				log.Info(fmt.Sprintf(`Ignoring the organization_id field in the config file. This field is not needed in %s mode.`, res.OutputMode))
			}
		case JetstackSecureOAuth, JetstackSecureAPIToken:
			if cfg.OrganizationID == "" {
				errs = multierror.Append(errs, fmt.Errorf("organization_id is required"))
			}
			if cfg.ClusterID == "" {
				errs = multierror.Append(errs, fmt.Errorf("cluster_id is required"))
			}
			organizationID = cfg.OrganizationID
			clusterID = cfg.ClusterID
		case MachineHub:
			clusterName = cfg.ClusterName
			if clusterName == "" {
				if arkUsername, found := os.LookupEnv("ARK_USERNAME"); found {
					log.Info("Using ARK_USERNAME environment variable as cluster name", "clusterName", arkUsername)
					clusterName = arkUsername
				}
			}
			if cfg.OrganizationID != "" {
				log.Info(fmt.Sprintf(`Ignoring the organization_id field in the config file. This field is not needed in %s mode.`, res.OutputMode))
			}
			if cfg.ClusterID != "" {
				log.Info(fmt.Sprintf(`Ignoring the cluster_id field in the config file. This field is not needed in %s mode.`, res.OutputMode))
			}
		}
		res.OrganizationID = organizationID
		res.ClusterID = clusterID
		res.ClusterName = clusterName
		res.ClusterDescription = cfg.ClusterDescription
	}

	// Validation of `data-gatherers`.
	{
		if dgErr := ValidateDataGatherers(cfg.DataGatherers); dgErr != nil {
			errs = multierror.Append(errs, dgErr)
		}
		res.DataGatherers = cfg.DataGatherers
	}

	// Validation of --period, -p, and the `period` field, as well as
	// --backoff-max-time, --one-shot, and --strict. The flag --period/-p takes
	// precedence over the config `period`.
	{
		var period time.Duration
		switch {
		case flags.OneShot:
			// OneShot mode doesn't need a period, skipping validation.
		case flags.Period == 0 && cfg.Period == 0:
			errs = multierror.Append(errs, fmt.Errorf("period must be set using --period or -p, or using the 'period' field in the config file"))
		case flags.Period == 0 && cfg.Period > 0:
			log.Info("Using period from config", "period", cfg.Period)
			period = cfg.Period
		case flags.Period > 0 && cfg.Period == 0:
			period = flags.Period
		case flags.Period > 0 && cfg.Period > 0:
			// The flag takes precedence.
			log.Info("Both the 'period' field and --period are set. Using the value provided with --period.")
			period = flags.Period
		}
		res.Period = period
		res.OneShot = flags.OneShot
		res.BackoffMaxTime = flags.BackoffMaxTime
		res.StrictMode = flags.StrictMode
	}

	// Validation of --install-namespace.
	{
		installNS := flags.InstallNS
		if installNS == "" {
			var err error
			installNS, err = getInClusterNamespace()
			if err != nil {
				if res.OutputMode == VenafiCloudVenafiConnection {
					errs = multierror.Append(errs, fmt.Errorf("could not guess which namespace the agent is running in: %w", err))
				}
			}
		}
		res.InstallNS = installNS
	}

	// Validation of --venafi-connection and --venafi-connection-namespace.
	if res.OutputMode == VenafiCloudVenafiConnection {
		res.VenConnName = flags.VenConnName
		venConnNS := flags.VenConnNS
		if flags.VenConnNS == "" {
			venConnNS = res.InstallNS
		}
		res.VenConnNS = venConnNS
	}

	// Validation of --output-path, --input-path, `output-path`, and
	// `input-path`. The flags --output-path and --input-path take precedence.
	{
		res.InputPath = cfg.InputPath
		res.OutputPath = cfg.OutputPath
		if flags.OutputPath != "" {
			res.OutputPath = flags.OutputPath
		}
		if flags.InputPath != "" {
			res.InputPath = flags.InputPath
		}
	}

	// Validation of the config fields exclude_annotation_keys_regex and
	// exclude_label_keys_regex.
	{
		for i, regex := range cfg.ExcludeAnnotationKeysRegex {
			r, err := regexp.Compile(regex)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("invalid exclude_annotation_keys_regex[%d]: %w", i, err))
				continue
			}
			res.ExcludeAnnotationKeysRegex = append(res.ExcludeAnnotationKeysRegex, r)
		}
		for i, regex := range cfg.ExcludeLabelKeysRegex {
			r, err := regexp.Compile(regex)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("invalid exclude_label_keys_regex[%d]: %w", i, err))
				continue
			}
			res.ExcludeLabelKeysRegex = append(res.ExcludeLabelKeysRegex, r)
		}
	}

	if errs != nil {
		return CombinedConfig{}, nil, errs
	}

	outputClient, err := validateCredsAndCreateClient(log, flags.CredentialsPath, flags.ClientID, flags.PrivateKeyPath, flags.APIToken, res)
	if err != nil {
		return CombinedConfig{}, nil, multierror.Prefix(err, "validating creds:")
	}

	return res, outputClient, nil
}

// Validation of --credentials-file/-k, --client-id, and --private-key-path,
// --api-token, and creation of the client.
//
// The error returned may be a multierror.Error. Use multierror.Prefix(err,
// "context:") rather than fmt.Errorf("context: %w", err) when wrapping the
// error.
func validateCredsAndCreateClient(log logr.Logger, flagCredentialsPath, flagClientID, flagPrivateKeyPath, flagAPIToken string, cfg CombinedConfig) (client.Client, error) {
	var errs error

	var outputClient client.Client
	metadata := &api.AgentMetadata{Version: version.PreflightVersion, ClusterID: cfg.ClusterID}
	switch cfg.OutputMode {
	case JetstackSecureOAuth:
		// Note that there are no command line flags to configure the
		// JetstackSecureOAuth mode.
		credsBytes, err := readCredentialsFile(flagCredentialsPath)
		if err != nil {
			errs = multierror.Append(errs, multierror.Prefix(err, "credentials file:"))
			break // Don't continue with parsing if could not load the file.
		}

		creds, err := client.ParseOAuthCredentials(credsBytes)
		if err != nil {
			errs = multierror.Append(errs, multierror.Prefix(err, "credentials file:"))
			break // Don't continue with the client if credentials file invalid.
		}

		outputClient, err = client.NewOAuthClient(metadata, creds, cfg.Server)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	case VenafiCloudKeypair:
		var creds *client.VenafiSvcAccountCredentials

		if flagClientID != "" && flagCredentialsPath != "" {
			errs = multierror.Append(errs, fmt.Errorf("--client-id and --credentials-file cannot be used simultaneously"))
			break
		}
		if flagPrivateKeyPath != "" && flagCredentialsPath != "" {
			errs = multierror.Append(errs, fmt.Errorf("--private-key-path and --credentials-file cannot be used simultaneously"))
			break
		}
		if flagClientID == "" && flagPrivateKeyPath == "" && flagCredentialsPath == "" {
			errs = multierror.Append(errs, fmt.Errorf("either --client-id and --private-key-path or --credentials-file must be provided"))
			break
		}

		switch {
		case flagClientID != "" && flagPrivateKeyPath != "":
			// If --client-id and --private-key-path are passed, then
			// --credentials-file is ignored.
			creds = &client.VenafiSvcAccountCredentials{
				ClientID:       flagClientID,
				PrivateKeyFile: flagPrivateKeyPath,
			}
		case flagCredentialsPath != "":
			credsBytes, err := readCredentialsFile(flagCredentialsPath)
			if err != nil {
				errs = multierror.Append(errs, multierror.Prefix(err, "credentials file:"))
				break // Don't continue if couldn't read the creds file.
			}
			creds, err = client.ParseVenafiCredentials(credsBytes)
			if err != nil {
				errs = multierror.Append(errs, multierror.Prefix(err, "credentials file:"))
				break // Don't continue with the client since creds is invalid.
			}
		default:
			return nil, fmt.Errorf("programmer mistake: --client-id and --private-key-path or --credentials-file must have been provided")
		}

		// The uploader ID isn't actually used in the backend, let's use an
		// arbitrary value.
		uploaderID := "no"

		// We don't do this for the VenafiCloudVenafiConnection mode because
		// the upload_path field is ignored in that mode.
		log.Info("Loading upload_path from \"venafi-cloud\" configuration.")

		var err error
		outputClient, err = client.NewVenafiCloudClient(metadata, creds, cfg.Server, uploaderID, cfg.UploadPath)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	case VenafiCloudVenafiConnection:
		var restCfg *rest.Config
		restCfg, err := kubeconfig.LoadRESTConfig("")
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("loading kubeconfig: %w", err))
			break // Don't continue with the client if kubeconfig wasn't loaded.
		}

		outputClient, err = client.NewVenConnClient(restCfg, metadata, cfg.InstallNS, cfg.VenConnName, cfg.VenConnNS, nil)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	case JetstackSecureAPIToken:
		var err error
		outputClient, err = client.NewAPITokenClient(metadata, flagAPIToken, cfg.Server)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	case LocalFile:
		outputClient = client.NewFileClient(cfg.OutputPath)
	case MachineHub:
		var (
			err     error
			rootCAs *x509.CertPool
		)
		httpClient := http_client.NewDefaultClient(version.UserAgent(), rootCAs)
		outputClient, err = client.NewCyberArk(httpClient)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	default:
		panic(fmt.Errorf("programmer mistake: output mode not implemented: %s", cfg.OutputMode))
	}

	if errs != nil {
		return nil, fmt.Errorf("failed loading config using the %s mode: %w", cfg.OutputMode, errs)
	}

	return outputClient, nil
}

// Same as ValidateAndCombineConfig but just for validating the data gatherers.
// This is separate because the `rbac` command only needs to validate the data
// gatherers, nothing else.
//
// The error returned may be a multierror.Error. Use multierror.Prefix(err,
// "context:") rather than fmt.Errorf("context: %w", err) when wrapping the
// error.
func ValidateDataGatherers(dataGatherers []DataGatherer) error {
	var err error
	for i, v := range dataGatherers {
		if v.Kind == "" {
			err = multierror.Append(err, fmt.Errorf("datagatherer %d/%d is missing a kind", i+1, len(dataGatherers)))
		}
		if v.Name == "" {
			err = multierror.Append(err, fmt.Errorf("datagatherer %d/%d is missing a name", i+1, len(dataGatherers)))
		}
	}

	return err
}

// Inspired by the controller-runtime project.
func getInClusterNamespace() (string, error) {
	ns := os.Getenv("POD_NAMESPACE")
	if ns != "" {
		return ns, nil
	}

	return "", fmt.Errorf("POD_NAMESPACE env var not set, meaning that you are probably not running in cluster. Please use --install-namespace or POD_NAMESPACE to specify the namespace in which the agent is running.")
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
		return "", fmt.Errorf("failed to generate YAML dump of config: %w", err)
	}

	return string(d), nil
}

// ParseConfig only parses. It does not validate anything except for the data
// gatherer types. To validate the config, use ValidateDataGatherers or
// getConfiguration.
func ParseConfig(data []byte) (Config, error) {
	var config Config

	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}

	return config, nil
}

type credType string

const (
	CredOldJetstackSecureOAuth credType = "CredOldJetstackSecureOAuth"
	CredVenafiCloudKeypair     credType = "CredVenafiCloudKeypair"
)

func readCredentialsFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials from file %s: %w", path, err)
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	return b, nil
}
