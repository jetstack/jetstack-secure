package cyberark_test

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jetstack/preflight/internal/cyberark"
)

// The subdomain is not a credential, so it is settable from the agent YAML
// config as well as the ARK_SUBDOMAIN environment variable that predates it.
// These tests pin the precedence rule in LoadClientConfig:
//   - config only  → config
//   - env only     → env (backward compatible with Secret-provided installs)
//   - both set     → config wins, with a warning logged
//   - neither      → ErrMissingSubdomain
func TestLoadClientConfig_SubdomainPrecedence(t *testing.T) {
	t.Run("config only -> config", func(t *testing.T) {
		t.Setenv("ARK_SUBDOMAIN", "")

		cfg, err := cyberark.LoadClientConfig(logr.Discard(), "from-config")
		require.NoError(t, err)
		assert.Equal(t, "from-config", cfg.Subdomain)
	})

	t.Run("env only -> env", func(t *testing.T) {
		t.Setenv("ARK_SUBDOMAIN", "from-env")

		cfg, err := cyberark.LoadClientConfig(logr.Discard(), "")
		require.NoError(t, err)
		assert.Equal(t, "from-env", cfg.Subdomain)
	})

	t.Run("both set -> config wins, with a warning logged", func(t *testing.T) {
		t.Setenv("ARK_SUBDOMAIN", "from-env")

		sink := &capturingSink{}
		cfg, err := cyberark.LoadClientConfig(logr.New(sink), "from-config")
		require.NoError(t, err)
		assert.Equal(t, "from-config", cfg.Subdomain)
		assert.Len(t, sink.messages, 1)
		if len(sink.messages) == 1 {
			assert.Contains(t, sink.messages[0], "both config.cyberark.subdomain and ARK_SUBDOMAIN are set")
		}
	})

	t.Run("neither set -> ErrMissingSubdomain", func(t *testing.T) {
		t.Setenv("ARK_SUBDOMAIN", "")

		_, err := cyberark.LoadClientConfig(logr.Discard(), "")
		require.ErrorIs(t, err, cyberark.ErrMissingSubdomain)
	})
}

// capturingSink is a minimal logr.LogSink that records Info() messages, used
// to assert the dual-source warning actually fires rather than just trusting
// the code path was reached.
type capturingSink struct {
	messages []string
}

func (s *capturingSink) Init(logr.RuntimeInfo)       {}
func (s *capturingSink) Enabled(int) bool            { return true }
func (s *capturingSink) Error(error, string, ...any) {}

func (s *capturingSink) Info(_ int, msg string, _ ...any) {
	s.messages = append(s.messages, msg)
}

func (s *capturingSink) WithValues(...any) logr.LogSink { return s }
func (s *capturingSink) WithName(string) logr.LogSink   { return s }
