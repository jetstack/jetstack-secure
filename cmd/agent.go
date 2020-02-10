package cmd

import (
	"log"
	"time"

	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "start the preflight agent",
	Long: `The agent will periodically gather data for the configured data
	gatherers and send it to a remote backend for evaluation`,
	Run: func(cmd *cobra.Command, args []string) {
		for {
			log.Printf("Running Agent... TODO")
			time.Sleep(10 * time.Second)
		}
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
}
