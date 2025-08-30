package testutil

import (
	"testing"

	"github.com/jetstack/preflight/internal/cyberark"
)

// ArkSkipIfNoEnv skips the test if the required CyberArk environment variables are not set.
func ArkSkipIfNoEnv(t testing.TB) {
	t.Helper()
	if _, err := cyberark.LoadClientConfigFromEnvironment(); err != nil {
		t.Skipf("skipping test as CyberArk environment variables are not set: %v", err)
	}
}
