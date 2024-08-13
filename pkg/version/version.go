package version

// This variables are injected at build time.

// PreflightVersion hosts the version of the app.
var PreflightVersion = "development"

// Commit is the commit hash of the build
var Commit string

// BuildDate is the date it was built
var BuildDate string

// GoVersion is the go version that was used to compile this
var GoVersion string
