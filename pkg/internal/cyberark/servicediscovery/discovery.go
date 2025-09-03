package servicediscovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/jetstack/preflight/pkg/version"
)

const (
	// ProdDiscoveryAPIBaseURL is the base URL for the production CyberArk Service Discovery API
	ProdDiscoveryAPIBaseURL = "https://platform-discovery.cyberark.cloud/"

	// IdentityServiceName is the name of the identity service we're looking for in responses from the Service Discovery API
	// We were told to use the identity_administration field, not the identity_user_portal field.
	IdentityServiceName = "identity_administration"

	// DiscoveryContextServiceName is the name of the discovery and context API
	// in responses from the Service Discovery API.
	DiscoveryContextServiceName = "discoverycontext"

	// maxDiscoverBodySize is the maximum allowed size for a response body from the CyberArk Service Discovery subdomain endpoint
	// As of 2025-04-16, a response from the integration environment is ~4kB
	maxDiscoverBodySize = 2 * 1024 * 1024
)

// Client is a Golang client for interacting with the CyberArk Discovery Service. It allows
// users to fetch URLs for various APIs available in CyberArk. This client is specialised to
// fetch only API endpoints, since only API endpoints are required by the Venafi Kubernetes Agent currently.
type Client struct {
	client  *http.Client
	baseURL string
}

// New creates a new CyberArk Service Discovery client. If the ARK_DISCOVERY_API
// environment variable is set, it is used as the base URL for the service
// discovery API. Otherwise, the production URL is used.
func New(httpClient *http.Client) *Client {
	baseURL := os.Getenv("ARK_DISCOVERY_API")
	if baseURL == "" {
		baseURL = ProdDiscoveryAPIBaseURL
	}
	client := &Client{
		client:  httpClient,
		baseURL: baseURL,
	}

	return client
}

// DiscoveryResponse represents the full JSON response returned by the CyberArk api/tenant-discovery/public API
// The API is documented here https://ca-il-confluence.il.cyber-ark.com/spaces/EV/pages/575618345/Updated+PD+APIs+doc
type DiscoveryResponse struct {
	Region      string                 `json:"region"`
	DRRegion    string                 `json:"dr_region"`
	Subdomain   string                 `json:"subdomain"`
	TenantID    string                 `json:"tenant_id"`
	PlatformID  string                 `json:"platform_id"`
	IdentityID  string                 `json:"identity_id"`
	DefaultURL  string                 `json:"default_url"`
	TenantFlags map[string]interface{} `json:"tenant_flags"`
	Services    []Service              `json:"services"`
}

type Service struct {
	ServiceName string            `json:"service_name"`
	Region      string            `json:"region"`
	Endpoints   []ServiceEndpoint `json:"endpoints"`
}

// ServiceEndpoint represents a single service endpoint returned by the CyberArk
// Service Discovery API. The JSON field names here must match the field names
// returned by the Service Discovery API.
type ServiceEndpoint struct {
	IsActive bool   `json:"is_active"`
	Type     string `json:"type"`
	UI       string `json:"ui"`
	API      string `json:"api"`
}

// This is a convenience struct to hold the two ServiceEndpoints we care about.
// Currently, we only care about the Identity API and the Discovery Context API.
type Services struct {
	Identity         ServiceEndpoint
	DiscoveryContext ServiceEndpoint
}

// DiscoverServices fetches from the service discovery service for a given subdomain
// and parses the CyberArk Identity API URL and Inventory API URL.
func (c *Client) DiscoverServices(ctx context.Context, subdomain string) (*Services, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL for service discovery: %w", err)
	}
	u.Path = path.Join(u.Path, "api/public/tenant-discovery")
	u.RawQuery = url.Values{"bySubdomain": []string{subdomain}}.Encode()
	endpoint := u.String()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise request to %s: %s", endpoint, err)
	}

	request.Header.Set("Accept", "application/json")
	version.SetUserAgent(request)

	resp, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to perform HTTP request: %s", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// a 404 error is returned with an empty JSON body "{}" if the subdomain is unknown; at the time of writing, we haven't observed
		// any other errors and so we can't special case them
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("got an HTTP 404 response from service discovery; maybe the subdomain %q is incorrect or does not exist?", subdomain)
		}

		return nil, fmt.Errorf("got unexpected status code %s from request to service discovery API", resp.Status)
	}

	var discoveryResp DiscoveryResponse
	err = json.NewDecoder(io.LimitReader(resp.Body, maxDiscoverBodySize)).Decode(&discoveryResp)
	if err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("rejecting JSON response from server as it was too large or was truncated")
		}
		return nil, fmt.Errorf("failed to parse JSON from otherwise successful request to service discovery endpoint: %s", err)
	}
	var identityAPI, discoveryContextAPI string
	for _, svc := range discoveryResp.Services {
		switch svc.ServiceName {
		case IdentityServiceName:
			for _, ep := range svc.Endpoints {
				if ep.Type == "main" && ep.IsActive && ep.API != "" {
					identityAPI = ep.API
					break
				}
			}
		case DiscoveryContextServiceName:
			for _, ep := range svc.Endpoints {
				if ep.Type == "main" && ep.IsActive && ep.API != "" {
					discoveryContextAPI = ep.API
					break
				}
			}
		}
	}

	if identityAPI == "" {
		return nil, fmt.Errorf("didn't find %s in service discovery response, "+
			"which may indicate a suspended tenant; unable to detect CyberArk Identity API URL", IdentityServiceName)
	}
	//TODO: Should add a check for discoveryContextAPI too?

	return &Services{
		Identity:         ServiceEndpoint{API: identityAPI},
		DiscoveryContext: ServiceEndpoint{API: discoveryContextAPI},
	}, nil
}
