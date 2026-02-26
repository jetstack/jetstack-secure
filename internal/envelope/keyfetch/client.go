package keyfetch

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/jetstack/venafi-connection-lib/http_client"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/internal/cyberark"
	"github.com/jetstack/preflight/internal/cyberark/identity"
	"github.com/jetstack/preflight/internal/cyberark/servicediscovery"
	"github.com/jetstack/preflight/pkg/version"
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
	identityClient  *identity.Client
	cfg             cyberark.ClientConfig

	// httpClient is the HTTP client used for requests
	httpClient *http.Client

	cachedKey      PublicKey
	cachedKeyMutex sync.Mutex
	cachedKeyTime  time.Time
}

// NewClient creates a new key fetching client.
// Uses CyberArk service discovery to derive the JWKS endpoint and CyberArk identity client for authentication.
// Constructing the client involves a service discovery call to initialise the identity client,
// so this may return an error if the discovery client is not able to connect to the service discovery endpoint.
// If httpClient is nil, a default HTTP client will be created.
func NewClient(ctx context.Context, discoveryClient *servicediscovery.Client, cfg cyberark.ClientConfig, httpClient *http.Client) (*Client, error) {
	if httpClient == nil {
		var rootCAs *x509.CertPool
		httpClient = http_client.NewDefaultClient(version.UserAgent(), rootCAs)
	}

	services, _, err := discoveryClient.DiscoverServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get services from discovery client for initialising identity client: %w", err)
	}

	return &Client{
		discoveryClient: discoveryClient,
		identityClient:  identity.New(httpClient, services.Identity.API, cfg.Subdomain),
		cfg:             cfg,
		httpClient:      httpClient,
	}, nil
}

// FetchKey retrieves the public keys from the configured endpoint.
// It returns a slice of PublicKey structs containing the key material and metadata.
func (c *Client) FetchKey(ctx context.Context) (PublicKey, error) {
	logger := klog.FromContext(ctx).WithName("keyfetch")
	c.cachedKeyMutex.Lock()
	defer c.cachedKeyMutex.Unlock()

	if time.Since(c.cachedKeyTime) < 15*time.Minute {
		klog.FromContext(ctx).WithName("keyfetch").V(2).Info("using cached key", "fetchedAt", c.cachedKeyTime.Format(time.RFC3339Nano), "kid", c.cachedKey.KeyID)
		return c.cachedKey, nil
	}

	services, _, err := c.discoveryClient.DiscoverServices(ctx)
	if err != nil {
		return PublicKey{}, fmt.Errorf("failed to get services from discovery client: %w", err)
	}

	err = c.identityClient.LoginUsernamePassword(ctx, c.cfg.Username, []byte(c.cfg.Secret))
	if err != nil {
		return PublicKey{}, fmt.Errorf("failed to authenticate for fetching JWKs: %w", err)
	}

	endpoint, err := url.JoinPath(services.DiscoveryContext.API, "discovery-context/jwks")
	if err != nil {
		return PublicKey{}, fmt.Errorf("failed to construct endpoint URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return PublicKey{}, fmt.Errorf("failed to create request: %w", err)
	}

	_, err = c.identityClient.AuthenticateRequest(req)
	if err != nil {
		return PublicKey{}, fmt.Errorf("failed to authenticate request: %s", err)
	}

	req.Header.Set("Accept", "application/json")
	version.SetUserAgent(req)

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

		logger.Info("fetched valid RSA key", "kid", kid)

		c.cachedKey = PublicKey{
			KeyID: kid,
			Key:   rsaKey,
		}
		c.cachedKeyTime = time.Now()

		return c.cachedKey, nil
	}

	return PublicKey{}, fmt.Errorf("no valid RSA keys found at %s", endpoint)
}
