package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/jetstack/preflight/api"
)

type (
	// Options is the struct describing additional information pertinent to an agent that isn't a data reading
	// These fields will then be uploaded together with data readings.
	Options struct {
		// Only used with Jetstack Secure.
		OrgID string

		// Only used with Jetstack Secure.
		ClusterID string

		// Used for Venafi Cloud and MachineHub mode.
		ClusterName string

		// Used for Venafi Cloud and MachineHub mode.
		ClusterDescription string
	}

	// The Client interface describes types that perform requests against the Jetstack Secure backend.
	Client interface {
		PostDataReadingsWithOptions(ctx context.Context, readings []*api.DataReading, options Options) error
	}

	// The Credentials interface describes methods for credential types to implement for verification.
	Credentials interface {
		IsClientSet() (ok bool, why string)
		Validate() error
	}
)

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
