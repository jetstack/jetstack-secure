package identity

import (
	"net/http"
	"sync"
	"time"
)

// defaultTokenTTL is the fallback lifetime this client assumes for a cached
// token before it re-logs-in. It's a Client field, not a const, so tests can
// shrink it.
const defaultTokenTTL = 15 * time.Minute

// Client is a client for interacting with the CyberArk Identity API.
// It caches an authentication token and exposes it for use by AuthenticateRequest.
type Client struct {
	httpClient *http.Client
	baseURL    string
	subdomain  string
	tokenTTL   time.Duration

	tokenCached      token
	tokenCachedMutex sync.Mutex
	tokenCachedTime  time.Time

	// username and secret are a durable copy of the credentials most recently
	// passed to LoginUsernamePassword, retained so AuthenticateRequest can
	// re-login on its own once the cached token ages out. Because this copy
	// exists, LoginUsernamePassword deliberately does NOT zero the caller's
	// slice — the caller may reuse it (NewCyberArk's ClientConfig is shared
	// across every upload). This doesn't meaningfully change the secret's
	// exposure: it already lives in memory for the process's lifetime in the
	// config this client was constructed from.
	username string
	secret   []byte
}

// token is a wrapper type for holding auth tokens we want to cache.
type token struct {
	Username string
	Token    string
}

// New returns an initialized CyberArk Identity client.
func New(httpClient *http.Client, baseURL string, subdomain string) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		subdomain:  subdomain,
		tokenTTL:   defaultTokenTTL,

		tokenCached:      token{},
		tokenCachedMutex: sync.Mutex{},
	}
}
