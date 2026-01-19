package hpke

import (
	"fmt"

	"filippo.io/hpke"
)

// NewDecryptor creates a new Decryptor with the provided HPKE private key.
// It uses the default algorithms: X25519 KEM, HKDF-SHA256, and AES-256-GCM.
func NewDecryptor(privateKey hpke.PrivateKey) (*Decryptor, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("HPKE private key cannot be nil")
	}

	return &Decryptor{
		privateKey: privateKey,
		kdf:        DefaultKDF(),
		aead:       DefaultAEAD(),
	}, nil
}

// NewDecryptorWithAlgorithms creates a new Decryptor with custom algorithms.
// The KDF and AEAD must match those used during encryption.
// Note: The KEM is determined by the private key itself.
func NewDecryptorWithAlgorithms(privateKey hpke.PrivateKey, kdf hpke.KDF, aead hpke.AEAD) (*Decryptor, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("HPKE private key cannot be nil")
	}

	if kdf == nil {
		return nil, fmt.Errorf("KDF cannot be nil")
	}

	if aead == nil {
		return nil, fmt.Errorf("AEAD cannot be nil")
	}

	return &Decryptor{
		privateKey: privateKey,
		kdf:        kdf,
		aead:       aead,
	}, nil
}

// Decrypt decrypts the provided EncryptedData using HPKE.
func (d *Decryptor) Decrypt(encrypted *EncryptedData) ([]byte, error) {
	if encrypted == nil {
		return nil, fmt.Errorf("encrypted data cannot be nil")
	}

	if len(encrypted.EncapsulatedKey) == 0 {
		return nil, fmt.Errorf("encapsulated key cannot be empty")
	}

	if len(encrypted.Ciphertext) == 0 {
		return nil, fmt.Errorf("ciphertext cannot be empty")
	}

	// NewRecipient creates a receiver context from the encapsulated key.
	// The info parameter must match what was used during encryption (nil in our case).
	recipient, err := hpke.NewRecipient(encrypted.EncapsulatedKey, d.privateKey, d.kdf, d.aead, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HPKE recipient: %w", err)
	}

	// Open decrypts and authenticates the ciphertext.
	plaintext, err := recipient.Open(nil, encrypted.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to open HPKE ciphertext: %w", err)
	}

	return plaintext, nil
}

// DecryptWithInfo decrypts data that was encrypted with application-specific context information.
// The info parameter must match what was used during encryption.
func (d *Decryptor) DecryptWithInfo(encrypted *EncryptedData, info []byte) ([]byte, error) {
	if encrypted == nil {
		return nil, fmt.Errorf("encrypted data cannot be nil")
	}

	if len(encrypted.EncapsulatedKey) == 0 {
		return nil, fmt.Errorf("encapsulated key cannot be empty")
	}

	if len(encrypted.Ciphertext) == 0 {
		return nil, fmt.Errorf("ciphertext cannot be empty")
	}

	recipient, err := hpke.NewRecipient(encrypted.EncapsulatedKey, d.privateKey, d.kdf, d.aead, info)
	if err != nil {
		return nil, fmt.Errorf("failed to create HPKE recipient: %w", err)
	}

	plaintext, err := recipient.Open(nil, encrypted.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to open HPKE ciphertext: %w", err)
	}

	return plaintext, nil
}

// DecryptWithAAD decrypts data that was encrypted with additional authenticated data.
// The aad parameter must match what was used during encryption.
func (d *Decryptor) DecryptWithAAD(encrypted *EncryptedData, aad []byte) ([]byte, error) {
	if encrypted == nil {
		return nil, fmt.Errorf("encrypted data cannot be nil")
	}

	if len(encrypted.EncapsulatedKey) == 0 {
		return nil, fmt.Errorf("encapsulated key cannot be empty")
	}

	if len(encrypted.Ciphertext) == 0 {
		return nil, fmt.Errorf("ciphertext cannot be empty")
	}

	recipient, err := hpke.NewRecipient(encrypted.EncapsulatedKey, d.privateKey, d.kdf, d.aead, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HPKE recipient: %w", err)
	}

	plaintext, err := recipient.Open(aad, encrypted.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to open HPKE ciphertext: %w", err)
	}

	return plaintext, nil
}
