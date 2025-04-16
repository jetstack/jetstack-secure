package version

import (
	"fmt"
	"net/http"
)

// This variables are injected at build time.

// PreflightVersion hosts the version of the app.
var PreflightVersion = "development"

// Commit is the commit hash of the build
var Commit string

// BuildDate is the date it was built
var BuildDate string

// GoVersion is the go version that was used to compile this
var GoVersion string

// UserAgent return a standard user agent for use with all HTTP requests. This is implemented in one place so
// it's uniform across the Kubernetes Agent.
func UserAgent() string {
	return fmt.Sprintf("venafi-kubernetes-agent/%s", PreflightVersion)
}

// SetUserAgent augments an http.Request with a standard user agent.
func SetUserAgent(req *http.Request) {
	req.Header.Set("User-Agent", UserAgent())
}
