package servicediscovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"k8s.io/client-go/transport"

	"github.com/jetstack/preflight/pkg/version"
)

const (
	ProdDiscoveryEndpoint = "https://platform-discovery.cyberark.cloud/api/v2/"

	// identityServiceName is the name of the identity service we're looking for in responses from the Service Discovery API
	// We were told to use the identity_administration field, not the identity_user_portal field.
	identityServiceName = "identity_administration"

	// maxDiscoverBodySize is the maximum allowed size for a response body from the CyberArk Service Discovery subdomain endpoint
	// As of 2025-04-16, a response from the integration environment is ~4kB
	maxDiscoverBodySize = 2 * 1024 * 1024
)

// Client is a Golang client for interacting with the CyberArk Discovery Service. It allows
// users to fetch URLs for various APIs available in CyberArk. This client is specialised to
// fetch only API endpoints, since only API endpoints are required by the Venafi Kubernetes Agent currently.
type Client struct {
	client   *http.Client
	endpoint string
}

// ClientOpt allows configuration of a Client when using New
type ClientOpt func(*Client)

// WithHTTPClient allows the user to specify a custom HTTP client for the discovery client
func WithHTTPClient(httpClient *http.Client) ClientOpt {
	return func(c *Client) {
		c.client = httpClient
	}
}

// WithCustomEndpoint sets the endpoint to a custom URL without checking that the URL is a CyberArk Service Discovery
// server.
func WithCustomEndpoint(endpoint string) ClientOpt {
	return func(c *Client) {
		c.endpoint = endpoint
	}
}

// New creates a new CyberArk Service Discovery client, configurable with ClientOpt
func New(clientOpts ...ClientOpt) *Client {
	client := &Client{
		client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport.NewDebuggingRoundTripper(http.DefaultTransport, transport.DebugByContext),
		},
		endpoint: ProdDiscoveryEndpoint,
	}

	for _, opt := range clientOpts {
		opt(client)
	}

	return client
}

// DiscoverIdentityAPIURL fetches from the service discovery service for a given subdomain
// and parses the CyberArk Identity API URL.
func (c *Client) DiscoverIdentityAPIURL(ctx context.Context, subdomain string) (string, error) {
	endpoint, err := url.JoinPath(c.endpoint, "services", "subdomain", subdomain)
	if err != nil {
		return "", fmt.Errorf("failed to build a valid URL for subdomain %s; possibly an invalid endpoint: %s", subdomain, err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to initialise request to %s: %s", endpoint, err)
	}

	request.Header.Set("Accept", "application/json")
	version.SetUserAgent(request)

	resp, err := c.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("failed to perform HTTP request: %s", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// a 404 error is returned with an empty JSON body "{}" if the subdomain is unknown; at the time of writing, we haven't observed
		// any other errors and so we can't special case them
		if resp.StatusCode == http.StatusNotFound {
			return "", fmt.Errorf("got an HTTP 404 response from service discovery; maybe the subdomain %q is incorrect or does not exist?", subdomain)
		}

		return "", fmt.Errorf("got unexpected status code %s from request to service discovery API", resp.Status)
	}

	type ServiceEndpoint struct {
		API string `json:"api"`
		// NB: other fields are intentionally ignored here; we only care about the API URL
	}

	decodedResponse := make(map[string]ServiceEndpoint)

	err = json.NewDecoder(io.LimitReader(resp.Body, maxDiscoverBodySize)).Decode(&decodedResponse)
	if err != nil {
		if err == io.ErrUnexpectedEOF {
			return "", fmt.Errorf("rejecting JSON response from server as it was too large or was truncated")
		}

		return "", fmt.Errorf("failed to parse JSON from otherwise successful request to service discovery endpoint: %s", err)
	}

	identityService, ok := decodedResponse[identityServiceName]
	if !ok {
		return "", fmt.Errorf("didn't find %s in service discovery response, which may indicate a suspended tenant; unable to detect CyberArk Identity API URL", identityServiceName)
	}

	return identityService.API, nil
}
