package agent

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"gopkg.in/yaml.v2"
)

// Credentials defines the format of the credentials.json file.
type Credentials struct {
	UserKey       string `json:"user_key"`
	UserKeySecret string `json:"user_key_secret"`
	Server        string `json:"server"`
}

func (c *Credentials) validate() error {
	var result *multierror.Error

	if c.UserKey == "" {
		result = multierror.Append(result, fmt.Errorf("user_key cannot be empty"))
	}

	if c.UserKeySecret == "" {
		result = multierror.Append(result, fmt.Errorf("user_key_secret cannot be empty"))
	}

	if c.Server == "" {
		result = multierror.Append(result, fmt.Errorf("server cannot be empty"))
	}

	return result.ErrorOrNil()
}

// ParseCredentials reads credentials into a struct used. Performs validations.
func ParseCredentials(data []byte) (*Credentials, error) {
	var credentials Credentials

	err := yaml.Unmarshal(data, &credentials)
	if err != nil {
		return nil, err
	}

	if err = credentials.validate(); err != nil {
		return nil, err
	}

	return &credentials, nil
}
