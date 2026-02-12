package rsa

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// This file contains helpers for loading keys. In practice we'll retrieve keys in some format from a DisCo endpoint

const (
	// HardcodedPublicKeyPEM contains a temporary hardcoded RSA public key (2048-bit) for envelope encryption.
	// This is a TEMPORARY solution for initial development and testing.
	// TODO: Replace with dynamic key fetching from CyberArk Discovery & Context API.
	HardcodedPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAoeq+dk4aoGdV9xjrnGJt
VbUh5jvkQgynkP+9Ph2NVeoasXWqYOmOVeKOI7Yr58W/L8Mro6C22iSEJrPFgPF6
t+RJsLAsAY6w1Pocq16COeelAWtxhHQGXt77WQKk0kmwhOJZ4VSeiQC4hWLUnq4N
Ft7lwLw/50opTXLuSErrwec/bEV7G/Xp11BMsHGEL7dzpwWAfIrbCEomyWrO/L6p
O3SAgYMdfup5ddnszeCU2FbFQziOkuMLOyir91XXk8wgdSy4IGAEGpwNx88i8fuj
Qafze2aGWUtpWlOEQPP8lH2cj2TGUgLxGITbczJRcwuGIoJBOzAmPDWi/bapj4b6
zQIDAQAB
-----END PUBLIC KEY-----`

	// hardcodedUID is a temporary hardcoded UID associated with the hardcoded public key
	// It was randomly generated with the macOS "uuidgen" command
	hardcodedUID = "A39798E6-8CE7-4E6E-9CF6-24A3C923B3A7"
)

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

// LoadHardcodedPublicKey loads and parses the hardcoded RSA public key.
// Returns a hardcoded UID associated with the key.
// This is a temporary solution for initial development and testing.
// Returns an error if the hardcoded key is invalid or cannot be parsed.
func LoadHardcodedPublicKey() (*rsa.PublicKey, string, error) {
	key, err := LoadPublicKeyFromPEM([]byte(HardcodedPublicKeyPEM))
	if err != nil {
		return nil, "", err
	}

	return key, hardcodedUID, nil
}
