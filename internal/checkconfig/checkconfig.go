// Package checkconfig provides all the necessary components for the `check`
// command to load its configuration.
package checkconfig

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/datagatherer/aks"
	"github.com/jetstack/preflight/pkg/datagatherer/eks"
	"github.com/jetstack/preflight/pkg/datagatherer/gke"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	localdatagatherer "github.com/jetstack/preflight/pkg/datagatherer/local"
	"github.com/jetstack/preflight/pkg/output"
	"github.com/jetstack/preflight/pkg/output/azblob"
	"github.com/jetstack/preflight/pkg/output/cli"
	"github.com/jetstack/preflight/pkg/output/gcs"
	localoutput "github.com/jetstack/preflight/pkg/output/local"
	"github.com/jetstack/preflight/pkg/packagesources"
	"github.com/jetstack/preflight/pkg/packagesources/local"
	"github.com/jetstack/preflight/pkg/pathutils"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GlobalConfigDirectory is a static path where configuration
// may be loaded from. This is designed to support this
// executable in Docker containers.
const GlobalConfigDirectory = "/etc/preflight/"

// Config is the configuration for a Preflight check.
type Config struct {
	ClusterName     string
	PackageSources  []*packagesources.TypedConfig
	DataGatherers   map[string]datagatherer.Config
	Outputs         []*output.TypedConfig
	EnabledPackages []*EnabledPackage
}

// EnabledPackage is the configuarion for an enabled Preflight package.
type EnabledPackage struct {
	ID              string
	EnabledRuleIDs  []string `mapstructure:"enabled-rules"`
	DisabledRuleIDs []string `mapstructure:"disabled-rules"`
}

