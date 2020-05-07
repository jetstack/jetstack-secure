package client

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/go-multierror"
)

// Credentials defines the format of the credentials.json file.
type Credentials struct {
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
