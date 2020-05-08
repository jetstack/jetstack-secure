package cmd

import (
	"fmt"

	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/version"
)

func printVersion(verbose bool) {
	fmt.Println("Preflight version: ", version.PreflightVersion, version.Platform)
	if verbose {
		fmt.Println("  Commit: ", version.Commit)
		fmt.Println("  Built:  ", version.BuildDate)
		fmt.Println("  Go:     ", version.GoVersion)
	}
}

func printOAuth2Config() {
	fmt.Println("OAuth2: ")
	fmt.Println("  ClientID:         ", client.ClientID)
	fmt.Println("  AuthServerDomain: ", client.AuthServerDomain)
}
