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
var ClientID string
var ClientSecret string
var AuthServerDomain string

// PreflightClient can be used to talk to the Preflight backend.
type PreflightClient struct {
	// OAuth2
	credentials *Credentials
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
func New(agentMetadata *api.AgentMetadata, credentials *Credentials, baseURL string) (*PreflightClient, error) {
	if credentials == nil || credentials.UserID == "" || credentials.UserSecret == "" {
		return nil, fmt.Errorf("cannot create PreflightClient: neither userID or userSecret can be empty")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("cannot create PreflightClient: baseURL cannot be empty")
	}

	if !credentials.IsClientSet() {
		credentials.ClientID = ClientID
		credentials.ClientSecret = ClientSecret
		credentials.AuthServerDomain = AuthServerDomain
	}

	if !credentials.IsClientSet() {
		return nil, fmt.Errorf("cannot create PreflightClient: invalid OAuth2 client configuration")
	}

	return &PreflightClient{
		agentMetadata: agentMetadata,
		credentials:   credentials,
		baseURL:       baseURL,
		accessToken:   &accessToken{},
	}, nil
}

func (c *PreflightClient) usingOAuth2() bool {
	return c.credentials.UserID != ""
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

		return fmt.Errorf("received response with status code %d. Body: %s", code, errorContent)
	}

	return nil
}
