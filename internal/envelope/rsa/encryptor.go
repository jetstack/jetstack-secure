package rsa

import (
	"context"
	"fmt"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwe"

	"github.com/jetstack/preflight/internal/envelope"
	"github.com/jetstack/preflight/internal/envelope/keyfetch"
)

const (
	// EncryptionType is the type identifier for RSA JWE encryption
	EncryptionType = "JWE-RSA"
)

// Compile-time check that Encryptor implements envelope.Encryptor
var _ envelope.Encryptor = (*Encryptor)(nil)

// Encryptor provides envelope encryption using RSA-OAEP-256 for key wrapping
// and AES-256-GCM for data encryption, outputting JWE Compact Serialization format.
type Encryptor struct {
	fetcher keyfetch.KeyFetcher
}

// NewEncryptor creates a new Encryptor with the provided key fetcher.
// The encryptor will use RSA-OAEP-256 for key encryption and A256GCM for content encryption.
func NewEncryptor(fetcher keyfetch.KeyFetcher) (*Encryptor, error) {
	return &Encryptor{
		fetcher: fetcher,
	}, nil
}

// Encrypt performs envelope encryption on the provided data.
// It returns an EncryptedData struct containing JWE Compact Serialization format and type metadata.
// The JWE uses RSA-OAEP-256 for key encryption and A256GCM for content encryption.
func (e *Encryptor) Encrypt(ctx context.Context, data []byte) (*envelope.EncryptedData, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data to encrypt cannot be empty")
	}

	key, err := e.fetcher.FetchKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch encryption key: %w", err)
	}

	// Create headers with the key ID
	headers := jwe.NewHeaders()
	if err := headers.Set("kid", key.KeyID); err != nil {
		return nil, fmt.Errorf("failed to set key ID header: %w", err)
	}

	// Encrypt using RSA-OAEP-256 for key algorithm and A256GCM for content encryption
	// TODO: in go1.26+, consider using secret.Do to wrap this call, since it will generate an AES key
	encrypted, err := jwe.Encrypt(
		data,
		jwe.WithKey(jwa.RSA_OAEP_256(), key.Key, jwe.WithPerRecipientHeaders(headers)),
		jwe.WithContentEncryption(jwa.A256GCM()),
		jwe.WithCompact(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt data: %w", err)
	}

	return &envelope.EncryptedData{
		Data: encrypted,
		Type: EncryptionType,
	}, nil
}
