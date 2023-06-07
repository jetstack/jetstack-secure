package client

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
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
	"github.com/jetstack/preflight/api"
	"gopkg.in/yaml.v2"
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
		client        *http.Client

		uploadID      string
		uploadPath    string
		privateKey    crypto.PrivateKey
		jwtSigningAlg jwt.SigningMethod
		lock          sync.RWMutex
	}

	VenafiSvcAccountCredentials struct {
		// ClientID is the service account client ID
		ClientID string `yaml:"client_id,omitempty"`
		// PrivateKeyFile is the path to the private key file paired to
		// the public key in the service account
		PrivateKeyFile string `yaml:"private_key_file,omitempty"`
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
	vaasProdURL         = "https://api.venafi.cloud"
	accessTokenEndpoint = "/v1/oauth/token/serviceaccount"
	requiredGrantType   = "urn:ietf:params:oauth:grant-type:jwt-bearer"
)

// NewVenafiCloudClient returns a new instance of the VenafiCloudClient type that will perform HTTP requests using a bearer token
// to authenticate to the backend API.
func NewVenafiCloudClient(agentMetadata *api.AgentMetadata, credentials *VenafiSvcAccountCredentials, baseURL string, uploadID string, uploadPath string) (*VenafiCloudClient, error) {
	if err := credentials.validate(); err != nil {
		return nil, fmt.Errorf("cannot create VenafiCloudClient: %v", err)
	}
	privateKey, jwtSigningAlg, err := parsePrivateKeyAndExtractSigningMethod(credentials.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("error parsing private key file %v", err)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("cannot create VenafiCloudClient: baseURL cannot be empty")
	}

	if !credentials.isClientSet() {
		return nil, fmt.Errorf("cannot create VenafiCloudClient: invalid Venafi Cloud client configuration")
	}

	return &VenafiCloudClient{
		agentMetadata: agentMetadata,
		credentials:   credentials,
		baseURL:       baseURL,
		accessToken:   &venafiCloudAccessToken{},
		client:        &http.Client{Timeout: time.Minute},
		uploadID:      uploadID,
		uploadPath:    uploadPath,
		privateKey:    privateKey,
		jwtSigningAlg: jwtSigningAlg,
	}, nil
}

// ParseVenafiSvcAccountCredentials reads credentials into a struct used. Performs validations.
func ParseVenafiSvcAccountCredentials(data []byte) (*VenafiSvcAccountCredentials, error) {
	var credentials VenafiSvcAccountCredentials

	err := yaml.Unmarshal(data, &credentials)
	if err != nil {
		return nil, err
	}

	if err = credentials.validate(); err != nil {
		return nil, err
	}

	return &credentials, nil
}

func (c *VenafiSvcAccountCredentials) validate() error {
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

// IsClientSet returns whether the client credentials are set or not.
func (c *VenafiSvcAccountCredentials) isClientSet() bool {
	return c.ClientID != "" && c.PrivateKeyFile != ""
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
	res, err := c.Post(filepath.Join(c.uploadPath, c.uploadID), bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if code := res.StatusCode; code < 200 || code >= 300 {
		errorContent := ""
		body, err := ioutil.ReadAll(res.Body)
		if err == nil {
			errorContent = string(body)
		}
		return fmt.Errorf("received response with status code %d. Body: %s", code, errorContent)
	}

	return nil
}

// Post performs an HTTP POST request.
func (c *VenafiCloudClient) Post(path string, body io.Reader) (*http.Response, error) {
	token, err := c.getValidAccessToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fullURL(c.baseURL, path), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	if len(token.accessToken) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.accessToken))
	}

	return c.client.Do(req)
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
	if err != nil {
		return err
	}

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
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}

	defer func() {
		if err = response.Body.Close(); err != nil {
		}
	}()

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("failed to execute http request to VaaS. Request %s, status code: %d, body: %s", request.URL, response.StatusCode, body)
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
	prodURL, err := url.Parse(vaasProdURL)
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
		return nil, fmt.Errorf("error decoding private key from pem file %q", privateKeyFilePath)
	}

	if key, err := x509.ParsePKCS1PrivateKey(der.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der.Bytes); err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey, *ecdsa.PrivateKey, ed25519.PrivateKey:
			return key, nil
		default:
			return nil, fmt.Errorf("found unknown private key type in PKCS#8 wrapping")
		}
	}
	if key, err := x509.ParseECPrivateKey(der.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("failed to parse private key")
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
