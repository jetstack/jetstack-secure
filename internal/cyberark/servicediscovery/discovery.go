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
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"

	arkapi "github.com/jetstack/preflight/internal/cyberark/api"
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

	// SecretsManagerServiceName is the name of the Secrets Manager (Conjur
	// Cloud) API in responses from the Service Discovery API. This is the host
	// that serves `authn-jwt/<service-id>/<account>/authenticate` — NOT the
	// identity_administration host. The server that validates the resulting
	// token resolves this same service name.
	SecretsManagerServiceName = "secrets_manager"

	// maxDiscoverBodySize is the maximum allowed size for a response body from the CyberArk Service Discovery subdomain endpoint
	// As of 2025-04-16, a response from the integration environment is ~4kB
	maxDiscoverBodySize = 2 * 1024 * 1024
)

// allowedRootDomains are the only root domains trusted for both (a) the
// discovery bootstrap call itself — c.baseURL, which is ARK_DISCOVERY_API if
// set — and (b) the identity/discoverycontext/secrets_manager hosts that
// call's response points us at. Without (b), mainActiveAPI's ep.API is
// trusted verbatim from the response body and handed straight to the
// Conjur/Identity clients, which then POST the agent's SA token (or
// username/password) to it. Without (a), ARK_DISCOVERY_API write access
// (e.g. a tampered pod spec or Helm values) bootstraps the entire trust
// chain from arbitrary infrastructure regardless of (b) — see CP-26002 /
// CP-23593. Mirrors the per-env ROOT_DOMAIN allowlist already enforced on
// the discoverycontext-regional-resources side (token.py, for the JWT `iss`
// host) — copied by value here since these domains rarely change and the
// agent has no access to that env-keyed map.
//
// Source of truth is the `everest_env_utils` package's ROOT_DOMAIN map
// (published to Artifactory as everest_env_utils_cyberark, v2.0.117 as of
// 2026-09-03), not the Lambda's local commercial-only clone — that clone
// omits the GOV_* environments entirely, which would have made this
// allowlist silently break every gov-cloud tenant's agent.
var allowedRootDomains = []string{
	"cyberark.cloud",
	"cyberark-everest-dev.com",
	"cyberark-everest-test.com",
	"cyberark-everest-stage.com",
	"sandbox-cyberark.cloud",
	"integration-cyberark.cloud",
	"pt-cyberark.cloud",
	"cyberark-everest-integdev.cloud",
	"cyberark-everest-preinteg.cloud",
	"cyberark-everest-perf.cloud",
	"cyberark-everest-pre-prod.cloud",
	// Gov-cloud environments.
	"dev-cyberarkgov.com",
	"test-cyberarkgov.com",
	"stage-cyberarkgov.com",
	"integdev-cyberarkgov.cloud",
	"integration-cyberarkgov.cloud",
	"preprod-cyberarkgov.cloud",
	"dum-preprod-cyberarkgov.cloud",
	"cyberarkgov.cloud",
}

// hostOnAllowedRootDomain reports whether host is, or is a subdomain of, one
// of allowedRootDomains.
//
// DNS is case-insensitive and net/url doesn't normalise host case (it
// lowercases the scheme but not the host — verified), so a discovery
// response with any uppercase in an otherwise-legitimate hostname must
// still match here.
func hostOnAllowedRootDomain(host string) bool {
	host = strings.ToLower(host)
	for _, root := range allowedRootDomains {
		if host == root || strings.HasSuffix(host, "."+root) {
			return true
		}
	}
	return false
}

// hostLeadingLabelMatchesSubdomain reports whether host's leading label
// names subdomain, tolerating the two label shapes actually observed on
// CyberArk API Gateway hosts (mirrors discoverycontext-regional-resources'
// _is_host_subdomain_matching, token.py:197-212, used there to validate the
// inbound Host header against a JWT's subdomain claim):
//   - {subdomain}.{service}.{domain}   e.g. eh1c6a8z1wf8hi.inventory.integration-cyberark.cloud
//   - {subdomain}-{service}.{domain}   e.g. disco4asaf-discoverycontext.integration-cyberark.cloud
//
// Unlike token.py, this doesn't check against one fixed service-name suffix
// (identity/discoverycontext/secrets_manager each render under a different,
// undocumented service label — "id"/"inventory"/"secretsmgr" observed live,
// not the JSON service_name values) — it accepts any hyphen suffix, which is
// looser than token.py's exact match but appropriate for a warn-only check.
func hostLeadingLabelMatchesSubdomain(host, subdomain string) bool {
	if subdomain == "" {
		return true
	}
	label, _, _ := strings.Cut(host, ".")
	return label == subdomain || strings.HasPrefix(label, subdomain+"-")
}

