package keyfetch

import (
	"context"
	"crypto/rsa"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"

	"github.com/jetstack/preflight/internal/cyberark/servicediscovery"
)

const (
	// minRSAKeySize is the minimum RSA key size in bits; we'd expect that keys will be larger but 2048 is a sane floor
	// to enforce to ensure that a weak key can't accidentally be used
	minRSAKeySize = 2048
)

// KeyFetcher is an interface for fetching public keys.
type KeyFetcher interface {
	// FetchKey retrieves a public key from the key source.
	FetchKey(ctx context.Context) (PublicKey, error)
}

// Compile-time check that Client implements KeyFetcher
var _ KeyFetcher = (*Client)(nil)

// PublicKey represents an RSA public key retrieved from the key server.
type PublicKey struct {
	// KeyID is the unique identifier for this key
	KeyID string

	// Key is the actual RSA public key
	Key *rsa.PublicKey
}

// Client fetches public keys from a CyberArk HTTP endpoint that provides keys in JWKS format.
// It can be expanded in future to support other key types and formats, but for now it only supports RSA keys
// and ignored other types.
type Client struct {
	discoveryClient *servicediscovery.Client

	// httpClient is the HTTP client used for requests
	httpClient *http.Client
}

// NewClient creates a new key fetching client.
// Uses CyberArk service discovery to derive the JWKS endpoint
func NewClient(discoveryClient *servicediscovery.Client) *Client {
	return &Client{
		discoveryClient: discoveryClient,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// FetchKey retrieves the public keys from the configured endpoint.
// It returns a slice of PublicKey structs containing the key material and metadata.
func (c *Client) FetchKey(ctx context.Context) (PublicKey, error) {
	services, _, err := c.discoveryClient.DiscoverServices(ctx)
	if err != nil {
		return PublicKey{}, fmt.Errorf("failed to get services from discovery client: %w", err)
	}

	endpoint, err := url.JoinPath(services.DiscoveryContext.API, "discovery-context/jwks")
	if err != nil {
		return PublicKey{}, fmt.Errorf("failed to construct endpoint URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return PublicKey{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PublicKey{}, fmt.Errorf("failed to fetch keys from %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return PublicKey{}, fmt.Errorf("unexpected status code %d from %s: %s", resp.StatusCode, endpoint, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return PublicKey{}, fmt.Errorf("failed to read response body: %w", err)
	}

	keySet, err := jwk.Parse(body)
	if err != nil {
		return PublicKey{}, fmt.Errorf("failed to parse JWKs response: %w", err)
	}

	for i := range keySet.Len() {
		key, ok := keySet.Key(i)
		if !ok {
			continue
		}

		// Only process RSA keys
		if key.KeyType().String() != "RSA" {
			continue
		}

		var rawKey any
		if err := jwk.Export(key, &rawKey); err != nil {
			// skip unparseable keys
			continue
		}

		rsaKey, ok := rawKey.(*rsa.PublicKey)
		if !ok {
			// only process RSA keys (for now)
			continue
		}

		if rsaKey.N.BitLen() < minRSAKeySize {
			// skip keys that are too small to be secure
			continue
		}

		kid, ok := key.KeyID()
		if !ok {
			// skip any keys which don't have an ID
			continue
		}

		alg, ok := key.Algorithm()
		if !ok {
			// skip any keys which don't have an algorithm specified
			continue
		}

		if alg.String() != "RSA-OAEP-256" {
			// we only use RSA keys for RSA-OAEP-256
			continue
		}

		// return the first valid key we find
		return PublicKey{
			KeyID: kid,
			Key:   rsaKey,
		}, nil
	}

	return PublicKey{}, fmt.Errorf("no valid RSA keys found at %s", endpoint)
}
