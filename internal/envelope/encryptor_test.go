package envelope_test

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jetstack/preflight/internal/envelope"
)

func TestNewEncryptor_ValidKeys(t *testing.T) {
	tests := []struct {
		name    string
		keySize int
	}{
		{"2048 bits", 2048},
		{"3072 bits", 3072},
		{"4096 bits", 4096},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := rsa.GenerateKey(rand.Reader, tt.keySize)
			require.NoError(t, err)

			enc, err := envelope.NewEncryptor(&key.PublicKey)
			require.NoError(t, err)
			require.NotNil(t, enc)
		})
	}
}

func TestNewEncryptor_RejectsSmallKeys(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)

	enc, err := envelope.NewEncryptor(&key.PublicKey)
	require.Error(t, err)
	require.Nil(t, enc)
	require.Contains(t, err.Error(), "must be at least 2048 bits")
}

func TestNewEncryptor_NilKey(t *testing.T) {
	enc, err := envelope.NewEncryptor(nil)
	require.Error(t, err)
	require.Nil(t, enc)
	require.Contains(t, err.Error(), "cannot be nil")
}

func TestEncrypt_VariousDataSizes(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	enc, err := envelope.NewEncryptor(&key.PublicKey)
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
			require.NotEmpty(t, result.EncryptedKey)
			require.NotEmpty(t, result.EncryptedData)
			require.NotEmpty(t, result.Nonce)

			// Verify nonce is correct size (12 bytes for GCM)
			require.Len(t, result.Nonce, 12)

			// Verify encrypted data differs from input
			require.NotEqual(t, data, result.EncryptedData)
		})
	}
}

func TestEncrypt_EmptyData(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	enc, err := envelope.NewEncryptor(&key.PublicKey)
	require.NoError(t, err)

	result, err := enc.Encrypt([]byte{})
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "cannot be empty")
}

func TestEncrypt_NonDeterministic(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	enc, err := envelope.NewEncryptor(&key.PublicKey)
	require.NoError(t, err)

	data := []byte("test data for encryption")

	// Encrypt the same data twice
	result1, err := enc.Encrypt(data)
	require.NoError(t, err)

	result2, err := enc.Encrypt(data)
	require.NoError(t, err)

	// Nonces should be different (random)
	require.NotEqual(t, result1.Nonce, result2.Nonce)

	// Encrypted data should be different due to different nonces
	require.NotEqual(t, result1.EncryptedData, result2.EncryptedData)

	// Encrypted keys should be different due to RSA-OAEP randomness
	require.NotEqual(t, result1.EncryptedKey, result2.EncryptedKey)
}

func TestEncrypt_AllFieldsPopulated(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	enc, err := envelope.NewEncryptor(&key.PublicKey)
	require.NoError(t, err)

	data := []byte("test data")
	result, err := enc.Encrypt(data)
	require.NoError(t, err)

	require.NotNil(t, result)
	require.NotEmpty(t, result.EncryptedKey, "EncryptedKey should be populated")
	require.NotEmpty(t, result.EncryptedData, "EncryptedData should be populated")
	require.NotEmpty(t, result.Nonce, "Nonce should be populated")

	// Verify encrypted key size is appropriate for RSA 2048
	require.Equal(t, 256, len(result.EncryptedKey), "EncryptedKey should be 256 bytes for RSA 2048")
}
