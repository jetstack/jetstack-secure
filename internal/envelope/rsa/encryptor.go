package rsa

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"

	"github.com/jetstack/preflight/internal/envelope"
)

const (
	// aesKeySize is the size of the AES-256 key in bytes; aes.NewCipher generates cipher.Block based
	// on the size of key passed in, and 32 bytes corresponds to a 256-bit AES key
	aesKeySize = 32

	// minRSAKeySize is the minimum RSA key size in bits; we'd expect that keys will be larger but 2048 is a sane floor
	// to enforce to ensure that a weak key can't accidentally be used
	minRSAKeySize = 2048

	// keyAlgorithmIdentifier is set in EncryptedData to identify the key wrapping algorithm used in this package
	keyAlgorithmIdentifier = "RSA-OAEP-SHA256"
)

// Compile-time check that Encryptor implements envelope.Encryptor
var _ envelope.Encryptor = (*Encryptor)(nil)

// Encryptor provides envelope encryption using RSA for key wrapping
// and AES-256-GCM for data encryption.
type Encryptor struct {
	keyID        string
	rsaPublicKey *rsa.PublicKey
}

// NewEncryptor creates a new Encryptor with the provided RSA public key.
// The RSA key must be at least minRSAKeySize bits
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
		keyID:        keyID,
		rsaPublicKey: publicKey,
	}, nil
}

// Encrypt performs envelope encryption on the provided data.
// It generates a random AES-256 key, encrypts the data with AES-256-GCM,
// then encrypts the AES key with RSA-OAEP-SHA256.
func (e *Encryptor) Encrypt(data []byte) (*envelope.EncryptedData, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data to encrypt cannot be empty")
	}

	aesKey := make([]byte, aesKeySize)
	if _, err := rand.Read(aesKey); err != nil {
		return nil, fmt.Errorf("failed to generate AES key: %w", err)
	}

	// zero the key from memory before the function returns
	// TODO: in go1.26+, consider using secret.Do in this function
	defer func() {
		for i := range aesKey {
			aesKey[i] = 0
		}
	}()

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	encryptedData := &envelope.EncryptedData{
		KeyID:         e.keyID,
		KeyAlgorithm:  keyAlgorithmIdentifier,
		EncryptedKey:  nil,
		EncryptedData: nil,
		Nonce:         make([]byte, gcm.NonceSize()),
	}

	// Generate a random nonce for AES-GCM.
	// Security: Nonces must never be re-used for a given key. Since we generate a new AES key for each encryption,
	// the risk of nonce reuse is not a concern here.
	if _, err := rand.Read(encryptedData.Nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal encrypts and authenticates the data. This could include additional authenticated data,
	// but we don't make use of that here.
	// First nil: allocate new slice for output.
	// Last nil: no additional authenticated data (AAD) needed.

	encryptedData.EncryptedData = gcm.Seal(nil, encryptedData.Nonce, data, nil)

	// Encrypt AES key with RSA-OAEP-SHA256. The nil parameter means no additional
	// context data is mixed into the hash; this could be used to disambiguate different uses of the same key,
	// but we only have one use for the key here.
	encryptedData.EncryptedKey, err = rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		e.rsaPublicKey,
		aesKey,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt AES key with RSA: %w", err)
	}

	return encryptedData, nil
}
