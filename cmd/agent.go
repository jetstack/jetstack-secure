package cmd

import (
	"github.com/jetstack/preflight/pkg/agent"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "start the preflight agent",
	Long: `The agent will periodically gather data for the configured data
	gatherers and send it to a remote backend for evaluation`,
	Run: agent.Run,
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.PersistentFlags().StringVarP(
		&agent.ConfigFilePath,
		"agent-config-file",
		"c",
		"./agent.yaml",
		"Config file location, default is `agent.yaml` in the current working directory.",
	)
	agentCmd.PersistentFlags().StringVarP(
		&agent.AuthToken,
		"auth-token",
		"t",
		"",
		"Authorization token. If used, it will override the authorization token in the configuration file.",
	)
	agentCmd.PersistentFlags().UintVarP(
		&agent.Period,
		"period",
		"p",
		3600,
		"Time between scans, in seconds.",
	)
	agentCmd.PersistentFlags().StringVarP(
		&agent.CredentialsPath,
		"credentials-file",
		"cr",
		"",
		"(Experimental) Location of the credentials file. For OAuth2 based authentication.",
	)
}
