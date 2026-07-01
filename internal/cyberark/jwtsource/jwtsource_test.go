// internal/cyberark/jwtsource/jwtsource_test.go
package jwtsource

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileSource_ReadsToken(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "jwt")
	require.NoError(t, os.WriteFile(p, []byte("the-jwt\n"), 0o600))
	got, err := NewFileSource(p).Read(context.Background())
	require.NoError(t, err)
	require.Equal(t, "the-jwt", got) // trimmed
}

func TestFileSource_MissingFile(t *testing.T) {
	_, err := NewFileSource("/no/such/file").Read(context.Background())
	require.Error(t, err)
}

func TestFileSource_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "jwt")
	require.NoError(t, os.WriteFile(p, []byte("  \n"), 0o600))
	_, err := NewFileSource(p).Read(context.Background())
	require.Error(t, err)
}
