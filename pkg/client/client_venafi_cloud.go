package client

import (
	"bytes"
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/microcosm-cc/bluemonday"
	"k8s.io/client-go/transport"
	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/version"
)

type (
	// The VenafiCloudClient type is a Client implementation used to upload data readings to the Venafi Cloud platform
	// using service account authentication as its authentication method.
	//
	// This form of authentication follows the Private Key JWT standard found at https://oauth.net/private-key-jwt,
	// which is a combination of two RFCs:
	// * RFC 7521 (Assertion Framework)
	// * RFC 7523 (JWT Profile for Client Authentication)
	VenafiCloudClient struct {
		credentials   *VenafiSvcAccountCredentials
		accessToken   *venafiCloudAccessToken
		baseURL       string
		agentMetadata *api.AgentMetadata

		uploaderID    string
		uploadPath    string
		privateKey    crypto.PrivateKey
		jwtSigningAlg jwt.SigningMethod
		lock          sync.RWMutex

		// Made public for testing purposes.
		Client *http.Client
	}

	VenafiSvcAccountCredentials struct {
		// ClientID is the service account client ID
		ClientID string `json:"client_id,omitempty"`
		// PrivateKeyFile is the path to the private key file paired to
		// the public key in the service account
		PrivateKeyFile string `json:"private_key_file,omitempty"`
	}

	venafiCloudAccessToken struct {
		accessToken    string
		expirationTime time.Time
	}

	accessTokenInformation struct {
		AccessToken string `json:"access_token"` // base 64 encoded token
		Type        string `json:"token_type"`   // always be “bearer” for now
		ExpiresIn   int64  `json:"expires_in"`   // number of seconds after which the access token will expire
	}
)

const (
	// URL for the venafi-cloud backend services
	VenafiCloudProdURL               = "https://api.venafi.cloud"
	defaultVenafiCloudUploadEndpoint = "v1/tlspk/uploads"
	accessTokenEndpoint              = "/v1/oauth/token/serviceaccount"
	requiredGrantType                = "urn:ietf:params:oauth:grant-type:jwt-bearer"
)

// NewVenafiCloudClient returns a new instance of the VenafiCloudClient type that will perform HTTP requests using a bearer token
// to authenticate to the backend API.
func NewVenafiCloudClient(agentMetadata *api.AgentMetadata, credentials *VenafiSvcAccountCredentials, baseURL string, uploaderID string, uploadPath string) (*VenafiCloudClient, error) {
	if err := credentials.Validate(); err != nil {
		return nil, fmt.Errorf("cannot create VenafiCloudClient: %w", err)
	}
	privateKey, jwtSigningAlg, err := parsePrivateKeyAndExtractSigningMethod(credentials.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("while parsing private key file: %w", err)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("cannot create VenafiCloudClient: baseURL cannot be empty")
	}

	ok, why := credentials.IsClientSet()
	if !ok {
		return nil, fmt.Errorf("%s", why)
	}

	if uploadPath == "" {
		// if the uploadPath is not given, use default upload path
		uploadPath = defaultVenafiCloudUploadEndpoint
	}

	return &VenafiCloudClient{
		agentMetadata: agentMetadata,
		credentials:   credentials,
		baseURL:       baseURL,
		accessToken:   &venafiCloudAccessToken{},
		Client: &http.Client{
			Timeout:   time.Minute,
			Transport: transport.DebugWrappers(http.DefaultTransport),
		},
		uploaderID:    uploaderID,
		uploadPath:    uploadPath,
		privateKey:    privateKey,
		jwtSigningAlg: jwtSigningAlg,
	}, nil
}

// ParseVenafiCredentials reads credentials into a VenafiSvcAccountCredentials struct. Performs validations.
func ParseVenafiCredentials(data []byte) (*VenafiSvcAccountCredentials, error) {
	var credentials VenafiSvcAccountCredentials

	err := json.Unmarshal(data, &credentials)
	if err != nil {
		return nil, err
	}

	if err = credentials.Validate(); err != nil {
		return nil, err
	}

	return &credentials, nil
}

func (c *VenafiSvcAccountCredentials) Validate() error {
	var result *multierror.Error

	if c == nil {
		return fmt.Errorf("credentials are nil")
	}

	if c.ClientID == "" {
		result = multierror.Append(result, fmt.Errorf("client_id cannot be empty"))
	}

	if c.PrivateKeyFile == "" {
		result = multierror.Append(result, fmt.Errorf("private_key_file cannot be empty"))
	}

	return result.ErrorOrNil()
}

