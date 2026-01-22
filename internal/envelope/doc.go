// Package envelope provides types and interfaces for envelope encryption.
//
// Envelope encryption combines asymmetric and symmetric cryptography to
// efficiently encrypt data. The EncryptedData type holds the result, and
// the Encryptor interface defines the encryption operation.
//
// Implementations are available in subpackages:
//
//   - internal/envelope/rsa: RSA-OAEP + AES-256-GCM
//
// See subpackage documentation for usage examples.
package envelope
