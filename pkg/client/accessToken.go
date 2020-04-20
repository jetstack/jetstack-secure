package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
			return nil, nil
		}
	}

	return c.accessToken, nil
}

func (c *PreflightClient) renewAccessToken() error {
	url := fmt.Sprintf("https://%s/oauth/token", authServer)
	// TODO: this will be dynamic in the future, but at the moment this client only sends readings.
	audience := "https://preflight.jetstack.io/api/v1/datareading"
	payload := fmt.Sprintf("grant_type=password&client_id=%s&client_secret=%s&audience=%s&username=%s&password=%s", clientID, clientSecret, audience, c.userKey, c.userKeySecret)
	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
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

	response := struct {
		Bearer    string `json:"access_token"`
		ExpiresIn uint   `json:"expires_in"`
	}{}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return errors.Trace(err)
	}

	if response.ExpiresIn == 0 {
		return fmt.Errorf("got wrong expiration for access token")
	}

	c.accessToken.bearer = response.Bearer
	c.accessToken.expirationDate = time.Now().Add(time.Duration(response.ExpiresIn) * time.Second)

	return nil
}
