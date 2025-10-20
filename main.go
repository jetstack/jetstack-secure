package main

import "github.com/jetstack/preflight/cmd"

func main() {
	startCoverageServer()
	cmd.Execute()
}
