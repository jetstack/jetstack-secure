package cmd

import (
	"github.com/spf13/cobra"
)

var packageCmd = &cobra.Command{
	Use:   "package",
	Short: "Tools for managing Preflight packages",
	Long: `A Package is a bundling of Open Policy Agent (OPA)
rules and a policy manifest file in YAML format.

The package command provides tools and utilites for working with
packages, and are primarally intended for development use.`,
}

func init() {
	rootCmd.AddCommand(packageCmd)
}
