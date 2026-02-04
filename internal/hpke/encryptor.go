package hpke

import (
	"crypto/ecdh"
	"fmt"

	"filippo.io/hpke"
)

// DefaultKEM returns the recommended KEM (Key Encapsulation Mechanism).
// Uses X25519-based Diffie-Hellman KEM.
func DefaultKEM() hpke.KEM {
	return hpke.DHKEM(ecdh.X25519())
}

// DefaultKDF returns the recommended KDF (Key Derivation Function).
// Uses HKDF with SHA-256.
func DefaultKDF() hpke.KDF {
	return hpke.HKDFSHA256()
}

// DefaultAEAD returns the recommended AEAD (Authenticated Encryption with Associated Data).
// Uses AES-256-GCM.
func DefaultAEAD() hpke.AEAD {
	return hpke.AES256GCM()
}

// NewEncryptor creates a new Encryptor with the provided HPKE public key.
// It uses the default algorithms: X25519 KEM, HKDF-SHA256, and AES-256-GCM.
func NewEncryptor(publicKey hpke.PublicKey) (*Encryptor, error) {
	if publicKey == nil {
		return nil, fmt.Errorf("HPKE public key cannot be nil")
	}

	return &Encryptor{
		publicKey: publicKey,
		kdf:       DefaultKDF(),
		aead:      DefaultAEAD(),
	}, nil
}

// NewEncryptorWithAlgorithms creates a new Encryptor with custom algorithms.
// Use this if you need to use different KDF or AEAD algorithms.
// Note: The KEM is determined by the public key itself.
func NewEncryptorWithAlgorithms(publicKey hpke.PublicKey, kdf hpke.KDF, aead hpke.AEAD) (*Encryptor, error) {
	if publicKey == nil {
		return nil, fmt.Errorf("HPKE public key cannot be nil")
	}

	if kdf == nil {
		return nil, fmt.Errorf("KDF cannot be nil")
	}

	if aead == nil {
		return nil, fmt.Errorf("AEAD cannot be nil")
	}

	return &Encryptor{
		publicKey: publicKey,
		kdf:       kdf,
		aead:      aead,
	}, nil
}

// Encrypt performs HPKE encryption on the provided data.
// HPKE combines key encapsulation and authenticated encryption in a single operation.
// It returns the encapsulated key and ciphertext that can be used for decryption.
func (e *Encryptor) Encrypt(data []byte) (*EncryptedData, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data to encrypt cannot be empty")
	}

	// NewSender creates a sender context and generates an encapsulated key.
	// The info parameter is application-specific context information (we use nil for now).
	encapsulatedKey, sender, err := hpke.NewSender(e.publicKey, e.kdf, e.aead, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HPKE sender: %w", err)
	}

	// Seal encrypts the plaintext with authenticated encryption.
	// The aad (additional authenticated data) parameter is optional (we use nil).
	ciphertext, err := sender.Seal(nil, data)
	if err != nil {
		return nil, fmt.Errorf("failed to seal data with HPKE: %w", err)
	}

	return &EncryptedData{
		EncapsulatedKey: encapsulatedKey,
		Ciphertext:      ciphertext,
	}, nil
}

// EncryptWithInfo performs HPKE encryption with application-specific context information.
// The info parameter is bound to the encryption operation and must be provided during decryption.
func (e *Encryptor) EncryptWithInfo(data []byte, info []byte) (*EncryptedData, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data to encrypt cannot be empty")
	}

	encapsulatedKey, sender, err := hpke.NewSender(e.publicKey, e.kdf, e.aead, info)
	if err != nil {
		return nil, fmt.Errorf("failed to create HPKE sender: %w", err)
	}

	ciphertext, err := sender.Seal(nil, data)
	if err != nil {
		return nil, fmt.Errorf("failed to seal data with HPKE: %w", err)
	}

	return &EncryptedData{
		EncapsulatedKey: encapsulatedKey,
		Ciphertext:      ciphertext,
	}, nil
}

// EncryptWithAAD performs HPKE encryption with additional authenticated data.
// The aad parameter is authenticated but not encrypted.
func (e *Encryptor) EncryptWithAAD(data []byte, aad []byte) (*EncryptedData, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data to encrypt cannot be empty")
	}

	encapsulatedKey, sender, err := hpke.NewSender(e.publicKey, e.kdf, e.aead, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HPKE sender: %w", err)
	}

	ciphertext, err := sender.Seal(aad, data)
	if err != nil {
		return nil, fmt.Errorf("failed to seal data with HPKE: %w", err)
	}

	return &EncryptedData{
		EncapsulatedKey: encapsulatedKey,
		Ciphertext:      ciphertext,
	}, nil
}

// GenerateKeyPair generates a new HPKE key pair using the default KEM (X25519).
// This is a helper function for testing and key generation.
func GenerateKeyPair() (hpke.PublicKey, hpke.PrivateKey, error) {
	kem := DefaultKEM()
	privateKey, err := kem.GenerateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate HPKE key pair: %w", err)
	}
	publicKey := privateKey.PublicKey()
	return publicKey, privateKey, nil
}
