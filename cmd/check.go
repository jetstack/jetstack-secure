package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/datagatherer/aks"
	"github.com/jetstack/preflight/pkg/datagatherer/eks"
	"github.com/jetstack/preflight/pkg/datagatherer/gke"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"github.com/jetstack/preflight/pkg/output"
	"github.com/jetstack/preflight/pkg/output/azblob"
	"github.com/jetstack/preflight/pkg/output/gcs"
	"github.com/jetstack/preflight/pkg/packagesources/local"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/reports"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var cfgFile string

// GlobalConfigDirectory is a static path where configuration
// may be loaded from. This is designed to support this
// executable in Docker containers.
const globalConfigDirectory = "/etc/preflight/"

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Execute Preflight checks",
	Long: `Preflight check will execute all configured Preflight packages. This will
, depending on configuration, usually preform read access against a remote Kuberentes cluster
and/or cloud provider API.

This command will never modify external resources, and is safe to run idempotently.`,
	Run: func(cmd *cobra.Command, args []string) {
		loadConfigFile()
		check()
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.PersistentFlags().StringVarP(
		&cfgFile,
		"config-file",
		"c",
		"",
		fmt.Sprintf("Config file location, without this flag we search for `preflight.yaml` in the current working directory and '%s'.", globalConfigDirectory))
}

func loadConfigFile() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		currentWorkingDirectory, err := os.Getwd()
		// Ignore any errors silently, but only search the
		// current working directory if we can resolve it.
		if err == nil {
			viper.AddConfigPath(currentWorkingDirectory)
		}
		viper.AddConfigPath(globalConfigDirectory)
		viper.SetConfigName("preflight")
	}

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		// Not having a configuration file is an usual case, so alert on it.
		log.Printf("Not using config file")
	}
}

