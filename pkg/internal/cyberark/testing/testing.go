package testing

import (
	"os"
	"testing"
)

// SkipIfNoEnv skips the test if the required CyberArk environment variables are not set.
func SkipIfNoEnv(t testing.TB) {
	t.Helper()

	if os.Getenv("ARK_SUBDOMAIN") == "" ||
		os.Getenv("ARK_USERNAME") == "" ||
		os.Getenv("ARK_SECRET") == "" {
		t.Skip("Skipping test because one of ARK_SUBDOMAIN, ARK_USERNAME or ARK_SECRET isn't set")
	}

}
