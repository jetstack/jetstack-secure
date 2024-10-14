package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/jetstack/preflight/pkg/logs"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Kubernetes cluster configuration checker 🚀",
	Long: `Preflight is a tool to automatically perform Kubernetes cluster
configuration checks using Open Policy Agent (OPA).

Preflight checks are bundled into Packages`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logs.Initialize()
	},
}

func init() {
	for _, command := range rootCmd.Commands() {
		setFlagsFromEnv("PREFLIGHT_", command.PersistentFlags())
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	logs.AddFlags(rootCmd.PersistentFlags())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func setFlagsFromEnv(prefix string, fs *pflag.FlagSet) {
	set := map[string]bool{}
	fs.Visit(func(f *pflag.Flag) {
		set[f.Name] = true
	})
	fs.VisitAll(func(f *pflag.Flag) {
		// ignore flags set from the commandline
		if set[f.Name] {
			return
		}
		// remove trailing _ to reduce common errors with the prefix, i.e. people setting it to MY_PROG_
		cleanPrefix := strings.TrimSuffix(prefix, "_")
		name := fmt.Sprintf("%s_%s", cleanPrefix, strings.Replace(strings.ToUpper(f.Name), "-", "_", -1))
		if e, ok := os.LookupEnv(name); ok {
			_ = f.Value.Set(e)
		}
	})
}
