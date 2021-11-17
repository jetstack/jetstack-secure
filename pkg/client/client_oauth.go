package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/juju/errors"
)

type (
	// The OAuthClient type is a Client implementation used to upload data readings to the Jetstack Secure platform
	// using OAuth as its authentication method.
	OAuthClient struct {
		credentials   *Credentials
		accessToken   *accessToken
		baseURL       string
		agentMetadata *api.AgentMetadata
		client        *http.Client
	}

	accessToken struct {
		bearer         string
		expirationDate time.Time
	}
)

func (t *accessToken) needsRenew() bool {
	return t.bearer == "" || time.Now().After(t.expirationDate)
}

// NewOAuthClient returns a new instance of the OAuthClient type that will perform HTTP requests using OAuth to provide
// authentication tokens to the backend API.
func NewOAuthClient(agentMetadata *api.AgentMetadata, credentials *Credentials, baseURL string) (*OAuthClient, error) {
	if err := credentials.validate(); err != nil {
		return nil, fmt.Errorf("cannot create OAuthClient: %v", err)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("cannot create OAuthClient: baseURL cannot be empty")
	}

	if !credentials.IsClientSet() {
		credentials.ClientID = ClientID
		credentials.ClientSecret = ClientSecret
		credentials.AuthServerDomain = AuthServerDomain
	}

	if !credentials.IsClientSet() {
		return nil, fmt.Errorf("cannot create OAuthClient: invalid OAuth2 client configuration")
	}

	return &OAuthClient{
		agentMetadata: agentMetadata,
		credentials:   credentials,
		baseURL:       baseURL,
		accessToken:   &accessToken{},
		client:        &http.Client{Timeout: time.Minute},
	}, nil
}

// PostDataReadings uploads the slice of api.DataReading to the Jetstack Secure backend to be processed for later
// viewing in the user-interface.
func (c *OAuthClient) PostDataReadings(orgID, clusterID string, readings []*api.DataReading) error {
	payload := api.DataReadingsPost{
		AgentMetadata:  c.agentMetadata,
		DataGatherTime: time.Now().UTC(),
		DataReadings:   readings,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	res, err := c.Post(filepath.Join("/api/v1/org", orgID, "datareadings", clusterID), bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if code := res.StatusCode; code < 200 || code >= 300 {
		errorContent := ""
		body, err := ioutil.ReadAll(res.Body)
		if err == nil {
			errorContent = string(body)
		}

		return fmt.Errorf("received response with status code %d. Body: %s", code, errorContent)
	}

	return nil
}

// Post performs an HTTP POST request.
func (c *OAuthClient) Post(path string, body io.Reader) (*http.Response, error) {
	token, err := c.getValidAccessToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fullURL(c.baseURL, path), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if len(token.bearer) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.bearer))
	}

	return c.client.Do(req)
}

// getValidAccessToken returns a valid access token. It will fetch a new access
// token from the auth server in case the current access token does not exist
// or it is expired.
func (c *OAuthClient) getValidAccessToken() (*accessToken, error) {
	if c.accessToken.needsRenew() {
		err := c.renewAccessToken()
		if err != nil {
			return nil, err
		}
	}

	return c.accessToken, nil
}

func (c *OAuthClient) renewAccessToken() error {
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
