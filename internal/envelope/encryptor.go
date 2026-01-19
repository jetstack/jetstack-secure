package envelope

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
)

const (
	// aesKeySize is the size of the AES-256 key in bytes; aes.NewCipher generates cipher.Block based
	// on the size of key passed in
	aesKeySize = 32

	// nonceSize is the size of the AES-GCM nonce in bytes. NB: Nonce sizes can be security critical.
	// Reusing a nonce with the same key breaks AES-256 GCM completely.
	// Due to the birthday paradox, the risk of reusing (randomly-generated) nonces can be quite high.
	// This package is assumed to be used in contexts where a new key is generated for each encryption operation,
	// so the nonce size doesn't matter.
	nonceSize = 12

	// minRSAKeySize is the minimum RSA key size in bits; we'd expect that keys will be larger but 2048 is a sane floor
	minRSAKeySize = 2048
)

// NewEncryptor creates a new Encryptor with the provided RSA public key.
// The RSA key must be at least minRSAKeySize bits
func NewEncryptor(publicKey *rsa.PublicKey) (*Encryptor, error) {
	if publicKey == nil {
		return nil, fmt.Errorf("RSA public key cannot be nil")
	}

	// Validate key size
	keySize := publicKey.N.BitLen()
	if keySize < minRSAKeySize {
		return nil, fmt.Errorf("RSA key size must be at least %d bits, got %d bits", minRSAKeySize, keySize)
	}

	return &Encryptor{
		rsaPublicKey: publicKey,
	}, nil
}

// Encrypt performs envelope encryption on the provided data.
// It generates a random AES-256 key, encrypts the data with AES-256-GCM,
// then encrypts the AES key with RSA-OAEP-SHA256.
func (e *Encryptor) Encrypt(data []byte) (*EncryptedData, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data to encrypt cannot be empty")
	}

	aesKey := make([]byte, aesKeySize)
	if _, err := rand.Read(aesKey); err != nil {
		return nil, fmt.Errorf("failed to generate AES key: %w", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	encryptedData := &EncryptedData{
		EncryptedKey:  nil,
		EncryptedData: nil,
		Nonce:         make([]byte, nonceSize),
	}

	// Generate random nonce
	if _, err := rand.Read(encryptedData.Nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	encryptedData.EncryptedData = gcm.Seal(nil, encryptedData.Nonce, data, nil)

	// Encrypt AES key with RSA-OAEP-SHA256
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
