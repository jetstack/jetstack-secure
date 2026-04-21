package client

import (
	"bytes"
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
	"k8s.io/client-go/transport"
	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/version"
)

// NGTSClient is a Client implementation for uploading data readings to NGTS
// using service account keypair authentication. It follows the Private Key JWT
// authentication pattern (RFC 7521 + RFC 7523).
type NGTSClient struct {
	credentials   *NGTSServiceAccountCredentials
	accessToken   *ngtsAccessToken
	baseURL       *url.URL
	agentMetadata *api.AgentMetadata

	tsgID         string
	privateKey    crypto.PrivateKey
	jwtSigningAlg jwt.SigningMethod
	lock          sync.RWMutex

	// Made public for testing purposes.
	Client *http.Client
}

// NGTSServiceAccountCredentials holds the service account authentication credentials for NGTS.
type NGTSServiceAccountCredentials struct {
	// ClientID is the service account client ID
	ClientID string `json:"client_id,omitempty"`
	// PrivateKeyFile is the path to the private key file paired to
	// the public key in the service account
	PrivateKeyFile string `json:"private_key_file,omitempty"`
}

// ngtsAccessToken stores an NGTS access token and its expiration time.
type ngtsAccessToken struct {
	accessToken    string
	expirationTime time.Time
}

// ngtsAccessTokenResponse represents the JSON response from the NGTS token endpoint.
type ngtsAccessTokenResponse struct {
	AccessToken string `json:"access_token"` // base 64 encoded token
	Type        string `json:"token_type"`   // always "bearer"
	ExpiresIn   int64  `json:"expires_in"`   // number of seconds after which the access token will expire
}

const (
	// ngtsProdURLFormat is the format used for constructing a URL for the production environment.
	// The TSG ID is part of the URL.
	ngtsProdURLFormat = "https://%s.ngts.paloaltonetworks.com"

	// ngtsUploadEndpoint matches the "new" CM-SaaS upload endpoint
	// Note that "no" is always passed to this endpoint in other paths (e.g. in the venafi-connection client and in the venafi-kubernetes-agent chart)
	// so we copy that behavior here.
	ngtsUploadEndpoint = "v1/tlspk/upload/clusterdata/no"

	// ngtsAccessTokenEndpoint matches the CM-SaaS token endpoint
	ngtsAccessTokenEndpoint = accessTokenEndpoint

	// ngtsRequiredGrantType matches the CM-SaaS required grant type for JWTs
	ngtsRequiredGrantType = requiredGrantType
)

// NewNGTSClient creates a new NGTS client that authenticates using keypair authentication
// and uploads data to NGTS endpoints. The baseURL parameter can override the default
// NGTS server URL for testing purposes.
func NewNGTSClient(agentMetadata *api.AgentMetadata, credentials *NGTSServiceAccountCredentials, baseURL string, tsgID string, rootCAs *x509.CertPool) (*NGTSClient, error) {
	// Load ClientID from file if not provided directly
	if err := credentials.LoadClientIDIfNeeded(); err != nil {
		return nil, fmt.Errorf("cannot create NGTSClient: %w", err)
	}

	if err := credentials.Validate(); err != nil {
		return nil, fmt.Errorf("cannot create NGTSClient: %w", err)
	}

	// NB: There may be more validation which can be done here, e.g. see
	// https://pan.dev/scm/api/tenancy/delete-tenancy-v-1-tenant-service-groups-tsg-id/
	// > Possible values: >= 10 characters and <= 10 characters, Value must match regular expression ^1[0-9]+$
	// For now, leaving this check simple
	if tsgID == "" {
		return nil, fmt.Errorf("cannot create NGTSClient: tsgID cannot be empty")
	}

	privateKey, jwtSigningAlg, err := parsePrivateKeyAndExtractSigningMethod(credentials.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("while parsing private key file: %w", err)
	}

	actualBaseURL := baseURL

	// Create prod NGTS URL if no explicit URL provided
	if actualBaseURL == "" {
		actualBaseURL = fmt.Sprintf(ngtsProdURLFormat, tsgID)
	}

	parsedBaseURL, err := url.Parse(actualBaseURL)
	if err != nil {
		extra := ""

		// A possible failure mode would be an incorrectly formatted TSG ID, so warn about that specifically
		// if we tried to create a prod URL
		if baseURL == "" {
			extra = fmt.Sprintf(" (possibly malformed TSG ID %q?)", tsgID)
		}

		return nil, fmt.Errorf("invalid NGTS base URL %q: %s%s", baseURL, err, extra)
	}

	// Create HTTP transport that honors proxy settings and custom CA certs
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if rootCAs != nil {
		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = &tls.Config{}
		}
		tr.TLSClientConfig.RootCAs = rootCAs
	}

	return &NGTSClient{
		agentMetadata: agentMetadata,
		credentials:   credentials,
		baseURL:       parsedBaseURL,
		tsgID:         tsgID,
		accessToken:   &ngtsAccessToken{},
		Client: &http.Client{
			Timeout:   time.Minute,
			Transport: transport.DebugWrappers(tr),
		},
		privateKey:    privateKey,
		jwtSigningAlg: jwtSigningAlg,
	}, nil
}

