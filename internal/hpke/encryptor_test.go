package hpke_test

import (
	"crypto/rand"
	"testing"

	"github.com/jetstack/preflight/internal/hpke"
	"github.com/stretchr/testify/require"
)

func TestNewEncryptor_ValidKey(t *testing.T) {
	publicKey, _, err := hpke.GenerateKeyPair()
	require.NoError(t, err)

	enc, err := hpke.NewEncryptor(publicKey)
	require.NoError(t, err)
	require.NotNil(t, enc)
}

func TestNewEncryptor_NilKey(t *testing.T) {
	enc, err := hpke.NewEncryptor(nil)
	require.Error(t, err)
	require.Nil(t, enc)
	require.Contains(t, err.Error(), "cannot be nil")
}

func TestEncrypt_VariousDataSizes(t *testing.T) {
	publicKey, _, err := hpke.GenerateKeyPair()
	require.NoError(t, err)

	enc, err := hpke.NewEncryptor(publicKey)
	require.NoError(t, err)

	tests := []struct {
		name     string
		dataSize int
	}{
		{"small (10 bytes)", 10},
		{"medium (1 KB)", 1024},
		{"large (1 MB)", 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, tt.dataSize)
			_, err := rand.Read(data)
			require.NoError(t, err)

			result, err := enc.Encrypt(data)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Verify all fields are populated
			require.NotEmpty(t, result.EncapsulatedKey)
			require.NotEmpty(t, result.Ciphertext)

			// Verify ciphertext differs from input
			require.NotEqual(t, data, result.Ciphertext)
		})
	}
}

func TestEncrypt_EmptyData(t *testing.T) {
	publicKey, _, err := hpke.GenerateKeyPair()
	require.NoError(t, err)

	enc, err := hpke.NewEncryptor(publicKey)
	require.NoError(t, err)

	result, err := enc.Encrypt([]byte{})
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "cannot be empty")
}

func TestEncrypt_NonDeterministic(t *testing.T) {
	publicKey, _, err := hpke.GenerateKeyPair()
	require.NoError(t, err)

	enc, err := hpke.NewEncryptor(publicKey)
	require.NoError(t, err)

	data := []byte("test data for encryption")

	// Encrypt the same data twice
	result1, err := enc.Encrypt(data)
	require.NoError(t, err)

	result2, err := enc.Encrypt(data)
	require.NoError(t, err)

	// Encapsulated keys should be different (random for each encryption)
	require.NotEqual(t, result1.EncapsulatedKey, result2.EncapsulatedKey)

	// Ciphertexts should be different
	require.NotEqual(t, result1.Ciphertext, result2.Ciphertext)
}

func TestEncrypt_AllFieldsPopulated(t *testing.T) {
	publicKey, _, err := hpke.GenerateKeyPair()
	require.NoError(t, err)

	enc, err := hpke.NewEncryptor(publicKey)
	require.NoError(t, err)

	data := []byte("test data")
	result, err := enc.Encrypt(data)
	require.NoError(t, err)

	require.NotNil(t, result)
	require.NotEmpty(t, result.EncapsulatedKey, "EncapsulatedKey should be populated")
	require.NotEmpty(t, result.Ciphertext, "Ciphertext should be populated")

	// Verify encapsulated key size is appropriate for X25519 (32 bytes)
	require.Equal(t, 32, len(result.EncapsulatedKey), "EncapsulatedKey should be 32 bytes for X25519")
}
