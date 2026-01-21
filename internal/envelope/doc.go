// Package envelope implements RSA envelope encryption, intended to be used to secure sensitive Secret data from a cluster
// being being sent to an external system. This protects against threats such as TLS interception middleware.
//
// Envelope encryption uses a combination of asymmetric encryption and symmetric encryption; since asymmetric encryption is
// slow and has size limits, we generate a random symmetric key for each encryption operation, use that to encrypt the data,
// then encrypt the symmetric key with the provided RSA public key. The recipient can then use their RSA private key to
// decrypt the symmetric key, then use that to decrypt the data.
//
// This implementation uses RSA-OAEP with SHA-256 for asymmetric encryption, and AES-256-GCM for symmetric encryption.
//
// In some documentation, the asymmetric key is called the "key encryption key" (KEK) and the symmetric key is called the "data encryption key" (DEK).
package envelope
