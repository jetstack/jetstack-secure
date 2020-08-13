package client

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Post performs a post request.
func (c *PreflightClient) Post(path string, body io.Reader) (*http.Response, error) {
	var bearer string
	if c.usingOAuth2() {
		token, err := c.getValidAccessToken()
		if err != nil {
			return nil, err
		}
		bearer = token.bearer
	}

	req, err := http.NewRequest(http.MethodPost, c.fullURL(path), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if len(bearer) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearer))
	}

	return http.DefaultClient.Do(req)
}

func (c *PreflightClient) fullURL(path string) string {
	base := c.baseURL
	for strings.HasSuffix(base, "/") {
		base = strings.TrimSuffix(base, "/")
	}
	for strings.HasPrefix(path, "/") {
		path = strings.TrimPrefix(path, "/")
	}
	return fmt.Sprintf("%s/%s", base, path)
}
