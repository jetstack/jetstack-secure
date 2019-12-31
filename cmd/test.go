package cmd

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jetstack/preflight/pkg/packagesources/local"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test REGO inside a Preflight package",
	Long: `This uses OPA's engine to run all the testsuites
inside the package.

It only works with local packages.
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			// Fail if given no input
			log.Fatal("No packages provided for linting")
		} else {
			loadedPackages := make([]packaging.Package, 0)

			for _, packagePath := range args {
				pkgs, err := local.LoadLocalPackages(packagePath)
				if err != nil {
					log.Fatalf("Error loading packages from %s: %v", packagePath, err)
				}
				for _, p := range pkgs {
					loadedPackages = append(loadedPackages, p)
				}
			}

			packagesWithErrors := make(map[string][]int)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			for _, pkg := range loadedPackages {
				log.Printf("Testing package %s", pkg.PolicyManifest().GlobalID())
				numFailures, numTotal, err := packaging.TestPackage(ctx, pkg)

				if err != nil {
					log.Fatalf("Error testing package %s: %v", pkg.PolicyManifest().GlobalID(), err)
				}

				if numFailures != 0 {
					packagesWithErrors[pkg.PolicyManifest().GlobalID()] = []int{numFailures, numTotal}
				}
			}

			if len(packagesWithErrors) > 0 {
				log.Fatalf("Encountered failed tests in these packages: %s", func() (s string) {
					pkgs := make([]string, 0)
					for k, v := range packagesWithErrors {
						pkgs = append(pkgs, fmt.Sprintf("%s (%d failures in %d tests)", k, v[0], v[1]))
					}
					return strings.Join(pkgs, ", ")
				}())
			} else {
				log.Printf("All packages tests passed :)")
			}
		}
	},
}

func init() {
	packageCmd.AddCommand(testCmd)

	// testCmd.Flags().BoolP("verbose", "v", false, "set verbose reporting mode")
	// testCmd.Flags().BoolP("show-failure-line", "l", false, "show test failure line")
	// testCmd.Flags().DurationP("timeout", "t", time.Second*5, "set test timeout")
	// testCmd.Flags().BoolP("coverage", "c", false, "report coverage (overrides debug tracing)")
	// testCmd.Flags().Float64P("threshold", "", 0, "set coverage threshold and exit with non-zero status if coverage is less than threshold %")
	// testCmd.Flags().BoolP("bundle", "b", false, "load paths as bundle files or root directories")
}
