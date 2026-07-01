package conjur

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jetstack/preflight/internal/cyberark/jwtsource"
)

const tokenTTL = 8 * time.Minute

// Client exchanges a JWT for a Conjur access token and authenticates requests with it.
type Client struct {
	httpClient *http.Client
	baseURL    string
	serviceID  string
	account    string
	src        jwtsource.Source

	mu        sync.Mutex
	token     string
	tokenTime time.Time
}

func New(httpClient *http.Client, baseURL, serviceID, account string, src jwtsource.Source) *Client {
	return &Client{httpClient: httpClient, baseURL: baseURL, serviceID: serviceID, account: account, src: src}
}

func (c *Client) exchange(ctx context.Context) (string, error) {
	jwt, err := c.src.Read(ctx)
	if err != nil {
		return "", err
	}
	endpoint, err := url.JoinPath(c.baseURL, "authn-jwt", c.serviceID, c.account, "authenticate")
	if err != nil {
		return "", err
	}
	form := url.Values{"jwt": {jwt}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("authn-jwt exchange transport error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// 401 here most often means the SA token audience != authenticator audience=conjur
		return "", fmt.Errorf("authn-jwt exchange rejected (%d): verify service_id, the authenticator is enabled, and the SA token audience is 'conjur'", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

// AuthenticateRequest implements identity.RequestAuthenticator.
// It exchanges the JWT for a Conjur access token, sets the Authorization header,
// and returns the service ID as the identity string (used for audit tagging).
func (c *Client) AuthenticateRequest(req *http.Request) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == "" || time.Since(c.tokenTime) >= tokenTTL {
		tok, err := c.exchange(req.Context())
		if err != nil {
			return "", err
		}
		c.token, c.tokenTime = tok, time.Now()
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	return c.serviceID, nil
}
