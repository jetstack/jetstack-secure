// Package rsa implements RSA envelope encryption using JWE (JSON Web Encryption) format.
// It conforms to the interface in the envelope package.
//
// The implementation uses:
//   - RSA-OAEP-256 (RSA-OAEP with SHA-256) for key encryption
//   - AES-256-GCM (A256GCM) for content encryption
//   - JWE Compact Serialization format as defined in RFC 7516
//
// The output is a JWE string with 5 base64url-encoded parts separated by dots:
// header.encryptedKey.iv.ciphertext.tag
package rsa
