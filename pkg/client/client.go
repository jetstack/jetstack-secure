package client

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jetstack/preflight/api"
)

type (
	// The Client interface describes types that perform requests against the Jetstack Secure backend.
	Client interface {
		PostDataReadings(orgID, clusterID string, readings []*api.DataReading) error
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