// sanitizeServiceAPI returns rawAPI unchanged if its scheme is https and its
// host is on an allowed root domain, or "" (treated the same as "service not
// present in the response") if not.
//
// subdomain is used only for a warn-only check, not enforcement: see
// hostLeadingLabelMatchesSubdomain's doc comment for why — we don't yet have
// live evidence covering all three services' host shapes across every
// environment, and fail-closed on an unverified assumption risks a real
// outage. CP-26010 tracks turning this into enforcement once telemetry
// confirms it holds.
func sanitizeServiceAPI(ctx context.Context, serviceName, rawAPI, subdomain string) string {
	if rawAPI == "" {
		return ""
	}
	u, err := url.Parse(rawAPI)
	if err != nil || u.Hostname() == "" {
		klog.FromContext(ctx).Info("dropping unparseable service discovery API URL", "service", serviceName, "api", rawAPI)
		return ""
	}
	if u.Scheme != "https" {
		klog.FromContext(ctx).Info("dropping non-HTTPS service discovery API URL", "service", serviceName, "scheme", u.Scheme)
		return ""
	}
	if !hostOnAllowedRootDomain(u.Hostname()) {
		klog.FromContext(ctx).Info("dropping service discovery API URL outside the allowed CyberArk domains", "service", serviceName, "host", u.Hostname())
		return ""
	}
	if !hostLeadingLabelMatchesSubdomain(u.Hostname(), subdomain) {
		// Not dropped -- see the function doc comment. A tampered response
		// could still redirect within the same allowed root domain to a
		// different tenant's host; this is the visibility half of closing
		// that gap, not the enforcement half.
		klog.FromContext(ctx).Info("service discovery API URL's host doesn't look like it belongs to this tenant's subdomain",
			"service", serviceName, "host", u.Hostname(), "subdomain", subdomain)
	}
	return rawAPI
}

// Client is a Golang client for interacting with the CyberArk Discovery Service. It allows
// users to fetch URLs for various APIs available in CyberArk. This client is specialised to
// fetch only API endpoints, since only API endpoints are required by the Venafi Kubernetes Agent currently.
type Client struct {
	client    *http.Client
	baseURL   string
	subdomain string

	cachedResponse      *Services
	cachedTenantID      string
	cachedResponseTime  time.Time
	cachedResponseMutex sync.Mutex
}

// mainActiveAPI returns the API URL of the first active "main" endpoint in
// eps, or "" if there isn't one.
func mainActiveAPI(eps []ServiceEndpoint) string {
	for _, ep := range eps {
		if ep.Type == "main" && ep.IsActive && ep.API != "" {
			return ep.API
		}
	}
	return ""
}

// New creates a new CyberArk Service Discovery client. If the ARK_DISCOVERY_API
// environment variable is set, it is used as the base URL for the service
// discovery API. Otherwise, the production URL is used.
func New(httpClient *http.Client, subdomain string) *Client {
	baseURL := os.Getenv("ARK_DISCOVERY_API")
	if baseURL == "" {
		baseURL = ProdDiscoveryAPIBaseURL
	}

	client := &Client{
		client:    httpClient,
		baseURL:   baseURL,
		subdomain: subdomain,

		cachedResponse:      nil,
		cachedTenantID:      "",
		cachedResponseTime:  time.Time{},
		cachedResponseMutex: sync.Mutex{},
	}

	return client
}

// DiscoveryResponse represents the full JSON response returned by the CyberArk api/tenant-discovery/public API
// The API is documented here https://ca-il-confluence.il.cyber-ark.com/spaces/EV/pages/575618345/Updated+PD+APIs+doc
type DiscoveryResponse struct {
	Region      string         `json:"region"`
	DRRegion    string         `json:"dr_region"`
	Subdomain   string         `json:"subdomain"`
	TenantID    string         `json:"tenant_id"`
	PlatformID  string         `json:"platform_id"`
	IdentityID  string         `json:"identity_id"`
	DefaultURL  string         `json:"default_url"`
	TenantFlags map[string]any `json:"tenant_flags"`
	Services    []Service      `json:"services"`
}

