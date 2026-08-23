package identity

import (
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
)

type RequestAuthenticator func(req *http.Request) (string, error)

// AuthenticateRequest is a helper function that adds the Authorization header
// to an HTTP request using a cached token, refreshing it first if it's aged
// past tokenTTL. It sets the Header directly, and if successful returns the
// username corresponding to the token.
//
// Refresh needs LoginUsernamePassword to have been called at least once —
// that's where the username/password this re-login uses gets captured. If a
// refresh attempt fails, this falls back to whatever's cached rather than
// failing the request outright: the failure is likely transient, and the
// next call will retry.
func (c *Client) AuthenticateRequest(req *http.Request) (string, error) {
	c.tokenCachedMutex.Lock()
	defer c.tokenCachedMutex.Unlock()

	if len(c.tokenCached.Token) == 0 && c.username == "" {
		return "", fmt.Errorf("no token cached")
	}

	if c.username != "" {
		if err := c.login(req.Context(), c.username, c.secret); err != nil {
			klog.FromContext(req.Context()).Error(err, "failed to refresh CyberArk Identity token; using cached token")
		}
	}

	if len(c.tokenCached.Token) == 0 {
		return "", fmt.Errorf("no token cached")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.tokenCached.Token))

	return c.tokenCached.Username, nil
}
