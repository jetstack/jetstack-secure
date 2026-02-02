package rsa

import (
	"crypto/rsa"
	"fmt"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwe"

	"github.com/jetstack/preflight/internal/envelope"
)

const (
	// minRSAKeySize is the minimum RSA key size in bits; we'd expect that keys will be larger but 2048 is a sane floor
	// to enforce to ensure that a weak key can't accidentally be used
	minRSAKeySize = 2048

	// EncryptionType is the type identifier for RSA JWE encryption
	EncryptionType = "JWE-RSA"
)

// Compile-time check that Encryptor implements envelope.Encryptor
var _ envelope.Encryptor = (*Encryptor)(nil)

// Encryptor provides envelope encryption using RSA-OAEP-256 for key wrapping
// and AES-256-GCM for data encryption, outputting JWE Compact Serialization format.
type Encryptor struct {
	keyID     string
	publicKey *rsa.PublicKey
}

// NewEncryptor creates a new Encryptor with the provided RSA public key.
// The RSA key must be at least minRSAKeySize bits.
// The encryptor will use RSA-OAEP-256 for key encryption and A256GCM for content encryption.
func NewEncryptor(keyID string, publicKey *rsa.PublicKey) (*Encryptor, error) {
	if publicKey == nil {
		return nil, fmt.Errorf("RSA public key cannot be nil")
	}

	// Validate key size
	keySize := publicKey.N.BitLen()
	if keySize < minRSAKeySize {
		return nil, fmt.Errorf("RSA key size must be at least %d bits, got %d bits", minRSAKeySize, keySize)
	}

	if len(keyID) == 0 {
		return nil, fmt.Errorf("keyID cannot be empty")
	}

	return &Encryptor{
		keyID:     keyID,
		publicKey: publicKey,
	}, nil
}

// Encrypt performs envelope encryption on the provided data.
// It returns an EncryptedData struct containing JWE Compact Serialization format and type metadata.
// The JWE uses RSA-OAEP-256 for key encryption and A256GCM for content encryption.
func (e *Encryptor) Encrypt(data []byte) (*envelope.EncryptedData, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data to encrypt cannot be empty")
	}

	// Create headers with the key ID
	headers := jwe.NewHeaders()
	if err := headers.Set("kid", e.keyID); err != nil {
		return nil, fmt.Errorf("failed to set key ID header: %w", err)
	}

	// Encrypt using RSA-OAEP-256 for key algorithm and A256GCM for content encryption
	// TODO: in go1.26+, consider using secret.Do to wrap this call, since it will generate an AES key
	encrypted, err := jwe.Encrypt(
		data,
		jwe.WithKey(jwa.RSA_OAEP_256(), e.publicKey, jwe.WithPerRecipientHeaders(headers)),
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
