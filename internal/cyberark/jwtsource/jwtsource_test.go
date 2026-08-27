// internal/cyberark/jwtsource/jwtsource_test.go
package jwtsource

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileSource_ReadsToken(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "jwt")
	require.NoError(t, os.WriteFile(p, []byte("the-jwt\n"), 0o600))
	got, err := NewFileSource(p).Read(t.Context())
	require.NoError(t, err)
	require.Equal(t, "the-jwt", got) // trimmed
}

func TestFileSource_MissingFile(t *testing.T) {
	_, err := NewFileSource("/no/such/file").Read(t.Context())
	require.Error(t, err)
}

func TestFileSource_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "jwt")
	require.NoError(t, os.WriteFile(p, []byte("  \n"), 0o600))
	_, err := NewFileSource(p).Read(t.Context())
	require.Error(t, err)
}
