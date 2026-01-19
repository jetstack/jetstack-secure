package envelope

import "crypto/rsa"

// Encryptor provides envelope encryption using RSA for key wrapping
// and AES-256-GCM for data encryption.
type Encryptor struct {
	rsaPublicKey *rsa.PublicKey
}

// EncryptedData contains the result of envelope encryption.
// It includes the encrypted data, the encrypted AES key which was used for encrypting the original data,
// and the nonce needed for AES-GCM decryption.
type EncryptedData struct {
	// EncryptedKey is the AES-256 key encrypted with RSA-OAEP-SHA256.
	// This is ciphertext and should only be decryptable by the holder of the corresponding RSA private key.
	EncryptedKey []byte `json:"encrypted_key"`

	// EncryptedData is the actual data encrypted using AES-256-GCM.
	// This is ciphertext and requires the AES key (after RSA decryption) and nonce for decryption.
	EncryptedData []byte `json:"encrypted_data"`

	// Nonce is the 12-byte nonce used for AES-GCM encryption.
	// This is intentionally plaintext.
	Nonce []byte `json:"nonce"`
}
