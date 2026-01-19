package envelope

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// This file contains helpers for loading keys. In practice we'll retrieve keys in some format from a DisCo endpoint

// LoadPublicKeyFromPEM parses an RSA public key from PEM-encoded bytes.
// The PEM block should be of type "PUBLIC KEY" or "RSA PUBLIC KEY".
func LoadPublicKeyFromPEM(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try parsing as PKIX public key first (most common format)
	if block.Type == "PUBLIC KEY" {
		pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKIX public key: %w", err)
		}

		rsaKey, ok := pubKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("key is not an RSA public key, got %T", pubKey)
		}

		return rsaKey, nil
	}

	// Try parsing as PKCS1 RSA public key
	if block.Type == "RSA PUBLIC KEY" {
		rsaKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS1 RSA public key: %w", err)
		}

		return rsaKey, nil
	}

	return nil, fmt.Errorf("unsupported PEM block type: %s (expected PUBLIC KEY or RSA PUBLIC KEY)", block.Type)
}

// LoadPublicKeyFromPEMFile reads and parses an RSA public key from a PEM file.
func LoadPublicKeyFromPEMFile(path string) (*rsa.PublicKey, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PEM file: %w", err)
	}

	return LoadPublicKeyFromPEM(pemBytes)
}
