package rsa_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	internalrsa "github.com/jetstack/preflight/internal/envelope/rsa"
)

func generateTestKeyPEM(t *testing.T, keySize int, pemType string) []byte {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, keySize)
	require.NoError(t, err)

	var pemBytes []byte
	if pemType == "PUBLIC KEY" {
		// PKIX format
		publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
		require.NoError(t, err)

		pemBytes = pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: publicKeyBytes,
		})
	} else {
		// PKCS1 format
		publicKeyBytes := x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)

		pemBytes = pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: publicKeyBytes,
		})
	}

	require.NotNil(t, pemBytes)
	return pemBytes
}

func TestLoadPublicKeyFromPEM_PKIX(t *testing.T) {
	pemBytes := generateTestKeyPEM(t, 2048, "PUBLIC KEY")

	key, err := internalrsa.LoadPublicKeyFromPEM(pemBytes)
	require.NoError(t, err)
	require.NotNil(t, key)
	require.Equal(t, 2048, key.N.BitLen())
}

func TestLoadPublicKeyFromPEM_PKCS1(t *testing.T) {
	pemBytes := generateTestKeyPEM(t, 2048, "RSA PUBLIC KEY")

	key, err := internalrsa.LoadPublicKeyFromPEM(pemBytes)
	require.NoError(t, err)
	require.NotNil(t, key)
	require.Equal(t, 2048, key.N.BitLen())
}

func TestLoadPublicKeyFromPEM_InvalidPEM(t *testing.T) {
	invalidPEM := []byte("this is not a valid PEM")

	key, err := internalrsa.LoadPublicKeyFromPEM(invalidPEM)
	require.Error(t, err)
	require.Nil(t, key)
	require.Contains(t, err.Error(), "failed to decode PEM block")
}

func TestLoadPublicKeyFromPEM_WrongPEMType(t *testing.T) {
	// Create a PEM block with wrong type
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	key, err := internalrsa.LoadPublicKeyFromPEM(pemBytes)
	require.Error(t, err)
	require.Nil(t, key)
	require.Contains(t, err.Error(), "unsupported PEM block type")
}

func TestLoadPublicKeyFromPEM_NonRSAKey(t *testing.T) {
	// Generate a real ECDSA key and try to load it as RSA
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	// Marshal as PKIX public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&ecdsaKey.PublicKey)
	require.NoError(t, err)

	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	key, err := internalrsa.LoadPublicKeyFromPEM(pemBytes)
	require.Error(t, err)
	require.Nil(t, key)
	require.Contains(t, err.Error(), "not an RSA public key")
}

func TestLoadPublicKeyFromPEMFile_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test_key.pem")

	pemBytes := generateTestKeyPEM(t, 2048, "PUBLIC KEY")
	err := os.WriteFile(keyPath, pemBytes, 0600)
	require.NoError(t, err)

	key, err := internalrsa.LoadPublicKeyFromPEMFile(keyPath)
	require.NoError(t, err)
	require.NotNil(t, key)
	require.Equal(t, 2048, key.N.BitLen())
}

func TestLoadPublicKeyFromPEMFile_MissingFile(t *testing.T) {
	key, err := internalrsa.LoadPublicKeyFromPEMFile("/nonexistent/path/key.pem")
	require.Error(t, err)
	require.Nil(t, key)
	require.Contains(t, err.Error(), "failed to read PEM file")
}

func TestLoadPublicKeyFromPEMFile_InvalidContent(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "invalid_key.pem")

	err := os.WriteFile(keyPath, []byte("not a valid PEM"), 0600)
	require.NoError(t, err)

	key, err := internalrsa.LoadPublicKeyFromPEMFile(keyPath)
	require.Error(t, err)
	require.Nil(t, key)
}

func TestLoadHardcodedPublicKey_CanBeUsedWithEncryptor(t *testing.T) {
	// Test that the hardcoded key can be used to create an encryptor
	// First, test that the key can be loaded successfully
	key, uid, err := internalrsa.LoadHardcodedPublicKey()
	require.NoError(t, err)
	require.NotNil(t, key)
	require.NotEmpty(t, uid)

	encryptor, err := internalrsa.NewEncryptor(uid, key)
	require.NoError(t, err)
	require.NotNil(t, encryptor)

	// Test that the encryptor can encrypt data
	testData := []byte("test data for encryption")
	encryptedData, err := encryptor.Encrypt(testData)
	require.NoError(t, err)
	require.NotNil(t, encryptedData)
	require.NotEmpty(t, encryptedData.Data)
	require.Equal(t, "JWE-RSA", encryptedData.Type)
}
