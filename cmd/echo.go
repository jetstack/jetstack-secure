package cmd

import (
	"github.com/spf13/cobra"

	"github.com/jetstack/preflight/pkg/echo"
)

var echoCmd = &cobra.Command{
	Use:   "echo",
	Short: "starts an echo server to test the agent",
	Long: `The agent sends data to a server. This echo server
can be used to act as the server part and echo the data received by the agent.`,
	RunE: echo.Echo,
}

func init() {
	rootCmd.AddCommand(echoCmd)
	echoCmd.PersistentFlags().StringVarP(
		&echo.EchoListen,
		"listen",
		"l",
		":8080",
		"Address where to listen.",
	)

	echoCmd.PersistentFlags().BoolVarP(
		&echo.Compact,
		"compact",
		"",
		false,
		"Prints compact output.",
	)
}