// LoadClientIDIfNeeded attempts to load the ClientID from a file if it is not already set.
// It looks for a "clientID" file in the same directory as the PrivateKeyFile.
// This allows the ClientID to be provided either as a direct value or via a Kubernetes secret.
func (c *NGTSServiceAccountCredentials) LoadClientIDIfNeeded() error {
	if c == nil {
		return fmt.Errorf("credentials are nil")
	}

	// If ClientID is already set via helm values / CLI args, nothing to do
	if c.ClientID != "" {
		klog.V(2).Info("Using clientID from config.clientID helm value")
		return nil
	}

	// We'd preferably have NGTSServiceAccountCredentials.CredentialPath but we didn't want to make another change
	// to existing CLI flags; so we depend on PrivateKeyFile and assume clientID is in the same directory.

	// If PrivateKeyFile is not set, we can't determine where to look for the clientID file
	if c.PrivateKeyFile == "" {
		return nil // This is actually a fatal error but will be caught by Validate() later
	}

	// Try to load ClientID from a file in the same directory as the private key
	clientIDPath := path.Dir(c.PrivateKeyFile) + "/clientID"
	clientIDBytes, err := os.ReadFile(clientIDPath)
	if err != nil {
		// If the file doesn't exist, that's okay - we'll let Validate() catch the empty ClientID error later
		klog.V(2).Info("Could not read clientID from file", "path", clientIDPath, "error", err)
		return nil
	}

	// Trim whitespace from the clientID
	c.ClientID = strings.TrimSpace(string(clientIDBytes))
	klog.V(2).Info("Loaded clientID from file", "path", clientIDPath)

	return nil
}

// Validate checks that the NGTS service account credentials are valid.
func (c *NGTSServiceAccountCredentials) Validate() error {
	if c == nil {
		return fmt.Errorf("credentials are nil")
	}

	if c.ClientID == "" {
		return fmt.Errorf("client_id cannot be empty")
	}

	if c.PrivateKeyFile == "" {
		return fmt.Errorf("NGTS private key file location cannot be empty")
	}

	return nil
}

// PostDataReadingsWithOptions uploads data readings to the NGTS backend.
// The TSG ID is included in the upload path to identify the tenant service group.
func (c *NGTSClient) PostDataReadingsWithOptions(ctx context.Context, readings []*api.DataReading, opts Options) error {
	payload := api.DataReadingsPost{
		AgentMetadata:  c.agentMetadata,
		DataGatherTime: time.Now().UTC(),
		DataReadings:   readings,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	uploadURL := c.baseURL.JoinPath(ngtsUploadEndpoint)

	// Add cluster name and description as query parameters
	query := uploadURL.Query()
	stripHTML := bluemonday.StrictPolicy()
	if opts.ClusterName != "" {
		query.Add("name", stripHTML.Sanitize(opts.ClusterName))
	}

	if opts.ClusterDescription != "" {
		query.Add("description", base64.RawURLEncoding.EncodeToString([]byte(stripHTML.Sanitize(opts.ClusterDescription))))
	}

	uploadURL.RawQuery = query.Encode()

	klog.FromContext(ctx).V(2).Info(
		"uploading data readings to NGTS",
		"url", uploadURL.String(),
		"cluster_name", opts.ClusterName,
		"data_readings_count", len(readings),
		"data_size_bytes", len(data),
	)

	res, err := c.post(ctx, uploadURL.String(), bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to upload data to NGTS: %w", err)
	}
	defer res.Body.Close()

	if code := res.StatusCode; code < 200 || code >= 300 {
		errorContent := ""
		body, err := io.ReadAll(res.Body)
		if err == nil {
			errorContent = string(body)
		}
		return fmt.Errorf("NGTS upload failed with status code %d. Body: [%s]", code, errorContent)
	}

	return nil
}

// post performs an HTTP POST request to NGTS with authentication.
func (c *NGTSClient) post(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	token, err := c.getValidAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	version.SetUserAgent(req)

	if len(token.accessToken) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.accessToken))
	}

	return c.Client.Do(req)
}

