package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// PreflightVersion hosts the version of the app. It is injected at build time.
var PreflightVersion = "development"

// Commit is the commit hash of the build
var Commit string

// BuildDate is the date it was built
var BuildDate string

// GoVersion is the go version that was used to compile this
var GoVersion string

// Platform is the target platform this was compiled for
var Platform string

var verbose bool

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the version",
	Long: `Display preflight version.
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Preflight version: ", PreflightVersion, Platform)
		if verbose {
			fmt.Println()
			fmt.Println("Commit: ", Commit)
			fmt.Println("Built: ", BuildDate)
			fmt.Println("Go: ", GoVersion)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.PersistentFlags().BoolVarP(
		&verbose,
		"verbose",
		"v",
		false,
		"If enabled, displays the additional information about this built.",
	)
}
