package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/pkg/logs"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Kubernetes cluster configuration checker 🚀",
	Long: `Preflight is a tool to automatically perform Kubernetes cluster
configuration checks using Open Policy Agent (OPA).

Preflight checks are bundled into Packages`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return logs.Initialize()
	},
	// SilenceErrors and SilenceUsage prevents this command or any sub-command
	// from printing arbitrary text to stderr.
	// Why? To ensure that each line of output can be parsed as a single message
	// for consumption by logging agents such as fluentd.
	// Usage information is still available on stdout with the `-h` and `--help`
	// flags.
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	for _, command := range rootCmd.Commands() {
		setFlagsFromEnv("PREFLIGHT_", command.PersistentFlags())
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
// If the root command or sub-command returns an error, the error message will
// be logged and the process will exit with status 1.
func Execute() {
	logs.AddFlags(rootCmd.PersistentFlags())
	ctx := klog.NewContext(context.Background(), klog.Background())
	var exitCode int
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		exitCode = 1
		klog.ErrorS(err, "Exiting due to error", "exit-code", exitCode)
	}
	klog.FlushAndExit(klog.ExitFlushTimeout, exitCode)
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
		name := fmt.Sprintf("%s_%s", cleanPrefix, strings.ReplaceAll(strings.ToUpper(f.Name), "-", "_"))
		if e, ok := os.LookupEnv(name); ok {
			_ = f.Value.Set(e)
		}
	})
}