// getValidAccessToken returns a valid access token. It will fetch a new access
// token from the auth server if the current token does not exist or has expired.
func (c *NGTSClient) getValidAccessToken(ctx context.Context) (*ngtsAccessToken, error) {
	c.lock.RLock()
	needsUpdate := c.accessToken == nil || time.Now().Add(time.Minute).After(c.accessToken.expirationTime)
	c.lock.RUnlock()

	if needsUpdate {
		err := c.updateAccessToken(ctx)
		if err != nil {
			return nil, err
		}
	}

	c.lock.RLock()
	token := c.accessToken
	c.lock.RUnlock()

	return token, nil
}

// updateAccessToken fetches a new access token from the NGTS auth server using JWT authentication.
func (c *NGTSClient) updateAccessToken(ctx context.Context) error {
	jwtToken, err := c.generateAndSignJwtToken()
	if err != nil {
		return fmt.Errorf("failed to generate JWT token for NGTS authentication: %w", err)
	}

	values := url.Values{}
	values.Set("grant_type", ngtsRequiredGrantType)
	values.Set("assertion", jwtToken)

	tokenURL := c.baseURL.JoinPath(ngtsAccessTokenEndpoint).String()

	encoded := values.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(encoded))
	if err != nil {
		return err
	}

	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Add("Content-Length", strconv.Itoa(len(encoded)))
	version.SetUserAgent(request)

	now := time.Now()
	accessToken := ngtsAccessTokenResponse{}
	err = c.sendHTTPRequest(request, &accessToken)
	if err != nil {
		return fmt.Errorf("failed to obtain NGTS access token: %w", err)
	}

	c.lock.Lock()
	c.accessToken = &ngtsAccessToken{
		accessToken:    accessToken.AccessToken,
		expirationTime: now.Add(time.Duration(accessToken.ExpiresIn) * time.Second),
	}
	c.lock.Unlock()
	return nil
}

// sendHTTPRequest executes an HTTP request and unmarshals the JSON response.
func (c *NGTSClient) sendHTTPRequest(request *http.Request, responseObject any) error {
	response, err := c.Client.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("NGTS API request failed. Request %s, status code: %d, body: [%s]", request.URL, response.StatusCode, body)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(body, responseObject); err != nil {
		return err
	}

	return nil
}

// generateAndSignJwtToken creates a JWT token signed with the service account's private key
// for authenticating to NGTS.
func (c *NGTSClient) generateAndSignJwtToken() (string, error) {
	// backend still expects "api.venafi.cloud/v1/oauth/token/serviceaccount" for audience, so force that for now
	venafiCloudProdURL, err := url.Parse(VenafiCloudProdURL)
	if err != nil {
		return "", err
	}

	claims := make(jwt.MapClaims)
	claims["sub"] = c.credentials.ClientID
	claims["iss"] = c.credentials.ClientID
	claims["iat"] = time.Now().Unix()
	claims["exp"] = time.Now().Add(time.Minute).Unix()
	claims["aud"] = path.Join(venafiCloudProdURL.Host, ngtsAccessTokenEndpoint)
	claims["jti"] = uuid.New().String()

	token, err := jwt.NewWithClaims(c.jwtSigningAlg, claims).SignedString(c.privateKey)
	if err != nil {
		return "", err
	}

	return token, nil
}
