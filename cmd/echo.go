package cmd

import (
	"github.com/jetstack/preflight/pkg/echo"
	"github.com/spf13/cobra"
)

var echoCmd = &cobra.Command{
	Use:   "echo",
	Short: "starts an echo server to test the agent",
	Long: `The agent sends data to a server. This echo server
can be used to act as the server part and echo the data received by the agent.`,
	Run: echo.Echo,
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
	echoCmd.PersistentFlags().StringVarP(
		&echo.AllowedToken,
		"allowed-token",
		"t",
		"",
		"If provided, will only authorize requests authenticated with the token.",
	)
	echoCmd.PersistentFlags().BoolVarP(
		&echo.Compact,
		"compact",
		"",
		false,
		"Prints compact output.",
	)
}
