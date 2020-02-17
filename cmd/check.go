package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime/schema"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/jetstack/preflight/api"
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
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/pathutils"
	"github.com/jetstack/preflight/pkg/reports"
	"github.com/jetstack/preflight/pkg/results"
)

var configPath string

// GlobalConfigDirectory is a static path where configuration
// may be loaded from. This is designed to support this
// executable in Docker containers.
const globalConfigDirectory = "/etc/preflight/"

type Config struct {
	PackageSources []packagesources.PackageSource
}

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
		&configPath,
		"config-file",
		"c",
		"",
		fmt.Sprintf("Config file location, without this flag we search for `preflight.yaml` in the current working directory and '%s'.", globalConfigDirectory))
}

func loadConfigFile() {
	if configPath != "" {
		viper.SetConfigFile(configPath)
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

	// Decode Preflight package sources config
	var packageSourcesConfig []*packagesources.TypedConfig
	packageSourcesConfigFromFile, ok := viper.Get("package-sources").([]interface{})
	if !ok {
		log.Fatalf("No package sources provided")
	}
	for idx, packageSourceConfigFromFile := range packageSourcesConfigFromFile {
		cfg, ok := packageSourceConfigFromFile.(map[interface{}]interface{})
		if !ok {
			log.Fatalf("Cannot parse configuration from package source #%d", idx)
		}

		// TODO: we need to do this to keep b/c with existing config files
		// for cmd/check, pkg/packagesources is designed to work with
		// another type of configuration (cmd/agent).
		t, ok := cfg["type"].(string)
		if !ok {
			log.Fatalf("Cannot read 'type' in package source #%d", idx)
		}
		dir, ok := cfg["dir"].(string)
		if !ok {
			log.Fatalf("Cannot read 'dir' in package source #%d", idx)
		}

		if t != "local" {
			log.Fatalf("Unsupported package source, type %q is unknown.", t)
		}
		parsedCfg := &packagesources.TypedConfig{
			Type: "local",
			Config: &local.Config{
				Dir: dir,
			},
		}
		packageSourcesConfig = append(packageSourcesConfig, parsedCfg)
	}

	// Load Preflight package sources
	var packageSources []packagesources.PackageSource
	for idx, packageSourceConfig := range packageSourcesConfig {
		pkgSrc, err := packageSourceConfig.NewPackageSource()
		if err != nil {
			log.Fatalf("Failed to instantiate package source #%d: %+v", idx, err)
		}
		packageSources = append(packageSources, pkgSrc)
	}

	// Load Preflight Packages
	var packages = make(map[string]*packaging.Package)
	for idx, pkgSrc := range packageSources {
		pkgs, err := pkgSrc.Load()
		if err != nil {
			log.Fatalf("Failed to load packages with package source #%d: %+v", idx, err)
		}

		for _, pkg := range pkgs {
			packages[pkg.PolicyManifest.GlobalID()] = pkg
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
			var err error
			dataGathererConfig, ok := config.(map[string]interface{})
			if !ok {
				log.Fatalf("Cannot parse %s data gatherer config.", name)
			}
			// Check if this data gatherer's config specifies a data-path.
			// If it does create a LocalDataGatherer to load this data but keep
			// the name of the data gatherer it is impersonating so it can
			// provide stubbed data.
			if dataPath, ok := dataGathererConfig["data-path"].(string); ok && dataPath != "" {
				dg, err = (&localdatagatherer.Config{DataPath: dataPath}).NewDataGatherer(ctx)
				if err != nil {
					log.Fatalf("Cannot instantiate Local datagatherer impersonating %s: %v", name, err)
				}
				gatherers[name] = dg
				continue
			}
			if name == "eks" {
				eksConfig, ok := config.(map[string]interface{})
				if !ok {
					log.Fatal("Cannot parse 'data-gatherers.eks' in config.")
				}

				clusterName, _ := eksConfig["cluster"].(string)
				dg, err = (&eks.Config{
					ClusterName: clusterName,
				}).NewDataGatherer(ctx)
				if err != nil {
					log.Fatalf("Cannot instantiate EKS datagatherer: %v", err)
				}
			} else if name == "gke" {
				gkeConfig, ok := config.(map[string]interface{})
				if !ok {
					log.Fatal("Cannot parse 'data-gatherers.gke' in config.")
				}

				project, _ := gkeConfig["project"].(string)
				zone, _ := gkeConfig["zone"].(string)
				location, _ := gkeConfig["location"].(string)
				cluster, _ := gkeConfig["cluster"].(string)
				credentialsPath, _ := gkeConfig["credentials"].(string)

				dg, err = (&gke.Config{
					Cluster: &gke.Cluster{
						Project:  project,
						Zone:     zone,
						Location: location,
						Name:     cluster,
					},
					CredentialsPath: credentialsPath,
				}).NewDataGatherer(ctx)
				if err != nil {
					log.Fatalf("Cannot instantiate GKE datagatherer: %v", err)
				}
			} else if name == "aks" {
				aksConfig, ok := config.(map[string]interface{})
				if !ok {
					log.Fatal("Cannot parse 'data-gatherers.aks' in config.")
				}

				clusterName, _ := aksConfig["cluster"].(string)
				resourceGroup, _ := aksConfig["resource-group"].(string)
				credentialsPath, _ := aksConfig["credentials"].(string)
				var err error
				dg, err = (&aks.Config{
					ClusterName:     clusterName,
					ResourceGroup:   resourceGroup,
					CredentialsPath: credentialsPath,
				}).NewDataGatherer(ctx)
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
				dg, err = (&k8s.Config{
					KubeConfigPath: pathutils.ExpandHome(kubeconfigPath),
					GroupVersionResource: schema.GroupVersionResource{
						Group:    "",
						Version:  "v1",
						Resource: "pods",
					},
				}).NewDataGatherer(ctx)
				if err != nil {
					log.Fatalf("Cannot create k8s client: %+v", err)
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
					log.Fatal("Failed to parse generic k8s plugin configuration. Expected data gatherer name of the form k8s/{resource-name}.{api-version}.{api-group}")
				}
				config, ok := config.(map[string]interface{})
				if !ok {
					log.Fatalf("cannot parse 'data-gatherers.%s' in config.", name)
				}
				kubeconfigPath, ok := config["kubeconfig"].(string)
				if !ok {
					log.Printf("Didn't find 'kubeconfig' in 'data-gatherers.%s' configuration. Assuming it runs in-cluster.", name)
				}
				dg, err = (&k8s.Config{
					KubeConfigPath: pathutils.ExpandHome(kubeconfigPath),
					GroupVersionResource: schema.GroupVersionResource{
						Resource: nameOnDots[0],
						Version:  nameOnDots[1],
						Group:    nameOnDots[2],
					},
				}).NewDataGatherer(ctx)
				if err != nil {
					log.Fatalf("Cannot create k8s client: %+v", err)
				}
			} else if name == "local" {
				localConfig, ok := config.(map[string]interface{})
				if !ok {
					log.Fatal("Cannot parse 'data-gatherers.local' in config.")
				}
				dataPath, ok := localConfig["data-path"].(string)
				dg, err = (&localdatagatherer.Config{DataPath: dataPath}).NewDataGatherer(ctx)
				if err != nil {
					log.Fatalf("Cannot instantiate Local datagatherer: %v", err)
				}
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
			op, err = (&cli.Config{
				Format: outputFormat,
			}).NewOutput(ctx)
		} else if outputType == "local" {
			outputFormat, ok := outputDefinition["format"].(string)
			if !ok {
				log.Fatal("Missing 'format' property in local output configuration.")
			}
			outputPath, ok := outputDefinition["path"].(string)
			if !ok {
				log.Fatal("Missing 'path' property in local output configuration.")
			}
			op, err = (&localoutput.Config{
				Format: outputFormat,
				Path:   pathutils.ExpandHome(outputPath),
			}).NewOutput(ctx)
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
			op, err = (&gcs.Config{
				Format:          outputFormat,
				BucketName:      outputBucketName,
				CredentialsPath: outputCredentialsPath,
			}).NewOutput(ctx)
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
			op, err = (&azblob.Config{
				Format:        outputFormat,
				ContainerName: outputContainer,
				AccountName:   accountName,
				AccountKey:    accountKey,
			}).NewOutput(ctx)
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
		op, err := (&cli.Config{}).NewOutput(ctx)
		if err != nil {
			log.Fatalf("Could not create cli output: %s", err)
		}
		outputs = append(outputs, op)
	}

	type EnabledPackage struct {
		ID              string
		EnabledRuleIDs  []string `mapstructure:"enabled-rules"`
		DisabledRuleIDs []string `mapstructure:"disabled-rules"`
	}
	var enabledPackages []EnabledPackage
	err := viper.UnmarshalKey("enabled-packages", &enabledPackages)
	if err != nil {
		log.Printf("unable to decode into struct, %v", err)
		log.Print("using legacy enabled-packages format")
		// The failed UnmarshalKey creates an EnabledPackage in the slice,
		// so we recreate the slice here to make sure it's empty.
		enabledPackages = []EnabledPackage{}
		enabledPackageIDs := viper.GetStringSlice("enabled-packages")
		for _, enabledPackageID := range enabledPackageIDs {
			enabledPackages = append(enabledPackages, EnabledPackage{ID: enabledPackageID})
		}
	}
	if len(enabledPackages) == 0 {
		log.Fatal("No packages were enabled. Use 'enables-packages' option in configuration to enable the packages you want to use.")
	}

	missingRules := false
	packageReports := []api.Report{}
	for _, enabledPackage := range enabledPackages {
		// Make sure we loaded the package for this.
		pkg := packages[enabledPackage.ID]
		if pkg == nil {
			log.Fatalf("Package with ID %q was specified in configuration but it wasn't found.", enabledPackage.ID)
		}

		manifest := pkg.PolicyManifest
		// Make sure we loaded the DataGatherers.
		for _, g := range manifest.DataGatherers {
			if gatherers[g] == nil {
				log.Fatalf("Package with ID %q requires DataGatherer %q, but it is not configured.", pkg.PolicyManifest.ID, g)
			}
		}

		// Extract the exact information needed for this package.
		input := make(map[string]interface{})
		for _, dg := range manifest.DataGatherers {
			input[dg] = information[dg]
		}

		rc, err := packaging.EvalPackage(ctx, pkg, input)
		if err != nil {
			if _, ok := err.(*reports.MissingRegoDefinitionError); ok {
				missingRules = true
				log.Printf("%+v", err)
			} else {
				log.Fatalf("Cannot evaluate package %q: %v", manifest.ID, err)
			}
		}

		rc = results.FilterResultCollection(rc, enabledPackage.DisabledRuleIDs, enabledPackage.EnabledRuleIDs)

		intermediateBytes, err := json.Marshal(input)
		if err != nil {
			log.Fatalf("Cannot marshal intermediate result: %v", err)
		}

		// build a report to build the updated context for the report index
		report, err := reports.NewReport(manifest, rc)
		if err != nil {
			log.Fatalf("Cannot generate report for results: %v", err)
		}
		// at the moment, we need to add Cluster and Timestamp here because of #1.
		report.Cluster = clusterName
		report.Timestamp = api.Time{Time: checkTime}
		packageReports = append(packageReports, report)

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

	clusterSummary, err := reports.NewClusterSummary(packageReports)
	if err != nil {
		log.Fatalf("Cannot generate index of reports: %v", err)
	}
	for _, output := range outputs {
		err := output.WriteIndex(ctx, clusterName, checkTime, &clusterSummary)
		if err != nil {
			log.Fatalf("failed to output index: %s", err)
		}
	}

	log.Printf("Done.")
}
