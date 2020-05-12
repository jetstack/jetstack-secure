package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/juju/errors"
)

type accessToken struct {
	bearer         string
	expirationDate time.Time
}

func (t *accessToken) needsRenew() bool {
	return t.bearer == "" || time.Now().After(t.expirationDate)
}

// getValidAccessToken returns a valid access token. It will fetch a new access
// token from the auth server in case the current access token does not exist
// or it is expired.
func (c *PreflightClient) getValidAccessToken() (*accessToken, error) {
	if c.accessToken.needsRenew() {
		err := c.renewAccessToken()
		if err != nil {
			return nil, err
		}
	}

	return c.accessToken, nil
}

func (c *PreflightClient) renewAccessToken() error {
	tokenURL := fmt.Sprintf("https://%s/oauth/token", c.credentials.AuthServerDomain)
	audience := "https://preflight.jetstack.io/api/v1"
	payload := url.Values{}
	payload.Set("grant_type", "password")
	payload.Set("client_id", c.credentials.ClientID)
	payload.Set("client_secret", c.credentials.ClientSecret)
	payload.Set("audience", audience)
	payload.Set("username", c.credentials.UserID)
	payload.Set("password", c.credentials.UserSecret)
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(payload.Encode()))
	if err != nil {
		return errors.Trace(err)
	}
	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Trace(err)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Trace(err)
	}

	defer res.Body.Close()

	if status := res.StatusCode; status < 200 || status >= 300 {
		return errors.Errorf("auth server did not provide an access token: (status %d) %s.", status, string(body))
	}

	response := struct {
		Bearer    string `json:"access_token"`
		ExpiresIn uint   `json:"expires_in"`
	}{}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return errors.Trace(err)
	}

	if response.ExpiresIn == 0 {
		return errors.Errorf("got wrong expiration for access token")
	}

	c.accessToken.bearer = response.Bearer
	c.accessToken.expirationDate = time.Now().Add(time.Duration(response.ExpiresIn) * time.Second)

	return nil
}
