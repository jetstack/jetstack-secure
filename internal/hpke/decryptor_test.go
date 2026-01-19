package hpke_test

import (
	"testing"

	"github.com/jetstack/preflight/internal/hpke"
	"github.com/stretchr/testify/require"
)

func TestNewDecryptor_NilKey(t *testing.T) {
	dec, err := hpke.NewDecryptor(nil)
	require.Error(t, err)
	require.Nil(t, dec)
	require.Contains(t, err.Error(), "cannot be nil")
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	publicKey, privateKey, err := hpke.GenerateKeyPair()
	require.NoError(t, err)

	enc, err := hpke.NewEncryptor(publicKey)
	require.NoError(t, err)

	dec, err := hpke.NewDecryptor(privateKey)
	require.NoError(t, err)

	originalData := []byte("sensitive data to encrypt and decrypt")

	// Encrypt
	encrypted, err := enc.Encrypt(originalData)
	require.NoError(t, err)
	require.NotNil(t, encrypted)

	// Decrypt
	decrypted, err := dec.Decrypt(encrypted)
	require.NoError(t, err)
	require.Equal(t, originalData, decrypted)
}

func TestEncryptDecrypt_WithInfo(t *testing.T) {
	publicKey, privateKey, err := hpke.GenerateKeyPair()
	require.NoError(t, err)

	enc, err := hpke.NewEncryptor(publicKey)
	require.NoError(t, err)

	dec, err := hpke.NewDecryptor(privateKey)
	require.NoError(t, err)

	originalData := []byte("sensitive data")
	info := []byte("application-specific context")

	// Encrypt with info
	encrypted, err := enc.EncryptWithInfo(originalData, info)
	require.NoError(t, err)

	// Decrypt with matching info
	decrypted, err := dec.DecryptWithInfo(encrypted, info)
	require.NoError(t, err)
	require.Equal(t, originalData, decrypted)

	// Decrypt with wrong info should fail
	_, err = dec.DecryptWithInfo(encrypted, []byte("wrong info"))
	require.Error(t, err)

	// Decrypt without info should fail
	_, err = dec.Decrypt(encrypted)
	require.Error(t, err)
}

func TestEncryptDecrypt_WithAAD(t *testing.T) {
	publicKey, privateKey, err := hpke.GenerateKeyPair()
	require.NoError(t, err)

	enc, err := hpke.NewEncryptor(publicKey)
	require.NoError(t, err)

	dec, err := hpke.NewDecryptor(privateKey)
	require.NoError(t, err)

	originalData := []byte("sensitive data")
	aad := []byte("additional authenticated data")

	// Encrypt with AAD
	encrypted, err := enc.EncryptWithAAD(originalData, aad)
	require.NoError(t, err)

	// Decrypt with matching AAD
	decrypted, err := dec.DecryptWithAAD(encrypted, aad)
	require.NoError(t, err)
	require.Equal(t, originalData, decrypted)

	// Decrypt with wrong AAD should fail
	_, err = dec.DecryptWithAAD(encrypted, []byte("wrong aad"))
	require.Error(t, err)

	// Decrypt without AAD should fail
	_, err = dec.Decrypt(encrypted)
	require.Error(t, err)
}

func TestDecrypt_WrongKey(t *testing.T) {
	publicKey1, _, err := hpke.GenerateKeyPair()
	require.NoError(t, err)

	_, privateKey2, err := hpke.GenerateKeyPair()
	require.NoError(t, err)

	enc, err := hpke.NewEncryptor(publicKey1)
	require.NoError(t, err)

	dec, err := hpke.NewDecryptor(privateKey2)
	require.NoError(t, err)

	data := []byte("test data")
	encrypted, err := enc.Encrypt(data)
	require.NoError(t, err)

	// Decryption with wrong key should fail
	_, err = dec.Decrypt(encrypted)
	require.Error(t, err)
}

func TestDecrypt_CorruptedData(t *testing.T) {
	publicKey, privateKey, err := hpke.GenerateKeyPair()
	require.NoError(t, err)

	enc, err := hpke.NewEncryptor(publicKey)
	require.NoError(t, err)

	dec, err := hpke.NewDecryptor(privateKey)
	require.NoError(t, err)

	data := []byte("test data")
	encrypted, err := enc.Encrypt(data)
	require.NoError(t, err)

	// Corrupt the ciphertext
	encrypted.Ciphertext[0] ^= 0xFF

	// Decryption should fail due to authentication failure
	_, err = dec.Decrypt(encrypted)
	require.Error(t, err)
}
