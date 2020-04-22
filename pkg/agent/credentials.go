package agent

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
}

func (c *Credentials) validate() error {
	var result *multierror.Error

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
