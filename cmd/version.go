package cmd

import (
	"fmt"

	"github.com/jetstack/preflight/pkg/version"

	"github.com/spf13/cobra"
)

var verbose bool

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the version",
	Long: `Display preflight version.
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Preflight version: ", version.PreflightVersion, version.Platform)
		if verbose {
			fmt.Println()
			fmt.Println("Commit: ", version.Commit)
			fmt.Println("Built: ", version.BuildDate)
			fmt.Println("Go: ", version.GoVersion)
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
