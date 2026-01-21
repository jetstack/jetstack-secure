package envelope

// EncryptedData contains the result of envelope encryption.
// It includes the encrypted data, the encrypted symmetric key which was used for encrypting the original data,
// and the nonce needed for the symmetric decryption.
type EncryptedData struct {
	// KeyID is the identifier of the asymmetric key used to encrypt the AES key.
	KeyID string `json:"key_id"`

	// KeyAlgorithm is the algorithm of the asymmetric key used to encrypt the AES key.
	KeyAlgorithm string `json:"key_algorithm"`

	// EncryptedKey is an encrypted AES-256-GCM symmetric key, used to encrypt EncryptedData.
	// This is ciphertext and should only be decryptable by the holder of the private key.
	EncryptedKey []byte `json:"encrypted_key"`

	// EncryptedData is the actual data encrypted using the AES-256-GCM in EncryptedKey.
	// This is ciphertext and requires the decrypted AES key and nonce for decryption.
	EncryptedData []byte `json:"encrypted_data"`

	// Nonce is the 12-byte nonce used for AES-GCM encryption.
	// This is intentionally plaintext.
	Nonce []byte `json:"nonce"`
}

// Encryptor performs envelope encryption on arbitrary data.
type Encryptor interface {
	// Encrypt encrypts data using envelope encryption, returning the resulting data along
	// with identifiers of the asymmetric key used to encrypt the AES key.
	Encrypt(data []byte) (*EncryptedData, error)
}
