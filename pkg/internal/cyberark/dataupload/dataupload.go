package dataupload

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"k8s.io/client-go/transport"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"github.com/jetstack/preflight/pkg/version"
)

const (
	// maxRetrievePresignedUploadURLBodySize is the maximum allowed size for a response body from the
	// Retrieve Presigned Upload URL service.
	maxRetrievePresignedUploadURLBodySize = 10 * 1024

	// apiPathSnapshotLinks is the URL path of the snapshot-links endpoint of the inventory API.
	// This endpoint returns an AWS presigned URL.
	// TODO(wallrj): Link to CyberArk API documentation when it is published.
	apiPathSnapshotLinks = "/api/ingestions/kubernetes/snapshot-links"
)

type ResourceData map[string][]interface{}

// Snapshot is the JSON that the CyberArk Discovery and Context API expects to
// be uploaded to the AWS presigned URL.
type Snapshot struct {
	AgentVersion    string        `json:"agent_version"`
	ClusterID       string        `json:"cluster_id"`
	K8SVersion      string        `json:"k8s_version"`
	Secrets         []interface{} `json:"secrets"`
	ServiceAccounts []interface{} `json:"service_accounts"`
	Roles           []interface{} `json:"roles"`
	RoleBindings    []interface{} `json:"role_bindings"`
}

// The names of Datagatherers which have the data to populate the Cyberark Snapshot mapped to the key in the Cyberark snapshot.
var gathererNameToresourceDataKeyMap = map[string]string{
	"k8s/secrets":             "secrets",
	"k8s/serviceaccounts":     "serviceaccounts",
	"k8s/roles":               "roles",
	"k8s/clusterroles":        "roles",
	"k8s/rolebindings":        "rolebindings",
	"k8s/clusterrolebindings": "rolebindings",
}

func extractResourceListFromReading(reading *api.DataReading) ([]interface{}, error) {
	data, ok := reading.Data.(*k8s.DynamicData)
	if !ok {
		return nil, fmt.Errorf("failed to convert data: %s", reading.DataGatherer)
	}
	items := data.Items
	resources := make([]interface{}, len(items))
	for i, resource := range items {
		resources[i] = resource.Resource
	}
	return resources, nil
}

func extractServerVersionFromReading(reading *api.DataReading) (string, error) {
	data, ok := reading.Data.(*k8s.DiscoveryData)
	if !ok {
		return "", fmt.Errorf("failed to convert data: %s", reading.DataGatherer)
	}
	if data.ServerVersion == nil {
		return "unknown", nil
	}
	return data.ServerVersion.GitVersion, nil
}

// ConvertDataReadingsToCyberarkSnapshot converts jetstack-secure DataReadings into Cyberark Snapshot format.
func ConvertDataReadingsToCyberarkSnapshot(
	input api.DataReadingsPost,
) (_ *Snapshot, err error) {
	k8sVersion := ""
	resourceData := ResourceData{}
	for _, reading := range input.DataReadings {
		if reading.DataGatherer == "k8s-discovery" {
			k8sVersion, err = extractServerVersionFromReading(reading)
			if err != nil {
				return nil, fmt.Errorf("while extracting server version from data-reading: %s", err)
			}
		}
		if key, found := gathererNameToresourceDataKeyMap[reading.DataGatherer]; found {
			var resources []interface{}
			resources, err = extractResourceListFromReading(reading)
			if err != nil {
				return nil, fmt.Errorf("while extracting resource list from data-reading: %s", err)
			}
			resourceData[key] = append(resourceData[key], resources...)
		}
	}

	return &Snapshot{
		AgentVersion:    input.AgentMetadata.Version,
		ClusterID:       input.AgentMetadata.ClusterID,
		K8SVersion:      k8sVersion,
		Secrets:         resourceData["secrets"],
		ServiceAccounts: resourceData["serviceaccounts"],
		Roles:           resourceData["roles"],
		RoleBindings:    resourceData["rolebindings"],
	}, nil
}

