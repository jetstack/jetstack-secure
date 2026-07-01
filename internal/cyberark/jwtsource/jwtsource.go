// internal/cyberark/jwtsource/jwtsource.go
package jwtsource

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// DefaultTokenPath is the default projected ServiceAccount token mount (aud=conjur).
const DefaultTokenPath = "/var/run/secrets/tokens/jwt"

// Source produces a raw JWT to exchange at SMS authn-jwt.
type Source interface {
	Read(ctx context.Context) (string, error)
}

type fileSource struct{ path string }

// NewFileSource reads a JWT from a file (the projected SA token).
func NewFileSource(path string) Source {
	if path == "" {
		path = DefaultTokenPath
	}
	return &fileSource{path: path}
}

func (f *fileSource) Read(_ context.Context) (string, error) {
	b, err := os.ReadFile(f.path)
	if err != nil {
		return "", fmt.Errorf("jwt source file %q not found or unreadable (is the projected serviceAccountToken volume mounted?): %w", f.path, err)
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return "", fmt.Errorf("jwt source file %q is empty", f.path)
	}
	return tok, nil
}
