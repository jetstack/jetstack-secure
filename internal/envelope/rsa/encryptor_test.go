package rsa

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"sync"
	"testing"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwe"
	"github.com/stretchr/testify/require"
)

const testKeyID = "test-key-id"

// smallRSAKey1024 is a hardcoded 1024-bit RSA public key in PEM format (PKIX)
// used for testing key size validation. This key is intentionally weak and should
// only be used for testing purposes.
// This is hardcoded rather than generated in order to save compute, and also on the
// assumption that future Go releases might restrict the ability to generate such small keys.
const smallRSAKey1024 = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDCNDoCM0OBt4HFxFxyU50FYsuZ
gK+lgel/Jlzb+ghkWpCL1Vk3Au7aet4KxNxQh5dFRxtMU7pe6fC5eZtdL3+0TCUu
XAUVgMhTRn3ZXlEmJXosuiFQ2y4+3nbWL51OxXRf3jsieSVqr4fbceakuOKXp4vX
wgiguV3/XqaysHs1uwIDAQAB
-----END PUBLIC KEY-----`

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
	// Parse the hardcoded 1024-bit RSA public key from PEM format
	block, _ := pem.Decode([]byte(smallRSAKey1024))
	require.NotNil(t, block, "failed to decode PEM block")

	// NB: a future Go update might restrict the ability to parse small keys;
	// if that happens, this test will need to be removed or changed.
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	require.NoError(t, err, "failed to parse RSA public key")

	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	require.True(t, ok, "key should be an RSA public key")

	enc, err := NewEncryptor(testKeyID, rsaPubKey)
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
			require.Equal(t, EncryptionType, result.Type, "Type should be JWE-RSA")

			// Verify JWE Compact Serialization format (5 base64url parts separated by dots)
			jweString := string(result.Data)
			parts := strings.Split(jweString, ".")
			require.Len(t, parts, 5, "JWE Compact Serialization should have 5 parts")

			// Verify each part is non-empty
			for i, part := range parts {
				require.NotEmpty(t, part, "JWE part %d should not be empty", i)

				_, err = base64.RawURLEncoding.DecodeString(part)
				require.NoError(t, err, "JWE part %d should be valid base64url: %s", i, part)
			}

			// Verify the result differs from input
			require.NotEqual(t, data, result.Data)
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
	require.Equal(t, EncryptionType, result1.Type, "Type should be JWE-RSA")

	result2, err := enc.Encrypt(data)
	require.NoError(t, err)
	require.Equal(t, EncryptionType, result2.Type, "Type should be JWE-RSA")

	// Results should be different due to random nonces and RSA-OAEP randomness
	require.NotEqual(t, result1.Data, result2.Data, "Encrypting the same data twice should produce different JWE outputs")
}

func TestEncrypt_JWEFormat(t *testing.T) {
	key := testKey()

	enc, err := NewEncryptor(testKeyID, &key.PublicKey)
	require.NoError(t, err)

	data := []byte("test data")
	result, err := enc.Encrypt(data)
	require.NoError(t, err)
	require.Equal(t, EncryptionType, result.Type, "Type should be JWE-RSA")

	// Parse and decrypt the JWE to verify format and algorithms
	decrypted, err := jwe.Decrypt(result.Data, jwe.WithKey(jwa.RSA_OAEP_256(), key), jwe.WithContext(t.Context()))
	require.NoError(t, err, "Result should be valid JWE with RSA-OAEP-256 and A256GCM, and should decrypt successfully")
	require.Equal(t, data, decrypted, "Decrypted data should match original")
}

func TestEncrypt_DecryptRoundtrip(t *testing.T) {
	key := testKey()

	enc, err := NewEncryptor(testKeyID, &key.PublicKey)
	require.NoError(t, err)

	originalData := []byte("test data for roundtrip encryption and decryption")

	// Encrypt the data
	encrypted, err := enc.Encrypt(originalData)
	require.NoError(t, err)
	require.Equal(t, EncryptionType, encrypted.Type, "Type should be JWE-RSA")

	msg, err := jwe.Parse(encrypted.Data)
	require.NoError(t, err)

	headers := msg.ProtectedHeaders()

	kidHeader, ok := headers.KeyID()
	require.True(t, ok, "JWE should contain 'kid' header")
	require.Equal(t, testKeyID, kidHeader, "JWE 'kid' header should match the encryptor's key ID")

	// Decrypt using the private key
	decrypted, err := jwe.Decrypt(encrypted.Data, jwe.WithKey(jwa.RSA_OAEP_256(), key), jwe.WithContext(t.Context()))
	require.NoError(t, err, "Decryption should succeed with the correct private key")

	// Verify the decrypted data matches the original
	require.Equal(t, originalData, decrypted, "Decrypted data should match original data")
}
