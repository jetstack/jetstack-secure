package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/jetstack/preflight/api"
)

type (
	// The UnauthenticatedClient type is a Client implementation used to upload data readings to the Jetstack Secure
	// platform using no authentication method.
	UnauthenticatedClient struct {
		baseURL       string
		agentMetadata *api.AgentMetadata
		client        *http.Client
	}
)

// NewUnauthenticatedClient returns a new instance of the UnauthenticatedClient type that will perform HTTP requests using
// no authentication.
func NewUnauthenticatedClient(agentMetadata *api.AgentMetadata, baseURL string) (*UnauthenticatedClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("cannot create UnauthenticatedClient: baseURL cannot be empty")
	}

	return &UnauthenticatedClient{
		agentMetadata: agentMetadata,
		baseURL:       baseURL,
		client:        &http.Client{Timeout: time.Minute},
	}, nil
}

func (c *UnauthenticatedClient) PostDataReadingsWithOptions(readings []*api.DataReading, opts Options) error {
	return c.PostDataReadings(opts.OrgID, opts.ClusterID, readings)
}

// PostDataReadings uploads the slice of api.DataReading to the Jetstack Secure backend to be processed for later
// viewing in the user-interface.
func (c *UnauthenticatedClient) PostDataReadings(orgID, clusterID string, readings []*api.DataReading) error {
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
		body, err := io.ReadAll(res.Body)
		if err == nil {
			errorContent = string(body)
		}

		return fmt.Errorf("received response with status code %d. Body: [%s]", code, errorContent)
	}

	return nil
}

// Post performs an HTTP POST request.
func (c *UnauthenticatedClient) Post(path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, fullURL(c.baseURL, path), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return c.client.Do(req)
}