func check() {
	ctx := context.Background()

	// Collect details about this run
	clusterName := viper.GetString("cluster-name")
	checkTime := time.Now()

	// Load Preflight Packages
	var packages = make(map[string]packaging.Package)

	packageSources, ok := viper.Get("package-sources").([]interface{})
	if !ok {
		log.Fatalf("No package sources provided")
	}
	for _, packageSource := range packageSources {
		ps := packageSource.(map[interface{}]interface{})
		sourceType := ps["type"].(string)
		// TODO Support source types that are not "local"
		// TODO Replace this awful if-else chain with something nicer
		if sourceType == "local" {
			dir := ps["dir"].(string)
			loadedPackages, err := local.LoadLocalPackages(dir)
			if err != nil {
				log.Fatalf("Failed to load package(s) from local source: %s", err)
			}
			for _, loadedPackage := range loadedPackages {
				packages[loadedPackage.PolicyManifest().GlobalID()] = loadedPackage
			}
		} else {
			log.Fatalf("Can't understand package source of type %s", sourceType)
		}
	}

	if len(packages) == 0 {
		log.Fatalf("No Packages loaded")
	}

	// Load datagatherers
	gatherers := make(map[string]datagatherer.DataGatherer)
	gatherersConfig, ok := viper.Get("data-gatherers").(map[string]interface{})
	// we don't error if no data-gatherers to keep backwards compatibility
	if ok {
		for name, config := range gatherersConfig {
			// TODO: create gatherer from config in a more clever way. We need to read gatherer config from here and its schema depends on the gatherer itself.
			var dg datagatherer.DataGatherer
			if name == "eks" {
				eksConfig, ok := config.(map[string]interface{})
				if !ok {
					log.Fatal("Cannot parse 'data-gatherers.eks' in config.")
				}
				if clusterName, ok := eksConfig["cluster"].(string); ok && clusterName != "" {
					dg = eks.NewEKSDataGatherer(clusterName)
				} else {
					log.Fatal("'data-gatherers.eks.cluster' should be a non empty string.")
				}
			} else if name == "gke" {
				gkeConfig, ok := config.(map[string]interface{})
				if !ok {
					log.Fatal("Cannot parse 'data-gatherers.gke' in config.")
				}
				var project, zone, cluster, location, credentialsPath string
				msg := "'data-gatherers.gke.%s' should be a non empty string."
				if project, ok = gkeConfig["project"].(string); !ok {
					log.Fatalf(msg, "project")
				}
				if zone, ok = gkeConfig["zone"].(string); ok {
					log.Println("'data-gatherers.gke.zone' is deprecated and will be deleted soon. Please use 'data-gatherers.gke.location' instead.")
				}
				if location, ok = gkeConfig["location"].(string); !ok {
					if len(zone) == 0 {
						log.Fatalf(msg, "location")
					}
				}
				if len(location) > 0 && len(zone) > 0 {
					log.Fatal("'data-gatherers.gke.zone' and 'data-gatherers.gke.location' cannot be used at the same time.")
				}
				if cluster, ok = gkeConfig["cluster"].(string); !ok {
					log.Fatalf(msg, "cluster")
				}
				// credentialsPath empty or not-present is also valid
				credentialsPath, _ = gkeConfig["credentials"].(string)
				dg = gke.NewGKEDataGatherer(ctx, &gke.Cluster{
					Project:  project,
					Zone:     zone,
					Location: location,
					Name:     cluster,
				}, credentialsPath)
			} else if name == "aks" {
				aksConfig, ok := config.(map[string]interface{})
				if !ok {
					log.Fatal("Cannot parse 'data-gatherers.aks' in config.")
				}
				msg := "'data-gatherers.aks.%s' should be a non empty string."
				var resourceGroup, clusterName, credentialsPath string
				if resourceGroup, ok = aksConfig["resource-group"].(string); !ok {
					log.Fatalf(msg, "resource-group")
				}
				if clusterName, ok = aksConfig["cluster"].(string); !ok {
					log.Fatalf(msg, "cluster")
				}
				if credentialsPath, ok = aksConfig["credentials"].(string); !ok {
					log.Fatalf(msg, "credentials")
				}
				var err error
				dg, err = aks.NewAKSDataGatherer(ctx, resourceGroup, clusterName, credentialsPath)
				if err != nil {
					log.Fatalf("Cannot instantiate AKS datagatherer: %v", err)
				}
			} else if name == "k8s/pods" {
				podsConfig, ok := config.(map[string]interface{})
				if !ok {
					log.Fatal("Cannot parse 'data-gatherers.k8s/pods' in config.")
				}
				kubeconfigPath, ok := podsConfig["kubeconfig"].(string)
				if !ok {
					log.Println("Didn't find 'kubeconfig' in 'data-gatherers.k8s/pods' configuration. Assuming it runs in-cluster.")
				}
				k8sClient, err := k8s.NewClient(expandHome(kubeconfigPath))
				if err != nil {
					log.Fatalf("Cannot create k8s client: %+v", err)
				}
				dg = k8s.NewPodsDataGatherer(k8sClient)
			} else {
				log.Fatalf("Found unsupported data-gatherer %q in config.", name)
			}
			gatherers[name] = dg
		}
	}

	// Fetch from all datagatherers
	information := make(map[string]interface{})
	for k, g := range gatherers {
		i, err := g.Fetch()
		if err != nil {
			log.Fatalf("Error fetching with DataGatherer %q: %s", k, err)
		}
		information[k] = i
	}

	// Load Output config
	var outputs = make([]output.Output, 0)
	outputDefinitions, ok := viper.Get("outputs").([]interface{})
	if !ok {
		log.Fatalf("No outputs provided")
	}
	for _, o := range outputDefinitions {
		outputDefinition := o.(map[interface{}]interface{})
		outputType := outputDefinition["type"].(string)
		var (
			op  output.Output
			err error
		)
		if outputType == "cli" {
			var outputFormat string
			// Format is optional for CLI, will be defaulted to CLI format
			if outputDefinition["format"] != nil {
				outputFormat = outputDefinition["format"].(string)
			} else {
				outputFormat = ""
			}
			op, err = output.NewCLIOutput(outputFormat)
		} else if outputType == "local" {
			outputFormat, ok := outputDefinition["format"].(string)
			if !ok {
				log.Fatal("Missing 'format' property in local output configuration.")
			}
			outputPath, ok := outputDefinition["path"].(string)
			if !ok {
				log.Fatal("Missing 'path' property in local output configuration.")
			}
			op, err = output.NewLocalOutput(outputFormat, expandHome(outputPath))
		} else if outputType == "gcs" {
			outputFormat, ok := outputDefinition["format"].(string)
			if !ok {
				log.Fatal("Missing 'format' property in gcs output configuration.")
			}
			outputBucketName, ok := outputDefinition["bucket-name"].(string)
			if !ok {
				log.Fatal("Missing 'bucket-name' property in gcs output configuration.")
			}
			outputCredentialsPath, ok := outputDefinition["credentials-path"].(string)
			if !ok {
				log.Fatal("Missing 'credentials-path' property in gcs output configuration.")
			}
			op, err = gcs.NewOutput(ctx, outputFormat, outputBucketName, outputCredentialsPath)
		} else if outputType == "azblob" {
			outputFormat, ok := outputDefinition["format"].(string)
			if !ok {
				log.Fatal("Missing 'format' property in azblob output configuration.")
			}
			outputContainer, ok := outputDefinition["container"].(string)
			if !ok {
				log.Fatal("Missing 'container' property in azblob output configuration.")
			}
			accountName, accountKey := os.Getenv("AZURE_STORAGE_ACCOUNT"), os.Getenv("AZURE_STORAGE_ACCESS_KEY")
			if len(accountName) == 0 || len(accountKey) == 0 {
				log.Fatal("Either the AZURE_STORAGE_ACCOUNT or AZURE_STORAGE_ACCESS_KEY environment variable is not set.")
			}
			op, err = azblob.NewOutput(ctx, outputFormat, outputContainer, accountName, accountKey)
		} else {
			log.Fatalf("Output type not recognised: %s", outputType)
		}
		if err != nil {
			log.Fatalf("Could not create %s output: %s", outputType, err)
		}
		outputs = append(outputs, op)
	}

	if len(outputs) == 0 {
		// Default to CLI output
		log.Printf("No outputs specified, will default to CLI")
		op, err := output.NewCLIOutput("")
		if err != nil {
			log.Fatalf("Could not create cli output: %s", err)
		}
		outputs = append(outputs, op)
	}

	type EnabledPackage struct {
		Name string
		// TODO: Add EnabledRules functionality later
		// EnabledRules  []string `mapstructure:"enabled-rules"`
		DisabledRules []string `mapstructure:"disabled-rules"`
	}
	var enabledPackages []EnabledPackage
	err := viper.UnmarshalKey("enabled-packages", &enabledPackages)
	if err != nil {
		log.Fatalf("unable to decode into struct, %v", err)
	}

	if len(enabledPackages) == 0 {
		log.Fatal("No packages were enabled. Use 'enables-packages' option in configuration to enable the packages you want to use.")
	}

	missingRules := false
	for _, enabledPackage := range enabledPackages {
		// Make sure we loaded the package for this.
		pkg := packages[enabledPackage.Name]
		if pkg == nil {
			log.Fatalf("Package with ID %q was specified in configuration but it wasn't found.", enabledPackage.Name)
		}

		manifest := pkg.PolicyManifest()
		// Make sure we loaded the DataGatherers.
		for _, g := range manifest.DataGatherers {
			if gatherers[g] == nil {
				log.Fatalf("Package with ID %q requires DataGatherer %q, but it is not configured.", pkg.PolicyManifest().ID, g)
			}
		}

		// Extract the exact information needed for this package.
		input := make(map[string]interface{})
		for _, dg := range manifest.DataGatherers {
			input[dg] = information[dg]
		}

		rc, err := packaging.EvalPackage(ctx, pkg, input, enabledPackage.DisabledRules)
		if err != nil {
			if _, ok := err.(*reports.MissingRegoDefinitionError); ok {
				missingRules = true
				log.Printf("%+v", err)
			} else {
				log.Fatalf("Cannot evaluate package %q: %v", manifest.ID, err)
			}
		}

		intermediateBytes, err := json.Marshal(input)
		if err != nil {
			log.Fatalf("Cannot marshal intermediate result: %v", err)
		}

		for _, output := range outputs {
			err := output.Write(ctx, manifest, intermediateBytes, rc, clusterName, checkTime)
			if err != nil {
				log.Fatalf("failed to output results: %s", err)
			}
		}
	}

	if missingRules {
		log.Fatalf("Some of the rules are missing their corresponding Rego definition. See the rest of the log or the reports to see more details.")
	} else {
		log.Printf("Done.")
	}
}

func homeDir() string {
	usr, err := user.Current()
	if err != nil {
		return ""
	}
	return usr.HomeDir
}

func expandHome(path string) string {
	if len(path) == 0 {
		return ""
	}

	if path[:2] == "~/" {
		return filepath.Join(homeDir(), path[2:])
	}
	return path
}
