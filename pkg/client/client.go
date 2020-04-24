package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/jetstack/preflight/api"
)

// These variables are injected at build time.

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

	baseURL string

	// basicAuthToken will be used instead of using OAuth2 based authentication if userID is not set.
	// It can be empty, meaning that no authentication will be used.
	basicAuthToken string

	agentMetadata *api.AgentMetadata
}

// NewWithBasicAuth creates a new client with basic authentication.
func NewWithBasicAuth(agentMetadata *api.AgentMetadata, authToken, baseURL string) (*PreflightClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("cannot create PreflightClient: baseURL cannot be empty")
	}

	return &PreflightClient{
		agentMetadata:  agentMetadata,
		basicAuthToken: authToken,
		baseURL:        baseURL,
	}, nil
}

// New creates a new client that uses OAuth2.
func New(agentMetadata *api.AgentMetadata, userID, userSecret, baseURL string) (*PreflightClient, error) {
	if userID == "" || userSecret == "" {
		return nil, fmt.Errorf("cannot create PreflightClient: neither userID or userSecret can be empty")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("cannot create PreflightClient: baseURL cannot be empty")
	}

	if clientID == "" || clientSecret == "" || authServer == "" {
		return nil, fmt.Errorf("cannot create PreflightClient: this build does not have a valid OAuth client configuration")
	}

	return &PreflightClient{
		agentMetadata: agentMetadata,
		userID:        userID,
		userSecret:    userSecret,
		baseURL:       baseURL,
		accessToken:   &accessToken{},
	}, nil
}

func (c *PreflightClient) usingOAuth2() bool {
	return c.userID != ""
}

// PostDataReadings sends a slice of readings to Preflight.
func (c *PreflightClient) PostDataReadings(orgID string, readings []*api.DataReading) error {
	payload := api.DataReadingsPost{
		AgentMetadata: c.agentMetadata,
		DataReadings:  readings,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	res, err := c.Post(filepath.Join("/api/v1/org", orgID, "datareadings"), bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	if code := res.StatusCode; code < 200 || code >= 300 {
		errorContent := ""
		body, err := ioutil.ReadAll(res.Body)
		if err == nil {
			errorContent = string(body)
		}
		defer res.Body.Close()

		return fmt.Errorf("Received response with status code %d. Body: %s", code, errorContent)
	}

	return nil
}
