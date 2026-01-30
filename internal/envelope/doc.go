// Package envelope provides types and interfaces for envelope encryption.
//
// Envelope encryption combines asymmetric and symmetric cryptography to
// efficiently encrypt data. The Encryptor interface defines the encryption
// operation, returning data in JWE (JSON Web Encryption) format as defined
// in RFC 7516.
//
// Implementations are available in subpackages:
//
//   - internal/envelope/rsa: RSA-OAEP-256 + AES-256-GCM using JWE
//
// See subpackage documentation for usage examples.
package envelope
