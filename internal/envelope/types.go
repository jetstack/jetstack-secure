package envelope

// EncryptedData represents encrypted data along with metadata about the encryption type.
type EncryptedData struct {
	// Data contains the encrypted payload
	Data []byte
	// Type indicates the encryption format (e.g., "JWE-RSA")
	Type string
}

// Encryptor performs envelope encryption on arbitrary data.
type Encryptor interface {
	// Encrypt encrypts data using envelope encryption, returning an EncryptedData struct
	// containing the encrypted payload and encryption type metadata.
	Encrypt(data []byte) (*EncryptedData, error)
}
