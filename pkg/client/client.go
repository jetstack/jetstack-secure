package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/jetstack/preflight/api"
)

// This variables are injected at build time.

var clientID string
var clientSecret string
var authServer string

// PreflightClient can be used to talk to the Preflight backend.
type PreflightClient struct {
	// OAuth2
	userID     string
	userSecret string
	// accessToken is the current OAuth access token.
	accessToken *accessToken

	// TODO: in the future, PreflightClient will be configured with the server
	// base URL, organization and cluster ID and it will calculate the readings
	// endpoint and others.

	// readingsEndpoint is the endpoint where the readings will be sent to.
	readingsEndpoint string

	// basicAuthToken will be used instead of using OAuth2 based authentication if userID is not set.
	// It can be empty, meaning that no authentication will be used.
	basicAuthToken string
}

// NewWithBasicAuth creates a new client with basic authentication.
func NewWithBasicAuth(authToken, readingsEndpoint string) (*PreflightClient, error) {
	if readingsEndpoint == "" {
		return nil, fmt.Errorf("cannot create PreflightClient: readingsEndpoint cannot be empty")
	}

	return &PreflightClient{
		basicAuthToken:   authToken,
		readingsEndpoint: readingsEndpoint,
	}, nil
}

// New creates a new client that uses OAuth2.
func New(userID, userSecret, readingsEndpoint string) (*PreflightClient, error) {
	if userID == "" || userSecret == "" {
		return nil, fmt.Errorf("cannot create PreflightClient: neither userID or userSecret can be empty")
	}
	if readingsEndpoint == "" {
		return nil, fmt.Errorf("cannot create PreflightClient: readingsEndpoint cannot be empty")
	}

	if clientID == "" || clientSecret == "" || authServer == "" {
		return nil, fmt.Errorf("cannot create PreflightClient: this build does not have a valid OAuth client configuration")
	}

	return &PreflightClient{
		userID:           userID,
		userSecret:       userSecret,
		readingsEndpoint: readingsEndpoint,
		accessToken:      &accessToken{},
	}, nil
}

func (c *PreflightClient) usingOAuth2() bool {
	return c.userID != ""
}

// PostDataReadings sends a slice of readings to Preflight.
func (c *PreflightClient) PostDataReadings(readings []*api.DataReading) error {
	var bearer string
	if !c.usingOAuth2() {
		bearer = c.basicAuthToken
	} else {
		token, err := c.getValidAccessToken()
		if err != nil {
			return err
		}
		bearer = token.bearer
	}

	data, err := json.Marshal(readings)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.readingsEndpoint, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	if len(bearer) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearer))
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if code := res.StatusCode; code < 200 || code >= 300 {
		return fmt.Errorf("Received response with status code %d. Body: %s", code, string(body))
	}

	return nil
}
