package cmd

import (
	"github.com/spf13/cobra"
)

var verbose bool

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the version",
	Long: `Display preflight version.
`,
	Run: func(cmd *cobra.Command, args []string) {
		printVersion(verbose)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.PersistentFlags().BoolVar(
		&verbose,
		"verbose",
		false,
		"If enabled, displays the additional information about this build.",
	)
}
