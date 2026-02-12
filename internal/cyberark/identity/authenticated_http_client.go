package identity

import (
	"fmt"
	"net/http"
)

type RequestAuthenticator func(req *http.Request) (string, error)

// AuthenticateRequest is a helper function that adds the Authorization header to an HTTP request using a cached token.
// It sets the Header directly, and if successful returns the username corresponding to the token.
func (c *Client) AuthenticateRequest(req *http.Request) (string, error) {
	c.tokenCachedMutex.Lock()
	defer c.tokenCachedMutex.Unlock()

	if len(c.tokenCached.Token) == 0 {
		return "", fmt.Errorf("no token cached")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.tokenCached.Token))

	return c.tokenCached.Username, nil
}
