package client

import (
	"fmt"
	"io"
	"net/http"
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

		// Only used with Venafi Cloud. The convention is to use the agent
		// config's `cluster_id` as ClusterName.
		ClusterName string

		// Only used with Venafi Cloud.
		ClusterDescription string
	}

	// The Client interface describes types that perform requests against the Jetstack Secure backend.
	Client interface {
		PostDataReadings(orgID, clusterID string, readings []*api.DataReading) error
		PostDataReadingsWithOptions(readings []*api.DataReading, options Options) error
		Post(path string, body io.Reader) (*http.Response, error)
	}

	// The Credentials interface describes methods for credential types to implement for verification.
	Credentials interface {
		IsClientSet() bool
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