// IsClientSet returns whether the client credentials are set or not. `why` is
// only returned when `ok` is false.
func (c *VenafiSvcAccountCredentials) IsClientSet() (ok bool, why string) {
	if c.ClientID == "" {
		return false, "ClientID is empty"
	}
	if c.PrivateKeyFile == "" {
		return false, "PrivateKeyFile is empty"
	}

	return true, ""
}

// PostDataReadingsWithOptions uploads the slice of api.DataReading to the Venafi Cloud backend to be processed.
// The Options are then passed as URL params in the request
func (c *VenafiCloudClient) PostDataReadingsWithOptions(ctx context.Context, readings []*api.DataReading, opts Options) error {
	payload := api.DataReadingsPost{
		AgentMetadata:  c.agentMetadata,
		DataGatherTime: time.Now().UTC(),
		DataReadings:   readings,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if !strings.HasSuffix(c.uploadPath, "/") {
		c.uploadPath = fmt.Sprintf("%s/", c.uploadPath)
	}

	venafiCloudUploadURL, err := url.Parse(filepath.Join(c.uploadPath, c.uploaderID))
	if err != nil {
		return err
	}

	// validate options and send them as URL params
	query := venafiCloudUploadURL.Query()
	stripHTML := bluemonday.StrictPolicy()
	if opts.ClusterName != "" {
		query.Add("name", stripHTML.Sanitize(opts.ClusterName))
	}
	if opts.ClusterDescription != "" {
		query.Add("description", base64.RawURLEncoding.EncodeToString([]byte(stripHTML.Sanitize(opts.ClusterDescription))))
	}
	venafiCloudUploadURL.RawQuery = query.Encode()

	klog.FromContext(ctx).V(2).Info(
		"uploading data readings",
		"url", venafiCloudUploadURL.String(),
		"cluster_name", opts.ClusterName,
		"data_readings_count", len(readings),
		"data_size_bytes", len(data),
	)

	res, err := c.post(ctx, venafiCloudUploadURL.String(), bytes.NewBuffer(data))
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
func (c *VenafiCloudClient) post(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	token, err := c.getValidAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL(c.baseURL, path), body)
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
// token from the auth server in case the current access token does not exist
// or it is expired.
func (c *VenafiCloudClient) getValidAccessToken(ctx context.Context) (*venafiCloudAccessToken, error) {
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

func (c *VenafiCloudClient) updateAccessToken(ctx context.Context) error {
	jwtToken, err := c.generateAndSignJwtToken()
	if err != nil {
		return err
	}

	values := url.Values{}
	values.Set("grant_type", requiredGrantType)
	values.Set("assertion", jwtToken)

	tokenURL := fullURL(c.baseURL, accessTokenEndpoint)

	encoded := values.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(encoded))
	if err != nil {
		return err
	}

	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Add("Content-Length", strconv.Itoa(len(encoded)))
	version.SetUserAgent(request)

	now := time.Now()
	accessToken := accessTokenInformation{}
	err = c.sendHTTPRequest(request, &accessToken)
	if err != nil {
		return err
	}

	c.lock.Lock()
	c.accessToken = &venafiCloudAccessToken{
		accessToken:    accessToken.AccessToken,
		expirationTime: now.Add(time.Duration(accessToken.ExpiresIn) * time.Second),
	}
	c.lock.Unlock()
	return nil
}

func (c *VenafiCloudClient) sendHTTPRequest(request *http.Request, responseObject any) error {
	response, err := c.Client.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("failed to execute http request to the Control Plane. Request %s, status code: %d, body: [%s]", request.URL, response.StatusCode, body)
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

func (c *VenafiCloudClient) generateAndSignJwtToken() (string, error) {
	prodURL, err := url.Parse(VenafiCloudProdURL)
	if err != nil {
		return "", err
	}

	claims := make(jwt.MapClaims)
	claims["sub"] = c.credentials.ClientID
	claims["iss"] = c.credentials.ClientID
	claims["iat"] = time.Now().Unix()
	claims["exp"] = time.Now().Add(time.Minute).Unix()
	claims["aud"] = path.Join(prodURL.Host, accessTokenEndpoint)
	claims["jti"] = uuid.New().String()

	token, err := jwt.NewWithClaims(c.jwtSigningAlg, claims).SignedString(c.privateKey)
	if err != nil {
		return "", err
	}

	return token, nil
}
