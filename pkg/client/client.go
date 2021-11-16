package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/jetstack/preflight/api"
)

var (
	// ClientID is the auth0 client identifier (injected at build time)
	ClientID string

	// ClientSecret is the auth0 client secret (injected at build time)
	ClientSecret string

	// AuthServerDomain is the auth0 domain (injected at build time)
	AuthServerDomain string
)

type (
	// The Client interface describes types that perform requests against the Jetstack Secure backend.
	Client interface {
		PostDataReadings(orgID, clusterID string, readings []*api.DataReading) error
		Post(path string, body io.Reader) (*http.Response, error)
	}

	// Credentials defines the format of the credentials.json file.
	Credentials struct {
		// UserID is the ID or email for the user or service account.
		UserID string `json:"user_id"`
		// UserSecret is the secret for the user or service account.
		UserSecret string `json:"user_secret"`
		// The following fields are optional as the default behaviour
		// is to use the equivalent variables defined at package level
		// and injected at build time.
		// ClientID is the oauth2 client ID.
		ClientID string `json:"client_id,omitempty"`
		// ClientSecret is the oauth2 client secret.
		ClientSecret string `json:"client_secret,omitempty"`
		// AuthServerDomain is the domain for the auth server.
		AuthServerDomain string `json:"auth_server_domain,omitempty"`
	}
)

// ParseCredentials reads credentials into a struct used. Performs validations.
func ParseCredentials(data []byte) (*Credentials, error) {
	var credentials Credentials

	err := json.Unmarshal(data, &credentials)
	if err != nil {
		return nil, err
	}

	if err = credentials.validate(); err != nil {
		return nil, err
	}

	return &credentials, nil
}

// IsClientSet returns whether the client credentials are set or not.
func (c *Credentials) IsClientSet() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.AuthServerDomain != ""
}

func (c *Credentials) validate() error {
	var result *multierror.Error

	if c == nil {
		return fmt.Errorf("credentials are nil")
	}

	if c.UserID == "" {
		result = multierror.Append(result, fmt.Errorf("user_id cannot be empty"))
	}

	if c.UserSecret == "" {
		result = multierror.Append(result, fmt.Errorf("user_secret cannot be empty"))
	}

	return result.ErrorOrNil()
}

func fullURL(baseURL, path string) string {
	base := baseURL
	for strings.HasSuffix(base, "/") {
		base = strings.TrimSuffix(base, "/")
	}
	for strings.HasPrefix(path, "/") {
		path = strings.TrimPrefix(path, "/")
	}
	return fmt.Sprintf("%s/%s", base, path)
}
