package testing

import (
	"os"
	"testing"
)

// SkipIfNoEnv skips the test if the required CyberArk environment variables are not set.
func SkipIfNoEnv(t testing.TB) {
	t.Helper()

	if os.Getenv("ARK_SUBDOMAIN") == "" {
		t.Skip("Skipping test because ARK_SUBDOMAIN isn't set")
	}
}