type Service struct {
	ServiceName       string            `json:"service_name"`
	ServiceSubdomains []string          `json:"service_subdomains"`
	Region            string            `json:"region"`
	Endpoints         []ServiceEndpoint `json:"endpoints"`
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

// This is a convenience struct to hold the ServiceEndpoints we care about:
// the Identity API, the Discovery Context API, and the Secrets Manager
// (Conjur Cloud) API used for the authn-jwt token exchange.
type Services struct {
	Identity         ServiceEndpoint
	DiscoveryContext ServiceEndpoint
	SecretsManager   ServiceEndpoint
}

// DiscoverServices fetches from the service discovery service for the configured subdomain
// and parses the CyberArk Identity API URL and Inventory API URL.
// It also returns the Tenant ID UUID corresponding to the subdomain.
func (c *Client) DiscoverServices(ctx context.Context) (*Services, string, error) {
	c.cachedResponseMutex.Lock()
	defer c.cachedResponseMutex.Unlock()

	if c.cachedResponse != nil && time.Since(c.cachedResponseTime) < 1*time.Hour {
		return c.cachedResponse, c.cachedTenantID, nil
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, "", fmt.Errorf("invalid base URL for service discovery: %w", err)
	}

	// CP-26002: the discovery bootstrap call itself must be to an allowed
	// CyberArk domain over HTTPS too — otherwise ARK_DISCOVERY_API write
	// access (e.g. a tampered pod spec or Helm values) would let an attacker
	// point trust-chain bootstrap at arbitrary infrastructure, which the
	// sanitizeServiceAPI checks below wouldn't catch on their own: they used
	// to also accept anything matching this same host, on the assumption
	// that whatever ARK_DISCOVERY_API pointed at was already trusted as much
	// as the discovery call itself. It no longer is assumed; it's checked.
	if u.Scheme != "https" || !hostOnAllowedRootDomain(u.Hostname()) {
		return nil, "", fmt.Errorf("service discovery base URL %q is not HTTPS on an allowed CyberArk domain; refusing to bootstrap trust from it", c.baseURL)
	}

	u.Path = path.Join(u.Path, "api/public/tenant-discovery")
	u.RawQuery = url.Values{"bySubdomain": []string{c.subdomain}}.Encode()

	endpoint := u.String()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to initialise request to %s: %s", endpoint, err)
	}

	request.Header.Set("Accept", "application/json")
	version.SetUserAgent(request)
	// Add telemetry headers
	arkapi.SetTelemetryRequestHeader(request)
	resp, err := c.client.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("failed to perform HTTP request: %s", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// a 404 error is returned with an empty JSON body "{}" if the subdomain is unknown; at the time of writing, we haven't observed
		// any other errors and so we can't special case them
		if resp.StatusCode == http.StatusNotFound {
			return nil, "", fmt.Errorf("got an HTTP 404 response from service discovery; maybe the subdomain %q is incorrect or does not exist?", c.subdomain)
		}

		return nil, "", fmt.Errorf("got unexpected status code %s from request to service discovery API", resp.Status)
	}

	var discoveryResp DiscoveryResponse
	err = json.NewDecoder(io.LimitReader(resp.Body, maxDiscoverBodySize)).Decode(&discoveryResp)
	if err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, "", fmt.Errorf("rejecting JSON response from server as it was too large or was truncated")
		}
		return nil, "", fmt.Errorf("failed to parse JSON from otherwise successful request to service discovery endpoint: %s", err)
	}
	var identityAPI, discoveryContextAPI, secretsManagerAPI string
	for _, svc := range discoveryResp.Services {
		switch svc.ServiceName {
		case IdentityServiceName:
			identityAPI = mainActiveAPI(svc.Endpoints)
		case DiscoveryContextServiceName:
			discoveryContextAPI = mainActiveAPI(svc.Endpoints)
		case SecretsManagerServiceName:
			secretsManagerAPI = mainActiveAPI(svc.Endpoints)
		}
	}

	// Drop any of the three API URLs whose host isn't one of the CyberArk
	// domains we actually trust, before anything downstream authenticates
	// against it. A dropped URL is treated exactly like one absent from the
	// response — see the required/optional distinction below.
	rawIdentityAPI := identityAPI
	identityAPI = sanitizeServiceAPI(ctx, IdentityServiceName, identityAPI, c.subdomain)
	discoveryContextAPI = sanitizeServiceAPI(ctx, DiscoveryContextServiceName, discoveryContextAPI, c.subdomain)
	secretsManagerAPI = sanitizeServiceAPI(ctx, SecretsManagerServiceName, secretsManagerAPI, c.subdomain)

	// identityAPI is required unconditionally, unlike discoveryContextAPI and
	// secretsManagerAPI below: it's present and active for every healthy
	// tenant, so callers may rely on it being non-empty without checking it
	// themselves again.
	if identityAPI == "" {
		if rawIdentityAPI == "" {
			return nil, "", fmt.Errorf("didn't find %s in service discovery response, "+
				"which may indicate a suspended tenant; unable to detect CyberArk Identity API URL", IdentityServiceName)
		}
		// The response did name an identity_administration endpoint, but its
		// host isn't on our allowlist — a distinct, more actionable failure
		// than "suspended tenant" (see sanitizeServiceAPI's Info log for
		// which host was rejected and why).
		return nil, "", fmt.Errorf("%s endpoint %q is not on an allowed CyberArk domain over HTTPS; refusing to use it",
			IdentityServiceName, rawIdentityAPI)
	}
	// discoveryContextAPI and secretsManagerAPI are deliberately not required
	// here, unlike identityAPI above: not every caller needs both, and
	// requiring secretsManagerAPI would break every existing
	// username/password install on a tenant not yet onboarded to Conjur.
	// Each caller that needs one validates it itself — e.g.
	// cyberark.NewDatauploadClient rejects an empty discoveryContextAPI, and
	// cyberark.selectAuthenticator rejects an empty secretsManagerAPI on the
	// Conjur JWT path. Not every caller does this yet (keyfetch's client
	// doesn't check discoveryContextAPI), so an empty value can still surface
	// downstream as a less obvious error.

	services := &Services{
		Identity:         ServiceEndpoint{API: identityAPI},
		DiscoveryContext: ServiceEndpoint{API: discoveryContextAPI},
		SecretsManager:   ServiceEndpoint{API: secretsManagerAPI},
	}

	c.cachedResponse = services
	c.cachedTenantID = discoveryResp.TenantID
	c.cachedResponseTime = time.Now()

	return services, discoveryResp.TenantID, nil
}
