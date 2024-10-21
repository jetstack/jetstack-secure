package client

import (
	"bytes"
	"compress/gzip"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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

	"github.com/jetstack/preflight/api"
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

		disableCompression bool

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
		AccessToken string `json:"access_token"` //base 64 encoded token
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
func NewVenafiCloudClient(agentMetadata *api.AgentMetadata, credentials *VenafiSvcAccountCredentials, baseURL string, uploaderID string, uploadPath string, disableCompression bool) (*VenafiCloudClient, error) {
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
		agentMetadata:      agentMetadata,
		credentials:        credentials,
		baseURL:            baseURL,
		accessToken:        &venafiCloudAccessToken{},
		Client:             &http.Client{Timeout: time.Minute},
		uploaderID:         uploaderID,
		uploadPath:         uploadPath,
		privateKey:         privateKey,
		jwtSigningAlg:      jwtSigningAlg,
		disableCompression: disableCompression,
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
func (c *VenafiCloudClient) PostDataReadingsWithOptions(readings []*api.DataReading, opts Options) error {
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

	res, err := c.Post(venafiCloudUploadURL.String(), bytes.NewBuffer(data))
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

// PostDataReadings uploads the slice of api.DataReading to the Venafi Cloud backend to be processed for later
// viewing in the user-interface.
func (c *VenafiCloudClient) PostDataReadings(_ string, _ string, readings []*api.DataReading) error {
	// orgID and clusterID are ignored in Venafi Cloud auth

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
	res, err := c.Post(filepath.Join(c.uploadPath, c.uploaderID), bytes.NewBuffer(data))
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
func (c *VenafiCloudClient) Post(path string, body io.Reader) (*http.Response, error) {
	token, err := c.getValidAccessToken()
	if err != nil {
		return nil, err
	}

	var encodedBody io.Reader
	if c.disableCompression {
		encodedBody = body
	} else {
		compressed := new(bytes.Buffer)
		gz := gzip.NewWriter(compressed)
		if _, err := io.Copy(gz, body); err != nil {
			return nil, err
		}
		err := gz.Close()
		if err != nil {
			return nil, err
		}
		encodedBody = compressed
	}

	req, err := http.NewRequest(http.MethodPost, fullURL(c.baseURL, path), encodedBody)
	if err != nil {
		return nil, err
	}

	// We have noticed that NGINX, which is Venafi Control Plane's API gateway,
	// has a limit on the request body size we can send (client_max_body_size).
	// On large clusters, the agent may exceed this limit, triggering the error
	// "413 Request Entity Too Large". Although this limit has been raised to
	// 1GB, NGINX still buffers the requests that the agent sends because
	// proxy_request_buffering isn't set to off. To reduce the strain on NGINX'
	// memory and disk, to avoid further 413s, and to avoid reaching the maximum
	// request body size of customer's proxies, we have decided to enable GZIP
	// compression. Ref: https://venafi.atlassian.net/browse/VC-36434.
	if !c.disableCompression {
		req.Header.Set("Content-Encoding", "gzip")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	if len(token.accessToken) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.accessToken))
	}

	return c.Client.Do(req)
}

// getValidAccessToken returns a valid access token. It will fetch a new access
// token from the auth server in case the current access token does not exist
// or it is expired.
func (c *VenafiCloudClient) getValidAccessToken() (*venafiCloudAccessToken, error) {
	if c.accessToken == nil || time.Now().Add(time.Minute).After(c.accessToken.expirationTime) {
		err := c.updateAccessToken()
		if err != nil {
			return nil, err
		}
	}

	return c.accessToken, nil
}

func (c *VenafiCloudClient) updateAccessToken() error {
	jwtToken, err := c.generateAndSignJwtToken()
	if err != nil {
		return err
	}

	values := url.Values{}
	values.Set("grant_type", requiredGrantType)
	values.Set("assertion", jwtToken)

	tokenURL := fullURL(c.baseURL, accessTokenEndpoint)

	encoded := values.Encode()
	request, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(encoded))
	if err != nil {
		return err
	}

	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Add("Content-Length", strconv.Itoa(len(encoded)))

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

func (c *VenafiCloudClient) sendHTTPRequest(request *http.Request, responseObject interface{}) error {
	response, err := c.Client.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("failed to execute http request to Venafi Control Plane. Request %s, status code: %d, body: [%s]", request.URL, response.StatusCode, body)
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

func parsePrivateKeyFromPemFile(privateKeyFilePath string) (crypto.PrivateKey, error) {
	pkBytes, err := os.ReadFile(privateKeyFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Venafi Cloud authentication private key %q: %s",
			privateKeyFilePath, err)
	}

	der, _ := pem.Decode(pkBytes)
	if der == nil {
		return nil, fmt.Errorf("while decoding the PEM-encoded private key %v, its content were: %s", privateKeyFilePath, string(pkBytes))
	}

	if key, err := x509.ParsePKCS1PrivateKey(der.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der.Bytes); err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey, *ecdsa.PrivateKey, ed25519.PrivateKey:
			return key, nil
		default:
			return nil, fmt.Errorf("found unknown private key type in PKCS#8 wrapping: %T", key)
		}
	}
	if key, err := x509.ParseECPrivateKey(der.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("while parsing EC private: %w", err)
}

func parsePrivateKeyAndExtractSigningMethod(privateKeyFile string) (crypto.PrivateKey, jwt.SigningMethod, error) {

	privateKey, err := parsePrivateKeyFromPemFile(privateKeyFile)
	if err != nil {
		return nil, nil, err
	}

	var signingMethod jwt.SigningMethod
	switch key := privateKey.(type) {
	case *rsa.PrivateKey:
		bitLen := key.N.BitLen()
		switch bitLen {
		case 2048:
			signingMethod = jwt.SigningMethodRS256
		case 3072:
			signingMethod = jwt.SigningMethodRS384
		case 4096:
			signingMethod = jwt.SigningMethodRS512
		default:
			signingMethod = jwt.SigningMethodRS256
		}
	case *ecdsa.PrivateKey:
		bitLen := key.Curve.Params().BitSize
		switch bitLen {
		case 256:
			signingMethod = jwt.SigningMethodES256
		case 384:
			signingMethod = jwt.SigningMethodES384
		case 521:
			signingMethod = jwt.SigningMethodES512
		default:
			signingMethod = jwt.SigningMethodES256
		}
	case ed25519.PrivateKey:
		signingMethod = jwt.SigningMethodEdDSA
	default:
		err = fmt.Errorf("unsupported private key type")
	}
	return privateKey, signingMethod, err
}
