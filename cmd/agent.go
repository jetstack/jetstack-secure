package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/jetstack/preflight/pkg/agent"
	"github.com/jetstack/preflight/pkg/permissions"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "start the preflight agent",
	Long: `The agent will periodically gather data for the configured data
	gatherers and send it to a remote backend for evaluation`,
	Run: agent.Run,
}

var agentInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "print several internal parameters of the agent",
	Long:  `Print several internal parameters of the agent, as the built-in OAuth2 client ID.`,
	Run: func(cmd *cobra.Command, args []string) {
		printVersion(true)
		fmt.Println()
		printOAuth2Config()
	},
}

var agentRBACCmd = &cobra.Command{
	Use:   "rbac",
	Short: "print the agent's minimal RBAC manifest",
	Long:  `Print RBAC string by reading GVRs`,
	Run: func(cmd *cobra.Command, args []string) {

		b, err := ioutil.ReadFile(agent.ConfigFilePath)
		if err != nil {
			log.Fatalf("Failed to read config file: %s", err)
		}
		config, err := agent.ParseConfig(b, false)
		if err != nil {
			log.Fatalf("Failed to parse config file: %s", err)
		}

		out := permissions.GenerateFullManifest(config.DataGatherers)
		fmt.Print(out)
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentInfoCmd)
	agentCmd.AddCommand(agentRBACCmd)
	agentCmd.PersistentFlags().StringVarP(
		&agent.ConfigFilePath,
		"agent-config-file",
		"c",
		"./agent.yaml",
		"Config file location, default is `agent.yaml` in the current working directory.",
	)
	agentCmd.PersistentFlags().DurationVarP(
		&agent.Period,
		"period",
		"p",
		0,
		"Override time between scans in the configuration file (given as XhYmZs).",
	)
	agentCmd.PersistentFlags().StringVarP(
		&agent.CredentialsPath,
		"credentials-file",
		"k",
		"",
		"Location of the credentials file. For OAuth2 based authentication.",
	)
	agentCmd.PersistentFlags().BoolVarP(
		&agent.VenafiCloudMode,
		"venafi-cloud",
		"",
		false,
		"Runs agent with parsing config and credentials file in Venafi Cloud format if true.",
	)
	agentCmd.PersistentFlags().BoolVarP(
		&agent.OneShot,
		"one-shot",
		"",
		false,
		"Runs agent a single time if true, or continously if false",
	)
	agentCmd.PersistentFlags().StringVarP(
		&agent.OutputPath,
		"output-path",
		"",
		"",
		"Output file path, if used, it will write data to a local file instead of uploading to the preflight server",
	)
	agentCmd.PersistentFlags().StringVarP(
		&agent.InputPath,
		"input-path",
		"",
		"",
		"Input file path, if used, it will read data from a local file instead of gathering data from clusters",
	)
	agentCmd.PersistentFlags().DurationVarP(
		&agent.BackoffMaxTime,
		"backoff-max-time",
		"",
		10*time.Minute,
		"Max time for retrying failed data gatherers (given as XhYmZs).",
	)
	agentCmd.PersistentFlags().BoolVarP(
		&agent.StrictMode,
		"strict",
		"",
		false,
		"Runs agent in strict mode. No retry attempts will be made for a missing data gatherer's data.",
	)
	agentCmd.PersistentFlags().StringVar(
		&agent.APIToken,
		"api-token",
		os.Getenv("API_TOKEN"),
		"Token used for authentication when API tokens are in use on the backend",
	)
	agentCmd.PersistentFlags().BoolVarP(
		&agent.Profiling,
		"enable-pprof",
		"",
		false,
		"Enables the pprof profiling server on the agent (port: 6060).",
	)
	agentCmd.PersistentFlags().BoolVarP(
		&agent.Prometheus,
		"enable-metrics",
		"",
		false,
		"Enables Prometheus metrics server on the agent (port: 8081).",
	)
}
