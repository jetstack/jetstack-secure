// Package keyfetch provides a client for fetching encryption keys from an HTTP endpoint.
//
// The client retrieves public keys in JSON Web Key Set (JWKs) format from a remote
// server and converts them into usable cryptographic keys for envelope encryption.
//
// This package uses github.com/lestrrat-go/jwx/v3/jwk for JWK parsing and handling.
//
// Currently, keyfetch only supports RSA keys for envelope encryption.
package keyfetch
