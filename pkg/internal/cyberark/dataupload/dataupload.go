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

// The names of Datagatherer configs which have the data to populate the Cyberark Snapshot
const (
	Discovery                   = "k8s-discovery"
	SecretsGatherer             = "k8s/secrets"
	ServiceAccountsGatherer     = "k8s/serviceaccounts"
	RolesGatherer               = "k8s/roles"
	RoleBindingsGatherer        = "k8s/rolebindings"
	ClusterRolesGatherer        = "k8s/clusterroles"
	ClusterRoleBindingsGatherer = "k8s/clusterrolebindings"
)

// ConvertDataReadingsToCyberarkSnapshot converts jetstack-secure DataReadings into Cyberark Snapshot format.
func ConvertDataReadingsToCyberarkSnapshot(
	input api.DataReadingsPost,
) (snapshot Snapshot, err error) {
	var (
		k8sVersion                                    string
		secrets, serviceAccounts, roles, roleBindings []interface{}
	)

	for _, reading := range input.DataReadings {
		switch reading.DataGatherer {
		case Discovery:
			data, ok := reading.Data.(map[string]interface{})
			if !ok {
				return snapshot, fmt.Errorf("failed to convert: %s", reading.DataGatherer)
			}
			serverVersion := data["server_version"]
			serverVersionBytes, err := json.Marshal(serverVersion)
			if err != nil {
				return snapshot, fmt.Errorf("while marshalling server_version: %s", err)
			}
			var serverVersionInfo map[string]string
			if err := json.Unmarshal(serverVersionBytes, &serverVersionInfo); err != nil {
				return snapshot, fmt.Errorf("while un-marshalling server_version bytes: %s", err)
			}
			k8sVersion = serverVersionInfo["gitVersion"]
		case SecretsGatherer:
			if data, ok := reading.Data.(map[string]interface{}); ok {
				if items, ok := data["items"].([]*api.GatheredResource); ok {
					resources := make([]interface{}, len(items))
					for i, resource := range items {
						resources[i] = resource.Resource
					}
					secrets = append(secrets, resources...)
				} else {
					return snapshot, fmt.Errorf("failed to convert: %s", reading.DataGatherer)
				}
			} else {
				return snapshot, fmt.Errorf("failed to convert: %s", reading.DataGatherer)
			}
		case ServiceAccountsGatherer:
			if data, ok := reading.Data.(map[string]interface{}); ok {
				if items, ok := data["items"].([]*api.GatheredResource); ok {
					resources := make([]interface{}, len(items))
					for i, resource := range items {
						resources[i] = resource.Resource
					}
					serviceAccounts = append(serviceAccounts, resources...)
				} else {
					return snapshot, fmt.Errorf("failed to convert: %s", reading.DataGatherer)
				}
			} else {
				return snapshot, fmt.Errorf("failed to convert: %s", reading.DataGatherer)
			}
		case RolesGatherer, ClusterRoleBindingsGatherer:
			if data, ok := reading.Data.(map[string]interface{}); ok {
				if items, ok := data["items"].([]*api.GatheredResource); ok {
					resources := make([]interface{}, len(items))
					for i, resource := range items {
						resources[i] = resource.Resource
					}
					roles = append(roles, resources...)
				} else {
					return snapshot, fmt.Errorf("failed to convert: %s", reading.DataGatherer)
				}
			} else {
				return snapshot, fmt.Errorf("failed to convert: %s", reading.DataGatherer)
			}
		case RoleBindingsGatherer, ClusterRolesGatherer:
			if data, ok := reading.Data.(map[string]interface{}); ok {
				if items, ok := data["items"].([]*api.GatheredResource); ok {
					resources := make([]interface{}, len(items))
					for i, resource := range items {
						resources[i] = resource.Resource
					}
					roleBindings = append(roleBindings, resources...)
				} else {
					return snapshot, fmt.Errorf("failed to convert: %s", reading.DataGatherer)
				}
			} else {
				return snapshot, fmt.Errorf("failed to convert: %s", reading.DataGatherer)
			}
		}
	}

	return Snapshot{
		AgentVersion:    input.AgentMetadata.Version,
		ClusterID:       input.AgentMetadata.ClusterID,
		K8SVersion:      k8sVersion,
		Secrets:         secrets,
		ServiceAccounts: serviceAccounts,
		Roles:           roles,
		RoleBindings:    roleBindings,
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
