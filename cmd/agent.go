package cmd

import (
	"fmt"
	"io/ioutil"

	"github.com/jetstack/preflight/pkg/agent"
	"github.com/jetstack/preflight/pkg/logs"
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

		b, err := ioutil.ReadFile(agent.Flags.ConfigFilePath)
		if err != nil {
			logs.Log.Fatalf("Failed to read config file: %s", err)
		}
		cfg, err := agent.ParseConfig(b, false)
		if err != nil {
			logs.Log.Fatalf("Failed to parse config file: %s", err)
		}

		out := permissions.GenerateFullManifest(cfg.DataGatherers)
		fmt.Print(out)
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentInfoCmd)
	agentCmd.AddCommand(agentRBACCmd)
	agent.InitAgentCmdFlags(agentCmd, &agent.Flags)
}
