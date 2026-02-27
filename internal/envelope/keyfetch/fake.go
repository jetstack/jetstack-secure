package keyfetch

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
)

// Compile-time check that FakeClient implements KeyFetcher
var _ KeyFetcher = (*FakeClient)(nil)

// FakeClient is a fake implementation of the key fetcher for testing.
// It can be configured to return specific keys or errors for testing different scenarios.
type FakeClient struct {
	// Key is the public key that will be returned by FetchKey.
	// If nil, a random key will be generated on the first call.
	Key *PublicKey

	// Err is the error that will be returned by FetchKey.
	// If both Key and Err are set, Err takes precedence.
	Err error

	// FetchKeyCalls tracks how many times FetchKey was called
	FetchKeyCalls int
}

// NewFakeClient creates a new fake client for testing.
func NewFakeClient() *FakeClient {
	return &FakeClient{}
}

// NewFakeClientWithKey creates a new fake client that returns the specified key.
func NewFakeClientWithKey(keyID string, key *rsa.PublicKey) *FakeClient {
	return &FakeClient{
		Key: &PublicKey{
			KeyID: keyID,
			Key:   key,
		},
	}
}

// NewFakeClientWithError creates a new fake client that returns the specified error.
func NewFakeClientWithError(err error) *FakeClient {
	return &FakeClient{
		Err: err,
	}
}

// FetchKey implements the key fetching interface for testing.
// It returns the configured key or error, or generates a random key if none is configured.
func (f *FakeClient) FetchKey(ctx context.Context) (PublicKey, error) {
	f.FetchKeyCalls++

	// Check if context is canceled
	if ctx.Err() != nil {
		return PublicKey{}, ctx.Err()
	}

	// If an error is configured, return it
	if f.Err != nil {
		return PublicKey{}, f.Err
	}

	// If a key is configured, return it
	if f.Key != nil {
		return *f.Key, nil
	}

	// Generate a random key for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, minRSAKeySize)
	if err != nil {
		return PublicKey{}, fmt.Errorf("failed to generate test key: %w", err)
	}

	generatedKey := PublicKey{
		KeyID: "test-key",
		Key:   &privateKey.PublicKey,
	}

	// Cache the generated key for subsequent calls
	f.Key = &generatedKey

	return generatedKey, nil
}
