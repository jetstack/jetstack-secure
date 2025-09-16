package identity

import (
	"fmt"
	"net/http"
)

func (c *Client) AuthenticateRequest(req *http.Request) error {
	c.tokenCachedMutex.Lock()
	defer c.tokenCachedMutex.Unlock()

	if len(c.tokenCached) == 0 {
		return fmt.Errorf("no token cached")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", string(c.tokenCached)))

	return nil
}