type CyberArkClient struct {
	baseURL string
	client  *http.Client

	authenticateRequest func(req *http.Request) error
}

type Options struct {
	ClusterName string
}

func NewCyberArkClient(trustedCAs *x509.CertPool, baseURL string, authenticateRequest func(req *http.Request) error) (*CyberArkClient, error) {
	cyberClient := &http.Client{}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if trustedCAs != nil {
		tr.TLSClientConfig.RootCAs = trustedCAs
	}
	cyberClient.Transport = transport.NewDebuggingRoundTripper(tr, transport.DebugByContext)

	return &CyberArkClient{
		baseURL:             baseURL,
		client:              cyberClient,
		authenticateRequest: authenticateRequest,
	}, nil
}

// PostDataReadingsWithOptions PUTs the supplied payload to an [AWS presigned URL] which it obtains via the CyberArk inventory API.
//
// [AWS presigned URL]: https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-query-string-auth.html
func (c *CyberArkClient) PostDataReadingsWithOptions(ctx context.Context, payload api.DataReadingsPost, opts Options) error {
	if opts.ClusterName == "" {
		return fmt.Errorf("programmer mistake: the cluster name (aka `cluster_id` in the config file) cannot be left empty")
	}

	snapshot, err := ConvertDataReadingsToCyberarkSnapshot(payload)
	if err != nil {
		return fmt.Errorf("while converting datareadings to Cyberark snapshot format: %s", err)
	}

	encodedBody := &bytes.Buffer{}
	checksum := sha256.New()
	if err := json.NewEncoder(io.MultiWriter(encodedBody, checksum)).Encode(snapshot); err != nil {
		return err
	}

	presignedUploadURL, err := c.retrievePresignedUploadURL(ctx, hex.EncodeToString(checksum.Sum(nil)), opts)
	if err != nil {
		return fmt.Errorf("while retrieving snapshot upload URL: %s", err)
	}

	// The snapshot-links endpoint returns an AWS presigned URL which only supports the PUT verb.
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignedUploadURL, encodedBody)
	if err != nil {
		return err
	}

	version.SetUserAgent(req)

	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if code := res.StatusCode; code < 200 || code >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 500))
		if len(body) == 0 {
			body = []byte(`<empty body>`)
		}
		return fmt.Errorf("received response with status code %d: %s", code, bytes.TrimSpace(body))
	}

	return nil
}

func (c *CyberArkClient) retrievePresignedUploadURL(ctx context.Context, checksum string, opts Options) (string, error) {
	uploadURL, err := url.JoinPath(c.baseURL, apiPathSnapshotLinks)
	if err != nil {
		return "", err
	}

	request := struct {
		ClusterID    string `json:"cluster_id"`
		Checksum     string `json:"checksum_sha256"`
		AgentVersion string `json:"agent_version"`
	}{
		ClusterID:    opts.ClusterName,
		Checksum:     checksum,
		AgentVersion: version.PreflightVersion,
	}

	encodedBody := &bytes.Buffer{}
	if err := json.NewEncoder(encodedBody).Encode(request); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, encodedBody)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if err := c.authenticateRequest(req); err != nil {
		return "", fmt.Errorf("failed to authenticate request: %s", err)
	}
	version.SetUserAgent(req)

	res, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if code := res.StatusCode; code < 200 || code >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 500))
		if len(body) == 0 {
			body = []byte(`<empty body>`)
		}
		return "", fmt.Errorf("received response with status code %d: %s", code, bytes.TrimSpace(body))
	}

	response := struct {
		URL string `json:"url"`
	}{}

	if err := json.NewDecoder(io.LimitReader(res.Body, maxRetrievePresignedUploadURLBodySize)).Decode(&response); err != nil {
		if err == io.ErrUnexpectedEOF {
			return "", fmt.Errorf("rejecting JSON response from server as it was too large or was truncated")
		}

		return "", fmt.Errorf("failed to parse JSON from otherwise successful request to start data upload: %s", err)
	}

	return response.URL, nil
}
