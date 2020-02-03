package cmd

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jetstack/preflight/pkg/packagesources/local"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/spf13/cobra"
)

var testParams = struct {
	verbose bool
	timeout time.Duration
}{}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test Rego inside a Preflight package",
	Long: `This uses OPA's engine to run all the test suites
inside the package.

It only works with local packages.
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			// Fail if given no input
			log.Fatal("No packages provided for linting")
		} else {
			ctx, cancel := context.WithCancel(context.Background())
			packages := make([]*packaging.Package, 0)

			for _, packagePath := range args {
				packageSource, err := local.NewLocalPackageSource(ctx, &local.LocalPackageSourceConfig{
					Path: packagePath,
				})
				loadedPackages, err := packageSource.Load()
				if err != nil {
					log.Fatalf("Error loading packages from %s: %v", packagePath, err)
				}
				packages = append(packages, loadedPackages...)
			}

			packagesWithErrors := make(map[string][]int)

			defer cancel()

			for _, pkg := range packages {
				log.Printf("Testing package %s", pkg.PolicyManifest.GlobalID())
				numFailures, numTotal, err := packaging.TestPackage(ctx, pkg, testParams.verbose, testParams.timeout)

				if err != nil {
					log.Fatalf("Error testing package %s: %v", pkg.PolicyManifest.GlobalID(), err)
				}

				if numFailures != 0 {
					packagesWithErrors[pkg.PolicyManifest.GlobalID()] = []int{numFailures, numTotal}
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

	testCmd.Flags().BoolVarP(&testParams.verbose, "verbose", "v", false, "set verbose reporting mode")
	testCmd.Flags().DurationVarP(&testParams.timeout, "timeout", "t", time.Second*10, "set test timeout")
}
