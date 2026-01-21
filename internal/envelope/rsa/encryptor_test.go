package rsa

import (
	"crypto/rand"
	"crypto/rsa"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

const testKeyID = "test-key-id"

var (
	testKeyOnce     sync.Once
	internalTestKey *rsa.PrivateKey
)

// testKey generates and returns a singleton RSA private key for testing purposes,
// to avoid needing to generate a new key for each test.
func testKey() *rsa.PrivateKey {
	testKeyOnce.Do(func() {
		key, err := rsa.GenerateKey(rand.Reader, minRSAKeySize)
		if err != nil {
			panic("failed to generate test RSA key: " + err.Error())
		}

		internalTestKey = key
	})

	return internalTestKey
}

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

			enc, err := NewEncryptor(testKeyID, &key.PublicKey)
			require.NoError(t, err)
			require.NotNil(t, enc)
		})
	}
}

func TestNewEncryptor_RejectsSmallKeys(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)

	enc, err := NewEncryptor(testKeyID, &key.PublicKey)
	require.Error(t, err)
	require.Nil(t, enc)
	require.Contains(t, err.Error(), "must be at least 2048 bits")
}

func TestNewEncryptor_NilKey(t *testing.T) {
	enc, err := NewEncryptor(testKeyID, nil)
	require.Error(t, err)
	require.Nil(t, enc)
	require.Contains(t, err.Error(), "cannot be nil")
}

func TestNewEncryptor_EmptyKeyID(t *testing.T) {
	key := testKey()

	enc, err := NewEncryptor("", &key.PublicKey)
	require.Error(t, err)
	require.Nil(t, enc)
	require.Contains(t, err.Error(), "keyID cannot be empty")
}

func TestEncrypt_VariousDataSizes(t *testing.T) {
	key := testKey()

	enc, err := NewEncryptor(testKeyID, &key.PublicKey)
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

			// Verify KeyID and KeyAlgorithm are set correctly
			require.Equal(t, testKeyID, result.KeyID)
			require.Equal(t, keyAlgorithmIdentifier, result.KeyAlgorithm)

			// Verify nonce is correct size (12 bytes for GCM)
			require.Len(t, result.Nonce, 12)

			// Verify encrypted data differs from input
			require.NotEqual(t, data, result.EncryptedData)
		})
	}
}

func TestEncrypt_EmptyData(t *testing.T) {
	key := testKey()

	enc, err := NewEncryptor(testKeyID, &key.PublicKey)
	require.NoError(t, err)

	result, err := enc.Encrypt([]byte{})
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "cannot be empty")
}

func TestEncrypt_NonDeterministic(t *testing.T) {
	key := testKey()

	enc, err := NewEncryptor(testKeyID, &key.PublicKey)
	require.NoError(t, err)

	data := []byte("test data for encryption")

	// Encrypt the same data twice
	result1, err := enc.Encrypt(data)
	require.NoError(t, err)

	result2, err := enc.Encrypt(data)
	require.NoError(t, err)

	// Verify KeyID and KeyAlgorithm are set correctly in both results
	require.Equal(t, testKeyID, result1.KeyID)
	require.Equal(t, keyAlgorithmIdentifier, result1.KeyAlgorithm)
	require.Equal(t, testKeyID, result2.KeyID)
	require.Equal(t, keyAlgorithmIdentifier, result2.KeyAlgorithm)

	// Nonces should be different (random)
	require.NotEqual(t, result1.Nonce, result2.Nonce)

	// Encrypted data should be different due to different nonces
	require.NotEqual(t, result1.EncryptedData, result2.EncryptedData)

	// Encrypted keys should be different due to RSA-OAEP randomness
	require.NotEqual(t, result1.EncryptedKey, result2.EncryptedKey)
}

func TestEncrypt_AllFieldsPopulated(t *testing.T) {
	key := testKey()

	enc, err := NewEncryptor(testKeyID, &key.PublicKey)
	require.NoError(t, err)

	data := []byte("test data")
	result, err := enc.Encrypt(data)
	require.NoError(t, err)

	require.NotNil(t, result)
	require.NotEmpty(t, result.EncryptedKey, "EncryptedKey should be populated")
	require.NotEmpty(t, result.EncryptedData, "EncryptedData should be populated")
	require.NotEmpty(t, result.Nonce, "Nonce should be populated")

	// Verify KeyID and KeyAlgorithm are set correctly
	require.Equal(t, testKeyID, result.KeyID, "KeyID should match the encryptor's keyID")
	require.Equal(t, keyAlgorithmIdentifier, result.KeyAlgorithm, "KeyAlgorithm should be the value of keyAlgorithmIdentifier")

	// Verify encrypted key size is appropriate for RSA 2048
	require.Equal(t, 256, len(result.EncryptedKey), "EncryptedKey should be 256 bytes for RSA 2048")
}
