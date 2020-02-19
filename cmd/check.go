package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/internal/checkconfig"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/output"
	"github.com/jetstack/preflight/pkg/output/cli"
	"github.com/jetstack/preflight/pkg/packagesources"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/reports"
	"github.com/jetstack/preflight/pkg/results"
)

// The path of the config file to load
var configPath string

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Execute Preflight checks",
	Long: `Preflight check will execute all configured Preflight packages. This will
, depending on configuration, usually preform read access against a remote Kuberentes cluster
and/or cloud provider API.

This command will never modify external resources, and is safe to run idempotently.`,
	Run: func(cmd *cobra.Command, args []string) {
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
		fmt.Sprintf("Config file location, without this flag we search for `preflight.yaml` in the current working directory and '%s'.", checkconfig.GlobalConfigDirectory))
}

func check() {
	ctx := context.Background()

	// Load config from file
	config, err := checkconfig.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error loading check config: %+v", err)
	}

	checkTime := time.Now()

	// Load Preflight package sources
	var packageSources []packagesources.PackageSource
	for idx, packageSourceConfig := range config.PackageSources {
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

	// Load data gatherers
	dataGatherers := make(map[string]datagatherer.DataGatherer)
	for name, config := range config.DataGatherers {
		dg, err := config.NewDataGatherer(ctx)
		if err != nil {
			log.Fatalf("Cannot instantiate %s datagatherer: %v", name, err)
		}
		dataGatherers[name] = dg
	}

	// Fetch from all datagatherers
	information := make(map[string]interface{})
	for k, g := range dataGatherers {
		i, err := g.Fetch()
		if err != nil {
			log.Fatalf("Error fetching with DataGatherer %q: %s", k, err)
		}
		information[k] = i
	}

	// Load Outputs
	var outputs = make([]output.Output, 0)
	for _, outputConfig := range config.Outputs {
		output, err := outputConfig.NewOutput(ctx)
		if err != nil {
			log.Fatalf("Could not create %s output: %s", outputConfig.Type, err)
		}
		outputs = append(outputs, output)
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

	missingRules := false
	packageReports := []api.Report{}
	for _, enabledPackage := range config.EnabledPackages {
		// Make sure we loaded the package for this.
		pkg := packages[enabledPackage.ID]
		if pkg == nil {
			log.Fatalf("Package with ID %q was specified in configuration but it wasn't found.", enabledPackage.ID)
		}

		manifest := pkg.PolicyManifest
		// Make sure we loaded the DataGatherers.
		for _, g := range manifest.DataGatherers {
			if dataGatherers[g] == nil {
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
		report.Cluster = config.ClusterName
		report.Timestamp = api.Time{Time: checkTime}
		packageReports = append(packageReports, report)

		for _, output := range outputs {
			err := output.Write(ctx, manifest, intermediateBytes, rc, config.ClusterName, checkTime)
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
		err := output.WriteIndex(ctx, config.ClusterName, checkTime, &clusterSummary)
		if err != nil {
			log.Fatalf("failed to output index: %s", err)
		}
	}

	log.Printf("Done.")
}
