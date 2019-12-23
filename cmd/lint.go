package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jetstack/preflight/pkg/lint"
	"github.com/jetstack/preflight/pkg/packagesources/local"
	"github.com/jetstack/preflight/pkg/packaging"

	"github.com/open-policy-agent/opa/ast"

	"gopkg.in/yaml.v2"
)

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Lint a Preflight package for errors",
	Long: `The linter is designed for use in development and
continious integration environments.

The linter will check for common issues with packages:
- That a correctly structured policy-manifest.yaml file is present
- That at least one .rego file is present with OPA rules
- That at least one .rego test file is present
- That the rego files use the correct module name`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			// Fail if given no input
			log.Fatal("No packages provided for linting")
		} else {
			// Run lint on each path and collect errors
			lintErrors := make([]lint.LintError, 0)
			for _, packagePath := range args {
				errors := LintPackage(packagePath)
				lintErrors = append(lintErrors, errors...)
			}
			for _, le := range lintErrors {
				// If verbose mode was enabled, print out full errors
				// where available.
				if le.Err == nil || !viper.GetBool("verbose") {
					log.Printf("Lint: %s - %s", le.PackagePath, le.Lint)
				} else {
					log.Printf("Lint: %s - %s: %s", le.PackagePath, le.Lint, le.Err)
				}
			}
			// Return a nonzero exit code if there were any lint errors at all
			if len(lintErrors) > 0 {
				log.Fatal("Encountered lint errors")
			} else {
				log.Printf("All packages passed linting :)")
			}
		}
	},
}

// LintPackage performs linting checks on a Preflight
// package on-disk. Returning a list of lint issues as
// strings.
func LintPackage(packagePath string) []lint.LintError {
	log.Printf("Linting package %s", packagePath)
	lints := make([]lint.LintError, 0)

	addLint := func(l string) {
		lints = append(lints, lint.LintError{PackagePath: packagePath, Lint: l})
	}
	addLintE := func(l string, e error) {
		lints = append(lints, lint.LintError{PackagePath: packagePath, Lint: l, Err: e})
	}

	// Try to open this directory
	fi, err := os.Stat(packagePath)
	if err != nil {
		addLintE("Unable to read package path", err)
		return lints
	}
	if !fi.IsDir() {
		addLint("Package path is not a directory")
		return lints
	}

	// Try to read the manifest
	manifestPath := filepath.Join(packagePath, "policy-manifest.yaml")
	fi, err = os.Stat(manifestPath)
	if err != nil {
		addLintE("Unable to read manifest path", err)
		return lints
	}
	manifestBytes, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		addLintE("Unable to open manifest for reading", err)
		return lints
	}

	var manifest packaging.PolicyManifest
	err = yaml.Unmarshal(manifestBytes, &manifest)
	if err != nil {
		addLintE("Unable to parse manifest YAML", err)
		return lints
	}

	manifestLints := lint.LintPolicyManifest(manifest)
	for i, _ := range manifestLints {
		manifestLints[i].PackagePath = packagePath
	}
	lints = append(lints, manifestLints...)

	// Look for rego and rego test files
	files, err := ioutil.ReadDir(packagePath)
	if err != nil {
		addLintE("Unable to read directory", err)
		return lints
	}

	policyFiles := make(map[string]string, 0)
	testFiles := make(map[string]string, 0)
	for _, fi := range files {
		path := filepath.Join(packagePath, fi.Name())
		if local.IsPolicyFile(fi) {
			policyBytes, err := ioutil.ReadFile(path)
			if err != nil {
				addLintE("Unable to read Open Policy Agent policy (`.rego`) file", err)
			} else {
				policyFiles[fi.Name()] = string(policyBytes)
			}
		} else if local.IsPolicyTestFile(fi) {
			testBytes, err := ioutil.ReadFile(path)
			if err != nil {
				addLintE("Unable to read Open Policy Agent policy test (`_test.rego`) file", err)
			} else {
				testFiles[fi.Name()] = string(testBytes)
			}
		}
	}
	if len(policyFiles) == 0 {
		addLint("Unable to find any Open Policy Agent policy (`.rego`) files")
	}
	if len(testFiles) == 0 {
		addLint("Unable to find any Open Policy Agent policy test (`_test.rego`) files")
	}

	// Parse the Rego files
	compiler, err := ast.CompileModules(policyFiles)
	if err != nil {
		addLintE("Unable to compile Open Policy Agent policy", err)
	} else {
		// I want what you'd call a "set" in other languages.
		rules := make(map[lint.RuleName]struct{}, 0)
		for moduleName, module := range compiler.Modules {
			log.Printf("Found module %s", moduleName)
			for _, rule := range module.Rules {
				ruleName, err := lint.NewRuleNameFromRego(string(rule.Head.Name))
				if err == nil {
					rules[*ruleName] = struct{}{}
				}
			}
		}
		// turn the map[string]struct{} into a []string of unique entries
		rulesNames := make([]lint.RuleName, len(rules))
		i := 0
		for ruleName, _ := range rules {
			rulesNames[i] = ruleName
			i++
		}
		// Get the ones from the manifest
		manifestRNs := lint.CollectManifestRuleNames(manifest)
		// We have two sets and want the symetric difference. (In mathmatical terms)
		// This method has terrible algorithmic complexity [O(n^2)], but n is at most 100 so who cares.
		// Look for manifest RNs not in Rego
		for _, mr := range manifestRNs {
			var found = false
			for _, rr := range rulesNames {
				if mr == rr {
					found = true
					break
				}
			}
			if !found {
				addLint(fmt.Sprintf("Rule %s declared in Manifest, but not present in rego code", mr))
			}
		}
		// Do the same in inverse, looking for rego RNs not in manifest
		for _, rr := range rulesNames {
			var found = false
			for _, mr := range manifestRNs {
				if mr == rr {
					found = true
					break
				}
			}
			if !found {
				addLint(fmt.Sprintf("Rule %s declared in rego code, but not present in Manifest", rr))
			}
		}
	}
	return lints
}

func init() {
	packageCmd.AddCommand(lintCmd)
	lintCmd.Flags().BoolP("verbose", "v", false, "Print full errors for any lint failures (if available)")
	viper.BindPFlag("verbose", lintCmd.Flags().Lookup("verbose"))
}
