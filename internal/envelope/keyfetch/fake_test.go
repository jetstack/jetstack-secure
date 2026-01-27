package keyfetch

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeClient(t *testing.T) {
	t.Run("returns generated key by default", func(t *testing.T) {
		fake := NewFakeClient()

		key, err := fake.FetchKey(t.Context())
		require.NoError(t, err)

		assert.Equal(t, "test-key", key.KeyID)
		assert.NotNil(t, key.Key)
		assert.Equal(t, 1, fake.FetchKeyCalls)

		// Subsequent calls return the same key
		key2, err := fake.FetchKey(t.Context())
		require.NoError(t, err)
		assert.Equal(t, key.KeyID, key2.KeyID)
		assert.Equal(t, key.Key, key2.Key)
		assert.Equal(t, 2, fake.FetchKeyCalls)
	})

	t.Run("returns configured key", func(t *testing.T) {
		privateKey, err := rsa.GenerateKey(rand.Reader, minRSAKeySize)
		require.NoError(t, err)

		fake := NewFakeClientWithKey("custom-key", &privateKey.PublicKey)

		key, err := fake.FetchKey(t.Context())
		require.NoError(t, err)

		assert.Equal(t, "custom-key", key.KeyID)
		assert.Equal(t, &privateKey.PublicKey, key.Key)
		assert.Equal(t, 1, fake.FetchKeyCalls)
	})

	t.Run("returns configured error", func(t *testing.T) {
		expectedErr := errors.New("test error")
		fake := NewFakeClientWithError(expectedErr)

		_, err := fake.FetchKey(t.Context())
		require.Error(t, err)

		assert.Equal(t, expectedErr, err)
		assert.Equal(t, 1, fake.FetchKeyCalls)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		fake := NewFakeClient()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := fake.FetchKey(ctx)
		require.Error(t, err)

		assert.Equal(t, context.Canceled, err)
		assert.Equal(t, 1, fake.FetchKeyCalls)
	})

	t.Run("error takes precedence over key", func(t *testing.T) {
		privateKey, err := rsa.GenerateKey(rand.Reader, minRSAKeySize)
		require.NoError(t, err)

		expectedErr := errors.New("test error")
		fake := &FakeClient{
			Key: &PublicKey{
				KeyID: "custom-key",
				Key:   &privateKey.PublicKey,
			},
			Err: expectedErr,
		}

		_, err = fake.FetchKey(t.Context())
		require.Error(t, err)

		assert.Equal(t, expectedErr, err)
	})
}
