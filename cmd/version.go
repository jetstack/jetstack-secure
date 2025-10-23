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
	versionCmd.PersistentFlags().BoolVarP(
		&verbose,
		"verbose",
		"V", // a lowercase v was causing a panic: unable to redefine 'v' shorthand in "version" flagset: it's already used for "verbose" flag
		false,
		"If enabled, displays the additional information about this built.",
	)
}