// LoadConfig loads a Preflight check configuration from a file at the path
// specified.
func LoadConfig(configPath string) (*Config, error) {
	// If a configPath has been specified use that, otherwise look in the
	// working directory and global config directory.
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		currentWorkingDirectory, err := os.Getwd()
		// Ignore any errors silently, but only search the
		// current working directory if we can resolve it.
		if err == nil {
			viper.AddConfigPath(currentWorkingDirectory)
		}
		viper.AddConfigPath(GlobalConfigDirectory)
		viper.SetConfigName("preflight")
	}

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		// Not having a configuration file is an usual case, so alert on it.
		log.Printf("Not using config file")
	}

	// Create config struct
	config := &Config{}

	// Decode cluster name
	config.ClusterName = viper.GetString("cluster-name")

	// Decode Preflight package sources config
	packageSourcesConfigFromFile, ok := viper.Get("package-sources").([]interface{})
	if !ok {
		return nil, errors.New("No package sources provided")
	}
	for idx, packageSourceConfigFromFile := range packageSourcesConfigFromFile {
		cfg, ok := packageSourceConfigFromFile.(map[interface{}]interface{})
		if !ok {
			return nil, fmt.Errorf("Cannot parse configuration from package source #%d", idx)
		}

		// TODO: we need to do this to keep b/c with existing config files
		// for cmd/check, pkg/packagesources is designed to work with
		// another type of configuration (cmd/agent).
		t, ok := cfg["type"].(string)
		if !ok {
			return nil, fmt.Errorf("Cannot read 'type' in package source #%d", idx)
		}
		dir, ok := cfg["dir"].(string)
		if !ok {
			return nil, fmt.Errorf("Cannot read 'dir' in package source #%d", idx)
		}

		if t != "local" {
			return nil, fmt.Errorf("Unsupported package source, type %q is unknown", t)
		}
		parsedCfg := &packagesources.TypedConfig{
			Type: "local",
			Config: &local.Config{
				Dir: dir,
			},
		}
		config.PackageSources = append(config.PackageSources, parsedCfg)
	}

	// Decode data gatherer config
	config.DataGatherers = map[string]datagatherer.Config{}
	dataGatherersConfigFromFile, ok := viper.Get("data-gatherers").(map[string]interface{})
	// we don't error if no data-gatherers to keep backwards compatibility
	if ok {
		for name, dataGathererConfigFromFile := range dataGatherersConfigFromFile {
			// TODO: create gatherer from config in a more clever way.
			// We need to read gatherer config from here and its schema depends on the gatherer itself.
			dataGathererConfigMap, ok := dataGathererConfigFromFile.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("Cannot parse %s data gatherer config.", name)
			}
			var dataGathererConfig datagatherer.Config
			// Check if this data gatherer's config specifies a data-path.
			// If it does create a LocalDataGatherer to load this data but keep
			// the name of the data gatherer it is impersonating so it can
			// provide stubbed data.
			if dataPath, ok := dataGathererConfigMap["data-path"].(string); ok && dataPath != "" {
				dataGathererConfig = &localdatagatherer.Config{
					DataPath: dataPath,
				}
			} else if name == "eks" {
				clusterName, _ := dataGathererConfigMap["cluster"].(string)
				dataGathererConfig = &eks.Config{
					ClusterName: clusterName,
				}
			} else if name == "gke" {
				project, _ := dataGathererConfigMap["project"].(string)
				zone, _ := dataGathererConfigMap["zone"].(string)
				location, _ := dataGathererConfigMap["location"].(string)
				cluster, _ := dataGathererConfigMap["cluster"].(string)
				credentialsPath, _ := dataGathererConfigMap["credentials"].(string)
				dataGathererConfig = &gke.Config{
					Cluster: &gke.Cluster{
						Project:  project,
						Zone:     zone,
						Location: location,
						Name:     cluster,
					},
					CredentialsPath: credentialsPath,
				}
			} else if name == "aks" {
				clusterName, _ := dataGathererConfigMap["cluster"].(string)
				resourceGroup, _ := dataGathererConfigMap["resource-group"].(string)
				credentialsPath, _ := dataGathererConfigMap["credentials"].(string)
				dataGathererConfig = &aks.Config{
					ClusterName:     clusterName,
					ResourceGroup:   resourceGroup,
					CredentialsPath: credentialsPath,
				}
			} else if name == "k8s/pods" {
				kubeconfigPath, ok := dataGathererConfigMap["kubeconfig"].(string)
				if !ok {
					log.Println("Didn't find 'kubeconfig' in 'data-gatherers.k8s/pods' configuration. Assuming it runs in-cluster.")
				}
				excludedNamespaces := []string{}
				excludedNamespacesFromFile, ok := dataGathererConfigMap["exclude-namespaces"].([]interface{})
				if ok {
					for _, excludedNamespaceFromFile := range excludedNamespacesFromFile {
						excludedNamespace := excludedNamespaceFromFile.(string)
						if ok {
							excludedNamespaces = append(excludedNamespaces, excludedNamespace)
						}
					}
				}
				dataGathererConfig = &k8s.Config{
					KubeConfigPath: pathutils.ExpandHome(kubeconfigPath),
					GroupVersionResource: schema.GroupVersionResource{
						Group:    "",
						Version:  "v1",
						Resource: "pods",
					},
					ExcludeNamespaces: excludedNamespaces,
				}
			} else if strings.HasPrefix(name, "k8s/") {
				trimmed := strings.TrimPrefix(name, "k8s/")
				nameOnDots := strings.SplitN(trimmed, ".", 3)
				// if a user has used for example `k8s/pods.v1`, we should
				// handle this case and set the group name to empty.
				if len(nameOnDots) == 2 {
					nameOnDots = append(nameOnDots, "")
				}
				if len(nameOnDots) != 3 {
					return nil, fmt.Errorf("Failed to parse generic k8s plugin configuration. Expected data gatherer name of the form k8s/{resource-name}.{api-version}.{api-group}")
				}
				kubeconfigPath, ok := dataGathererConfigMap["kubeconfig"].(string)
				if !ok {
					log.Printf("Didn't find 'kubeconfig' in 'data-gatherers.%s' configuration. Assuming it runs in-cluster.", name)
				}
				dataGathererConfig = &k8s.Config{
					KubeConfigPath: pathutils.ExpandHome(kubeconfigPath),
					GroupVersionResource: schema.GroupVersionResource{
						Resource: nameOnDots[0],
						Version:  nameOnDots[1],
						Group:    nameOnDots[2],
					},
				}
			} else if name == "local" {
				dataPath, ok := dataGathererConfigMap["data-path"].(string)
				if !ok {
					return nil, fmt.Errorf("Didn't find 'data-path' in 'data-gatherers.%s' configuration", name)
				}
				dataGathererConfig = &localdatagatherer.Config{
					DataPath: dataPath,
				}
			} else {
				return nil, fmt.Errorf("Found unsupported data-gatherer %q in config", name)
			}
			config.DataGatherers[name] = dataGathererConfig
		}
	}

	// Decode output config
	outputsConfigFromFile, ok := viper.Get("outputs").([]interface{})
	if !ok {
		return nil, fmt.Errorf("No outputs provided")
	}
	for _, outputConfigFromFile := range outputsConfigFromFile {
		outputConfigMap := outputConfigFromFile.(map[interface{}]interface{})
		outputType := outputConfigMap["type"].(string)
		var outputConfig output.Config
		if outputType == "cli" {
			var outputFormat string
			// Format is optional for CLI, will be defaulted to CLI format
			if outputConfigMap["format"] != nil {
				outputFormat = outputConfigMap["format"].(string)
			} else {
				outputFormat = ""
			}
			outputConfig = &cli.Config{
				Format: outputFormat,
			}
		} else if outputType == "local" {
			outputFormat, ok := outputConfigMap["format"].(string)
			if !ok {
				return nil, errors.New("Missing 'format' property in local output configuration")
			}
			outputPath, ok := outputConfigMap["path"].(string)
			if !ok {
				return nil, errors.New("Missing 'path' property in local output configuration")
			}
			outputConfig = &localoutput.Config{
				Format: outputFormat,
				Path:   pathutils.ExpandHome(outputPath),
			}
		} else if outputType == "gcs" {
			outputFormat, ok := outputConfigMap["format"].(string)
			if !ok {
				return nil, errors.New("Missing 'format' property in gcs output configuration")
			}
			outputBucketName, ok := outputConfigMap["bucket-name"].(string)
			if !ok {
				return nil, errors.New("Missing 'bucket-name' property in gcs output configuration")
			}
			outputCredentialsPath, ok := outputConfigMap["credentials-path"].(string)
			if !ok {
				return nil, errors.New("Missing 'credentials-path' property in gcs output configuration")
			}
			outputConfig = &gcs.Config{
				Format:          outputFormat,
				BucketName:      outputBucketName,
				CredentialsPath: outputCredentialsPath,
			}
		} else if outputType == "azblob" {
			outputFormat, ok := outputConfigMap["format"].(string)
			if !ok {
				return nil, errors.New("Missing 'format' property in azblob output configuration")
			}
			outputContainer, ok := outputConfigMap["container"].(string)
			if !ok {
				return nil, errors.New("Missing 'container' property in azblob output configuration")
			}
			accountName, accountKey := os.Getenv("AZURE_STORAGE_ACCOUNT"), os.Getenv("AZURE_STORAGE_ACCESS_KEY")
			if len(accountName) == 0 || len(accountKey) == 0 {
				return nil, fmt.Errorf("Either the AZURE_STORAGE_ACCOUNT or AZURE_STORAGE_ACCESS_KEY environment variable is not set")
			}
			outputConfig = &azblob.Config{
				Format:        outputFormat,
				ContainerName: outputContainer,
				AccountName:   accountName,
				AccountKey:    accountKey,
			}
		} else {
			return nil, fmt.Errorf("Output type not recognised: %s", outputType)
		}
		config.Outputs = append(config.Outputs, &output.TypedConfig{
			Type:   outputType,
			Config: outputConfig,
		})
	}

	// Decode enabled packages
	err := viper.UnmarshalKey("enabled-packages", &config.EnabledPackages)
	if err != nil {
		log.Printf("unable to decode into struct, %v", err)
		log.Print("using legacy enabled-packages format")
		// The failed UnmarshalKey creates an EnabledPackage in the slice,
		// so we recreate the slice here to make sure it's empty.
		config.EnabledPackages = []*EnabledPackage{}
		enabledPackageIDs := viper.GetStringSlice("enabled-packages")
		for _, enabledPackageID := range enabledPackageIDs {
			config.EnabledPackages = append(config.EnabledPackages, &EnabledPackage{
				ID: enabledPackageID,
			})
		}
	}
	if len(config.EnabledPackages) == 0 {
		return nil, errors.New("No packages were enabled. Use 'enables-packages' option in configuration to enable the packages you want to use")
	}

	return config, nil
}
