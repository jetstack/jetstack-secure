package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"k8s.io/client-go/transport"
	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/version"
)

type (
	// The APITokenClient type is a Client implementation used to upload data readings to the Jetstack Secure platform
	// using API tokens as its authentication method.
	APITokenClient struct {
		apiToken      string
		baseURL       string
		agentMetadata *api.AgentMetadata
		client        *http.Client
	}
)

// NewAPITokenClient returns a new instance of the APITokenClient type that will perform HTTP requests using
// the provided API token for authentication.
func NewAPITokenClient(agentMetadata *api.AgentMetadata, apiToken, baseURL string) (*APITokenClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("cannot create APITokenClient: baseURL cannot be empty")
	}

	return &APITokenClient{
		apiToken:      apiToken,
		agentMetadata: agentMetadata,
		baseURL:       baseURL,
		client: &http.Client{
			Timeout:   time.Minute,
			Transport: transport.DebugWrappers(http.DefaultTransport),
		},
	}, nil
}

// PostDataReadingsWithOptions uploads the slice of api.DataReading to the Jetstack Secure backend to be processed for later
// viewing in the user-interface.
func (c *APITokenClient) PostDataReadingsWithOptions(ctx context.Context, readings []*api.DataReading, opts Options) error {
	return c.postDataReadings(ctx, opts.OrgID, opts.ClusterID, readings)
}

// PostDataReadings uploads the slice of api.DataReading to the Jetstack Secure backend to be processed for later
// viewing in the user-interface.
func (c *APITokenClient) postDataReadings(ctx context.Context, orgID, clusterID string, readings []*api.DataReading) error {
	payload := api.DataReadingsPost{
		AgentMetadata:  c.agentMetadata,
		DataGatherTime: time.Now().UTC(),
		DataReadings:   readings,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	klog.FromContext(ctx).V(2).Info(
		"uploading data readings",
		"url", filepath.Join("/api/v1/org", orgID, "datareadings", clusterID),
		"cluster_id", clusterID,
		"data_readings_count", len(readings),
		"data_size_bytes", len(data),
	)

	res, err := c.post(ctx, filepath.Join("/api/v1/org", orgID, "datareadings", clusterID), bytes.NewBuffer(data))
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
func (c *APITokenClient) post(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL(c.baseURL, path), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))
	version.SetUserAgent(req)

	return c.client.Do(req)
}
