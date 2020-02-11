package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/exporter"
	"github.com/jetstack/preflight/pkg/output"
	"github.com/jetstack/preflight/pkg/packagesources"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/reports"
	"github.com/jetstack/preflight/pkg/results"

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

type PreflightCheckConfig struct {
	ClusterName     string                                `mapstructure:"cluster-name"`
	DataGatherers   *datagatherer.DataGatherersConfig     `mapstructure:"data-gatherers,omitempty"`
	PackageSources  []*packagesources.PackageSourceConfig `mapstructure:"package-sources,omitempty"`
	EnabledPackages []*EnabledPackage                     `mapstructure:"enabled-packages,omitempty"`
	Outputs         []*output.OutputConfig
}

type EnabledPackage struct {
	ID              string
	EnabledRuleIDs  []string `mapstructure:"enabled-rules"`
	DisabledRuleIDs []string `mapstructure:"disabled-rules"`
}

var config PreflightCheckConfig

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

	err := viper.Unmarshal(&config)
	if err != nil {
		log.Println("Unable to decode config:", err)
		log.Println("Will now decode with legacy config format")
		unmarshalLegacy()
	}

	// If any AZBlobOutputs are configured get the account name and key from
	// environment variables
	accountName := os.Getenv("AZURE_STORAGE_ACCOUNT")
	accountKey := os.Getenv("AZURE_STORAGE_ACCESS_KEY")
	for _, outputConfig := range config.Outputs {
		outputConfig.AccountName = accountName
		outputConfig.AccountKey = accountKey
	}
}

func unmarshalLegacy() {
	var legacyConfig struct {
		ClusterName     string                                `mapstructure:"cluster-name"`
		DataGatherers   *datagatherer.DataGatherersConfig     `mapstructure:"data-gatherers,omitempty"`
		PackageSources  []*packagesources.PackageSourceConfig `mapstructure:"package-sources,omitempty"`
		EnabledPackages []string                              `mapstructure:"enabled-packages,omitempty"`
		Outputs         []*output.OutputConfig
	}
	err := viper.Unmarshal(&legacyConfig)
	if err != nil {
		log.Fatal("Unable to decode legacy config:", err)
	}
	enabledPackages := []*EnabledPackage{}
	for _, enabledPackageID := range legacyConfig.EnabledPackages {
		enabledPackage := &EnabledPackage{
			ID: enabledPackageID,
		}
		enabledPackages = append(enabledPackages, enabledPackage)
	}
	config = PreflightCheckConfig{
		ClusterName:     legacyConfig.ClusterName,
		DataGatherers:   legacyConfig.DataGatherers,
		PackageSources:  legacyConfig.PackageSources,
		Outputs:         legacyConfig.Outputs,
		EnabledPackages: enabledPackages,
	}
}

func check() {
	ctx := context.Background()

	checkTime := time.Now()
	clusterName := config.ClusterName

	packageSources := packagesources.NewPackageSources(ctx, config.PackageSources)
	dataGatherers := datagatherer.NewDataGatherers(ctx, config.DataGatherers)
	outputs := output.NewOutputs(ctx, config.Outputs)
	enabledPackages := config.EnabledPackages

	if len(enabledPackages) == 0 {
		log.Fatal("No packages were enabled. Use 'enabled-packages' in configuration to enable the packages you want to use.")
	}

	// If no outputs are specified add a default CLI output.
	if len(outputs) == 0 {
		log.Printf("No outputs specified, will default to CLI.")
		cliOutput, err := output.NewCLIOutput(ctx, &output.CLIOutputConfig{
			Format: exporter.FormatCLI,
		})
		if err != nil {
			log.Fatalf("Could not create cli output: %s", err)
		}
		outputs = append(outputs, cliOutput)
	}

	// Load Preflight packages from PackageSources
	packages := make([]*packaging.Package, 0)
	for _, packageSource := range packageSources {
		loadedPackages, err := packageSource.Load()
		if err != nil {
			log.Fatalf("%s", err)
		}
		packages = append(packages, loadedPackages...)
	}
	if len(packages) == 0 {
		log.Fatalf("No Preflight packages loaded from package sources.")
	}

	// Fetch data from data gatherers.
	data := make(map[string]interface{})
	for dataGathererType, dataGatherer := range dataGatherers {
		fetchedData, err := dataGatherer.Fetch()
		if err != nil {
			log.Fatalf("Error fetching data with %s data gatherer : %s", dataGathererType, err)
		}
		data[dataGathererType] = fetchedData
	}

	missingRules := false
	packageReports := []api.Report{}
	for _, enabledPackage := range enabledPackages {
		// Make sure we loaded the package for this.
		pkg := getPackageByID(packages, enabledPackage.ID)
		if pkg == nil {
			log.Fatalf("Package with ID %q was specified in configuration but it wasn't found.", enabledPackage.ID)
		}

		manifest := pkg.PolicyManifest
		// Make sure we loaded the DataGatherers.
		for _, dataGathererType := range manifest.DataGatherers {
			if dataGatherers[dataGathererType] == nil {
				log.Fatalf("Package with ID %q requires DataGatherer %q, but it is not configured.", pkg.PolicyManifest.ID, dataGathererType)
			}
		}

		// Collect the data required for this package
		inputData := make(map[string]interface{})
		for _, dataGathererType := range manifest.DataGatherers {
			inputData[dataGathererType] = data[dataGathererType]
		}

		rc, err := packaging.EvalPackage(ctx, pkg, inputData)
		if err != nil {
			if _, ok := err.(*reports.MissingRegoDefinitionError); ok {
				missingRules = true
				log.Printf("%+v", err)
			} else {
				log.Fatalf("Cannot evaluate package %q: %v", manifest.ID, err)
			}
		}

		rc = results.FilterResultCollection(rc, enabledPackage.DisabledRuleIDs, enabledPackage.EnabledRuleIDs)

		intermediateBytes, err := json.Marshal(inputData)
		if err != nil {
			log.Fatalf("Cannot marshal intermediate result: %v", err)
		}

		// Build a report to build the updated context for the report index
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

func getPackageByID(packages []*packaging.Package, packageID string) *packaging.Package {
	for _, pkg := range packages {
		if pkg.PolicyManifest.ID == packageID {
			return pkg
		}
	}
	return nil
}
