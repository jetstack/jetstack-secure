package cyberark_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jetstack/preflight/internal/cyberark"
)

// The subdomain is not a credential, so it is settable from the agent YAML
// config as well as the ARK_SUBDOMAIN environment variable that predates it.
// These tests pin the precedence rule in LoadClientConfigFromEnvironment:
//   - config only  → config
//   - env only     → env (backward compatible with Secret-provided installs)
//   - both set     → config wins
//   - neither      → ErrMissingSubdomain
func TestLoadClientConfigFromEnvironment_SubdomainPrecedence(t *testing.T) {
	t.Run("config only -> config", func(t *testing.T) {
		t.Setenv("ARK_SUBDOMAIN", "")

		cfg, err := cyberark.LoadClientConfigFromEnvironment("from-config")
		require.NoError(t, err)
		assert.Equal(t, "from-config", cfg.Subdomain)
	})

	t.Run("env only -> env", func(t *testing.T) {
		t.Setenv("ARK_SUBDOMAIN", "from-env")

		cfg, err := cyberark.LoadClientConfigFromEnvironment("")
		require.NoError(t, err)
		assert.Equal(t, "from-env", cfg.Subdomain)
	})

	t.Run("both set -> config wins", func(t *testing.T) {
		t.Setenv("ARK_SUBDOMAIN", "from-env")

		cfg, err := cyberark.LoadClientConfigFromEnvironment("from-config")
		require.NoError(t, err)
		assert.Equal(t, "from-config", cfg.Subdomain)
	})

	t.Run("neither set -> ErrMissingSubdomain", func(t *testing.T) {
		t.Setenv("ARK_SUBDOMAIN", "")

		_, err := cyberark.LoadClientConfigFromEnvironment("")
		require.ErrorIs(t, err, cyberark.ErrMissingSubdomain)
	})
}
