package dgerror

import (
	"fmt"
)

// ConfigError is the error type for a misconfiguration. e.g. A missing CRD
type ConfigError struct {
	Err string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("%s", e.Err)
}
