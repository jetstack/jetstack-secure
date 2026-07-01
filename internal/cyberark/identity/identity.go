package identity

import (
	"net/http"
	"sync"
	"time"
)

// Client is a client for interacting with the CyberArk Identity API.
// It caches an authentication token and exposes it for use by AuthenticateRequest.
type Client struct {
	httpClient *http.Client
	baseURL    string
	subdomain  string

	tokenCached      token
	tokenCachedMutex sync.Mutex
	tokenCachedTime  time.Time
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

		tokenCached:      token{},
		tokenCachedMutex: sync.Mutex{},
	}
}
