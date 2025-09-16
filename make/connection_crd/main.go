package main

import (
	"fmt"

	crd "github.com/jetstack/venafi-connection-lib/config/crd/bases"
)

// With this tool, we no longer have to use something like `helm template` to
// pull the CRD manifest from the venafi-connection-lib project.
func main() {
	fmt.Print(string(crd.VenafiConnectionCrd))
}
