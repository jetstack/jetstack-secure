package hpke

import "filippo.io/hpke"

// Encryptor provides envelope encryption using HPKE (Hybrid Public Key Encryption).
// HPKE combines key encapsulation and authenticated encryption in a single operation.
type Encryptor struct {
	publicKey hpke.PublicKey
	kdf       hpke.KDF
	aead      hpke.AEAD
}

// Decryptor provides HPKE decryption using a private key.
type Decryptor struct {
	privateKey hpke.PrivateKey
	kdf        hpke.KDF
	aead       hpke.AEAD
}

// EncryptedData contains the result of HPKE encryption.
// Unlike RSA envelope encryption, HPKE integrates key encapsulation and data encryption,
// so we only need the encapsulated key and the ciphertext.
type EncryptedData struct {
	// EncapsulatedKey is the KEM (Key Encapsulation Mechanism) output that
	// allows the recipient to derive the shared secret
	EncapsulatedKey []byte

	// Ciphertext is the encrypted data including the authentication tag
	Ciphertext []byte
}
