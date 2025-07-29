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

type CyberArkClient struct {
	baseURL string
	client  *http.Client

	authenticateRequest func(req *http.Request) error
}

type Options struct {
	ClusterName string
}

type CyberarkPayload struct {
	AgentVersion    string        `json:"agent_version"`
	K8sVersion      string        `json:"k8s_version"`
	ClusterID       string        `json:"cluster_id"`
	Secrets         []interface{} `json:"secrets"`          // kubectl output, sanitized
	ServiceAccounts []interface{} `json:"service_accounts"` // kubectl output
	Roles           []interface{} `json:"roles"`            // k8s native format
	RoleBindings    []interface{} `json:"role_bindings"`    // k8s native format
}

// You may want to define these constants based on your data-gatherer names
const (
	Discovery                   = "k8s-discovery"
	SecretsGatherer             = "k8s/secrets"
	ServiceAccountsGatherer     = "k8s/serviceaccounts"
	RolesGatherer               = "k8s/roles"
	RoleBindingsGatherer        = "k8s/rolebindings"
	ClusterRolesGatherer        = "k8s/clusterroles"
	ClusterRoleBindingsGatherer = "k8s/clusterrolebindings"
)

// ConvertDataReadingsToCyberarkPayload converts jetstack-secure DataReadings into CyberarkPayload
func ConvertDataReadingsToCyberarkPayload(
	input api.DataReadingsPost,
) CyberarkPayload {
	var (
		k8sVersion                                    string
		secrets, serviceAccounts, roles, roleBindings []interface{}
	)

	for _, reading := range input.DataReadings {
		switch reading.DataGatherer {
		case Discovery:
			data, ok := reading.Data.(map[string]interface{})
			if !ok {
				panic("failed to parse server version")
			}
			serverVersion := data["server_version"]
			serverVersionBytes, err := json.Marshal(serverVersion)
			if err != nil {
				panic(err)
			}
			var serverVersionInfo map[string]string
			if err := json.Unmarshal(serverVersionBytes, &serverVersionInfo); err != nil {
				panic(err)
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
				}
			}
		case ServiceAccountsGatherer:
			if data, ok := reading.Data.(map[string]interface{}); ok {
				if items, ok := data["items"].([]*api.GatheredResource); ok {
					resources := make([]interface{}, len(items))
					for i, resource := range items {
						resources[i] = resource.Resource
					}
					serviceAccounts = append(serviceAccounts, resources...)
				}
			}
		case RolesGatherer, ClusterRoleBindingsGatherer:
			if data, ok := reading.Data.(map[string]interface{}); ok {
				if items, ok := data["items"].([]*api.GatheredResource); ok {
					resources := make([]interface{}, len(items))
					for i, resource := range items {
						resources[i] = resource.Resource
					}
					roles = append(roles, resources...)
				}
			}
		case RoleBindingsGatherer, ClusterRolesGatherer:
			if data, ok := reading.Data.(map[string]interface{}); ok {
				if items, ok := data["items"].([]*api.GatheredResource); ok {
					resources := make([]interface{}, len(items))
					for i, resource := range items {
						resources[i] = resource.Resource
					}
					roleBindings = append(roleBindings, resources...)
				}
			}
		}
	}

	return CyberarkPayload{
		AgentVersion:    input.AgentMetadata.Version,
		K8sVersion:      k8sVersion,
		ClusterID:       input.AgentMetadata.ClusterID,
		Secrets:         secrets,
		ServiceAccounts: serviceAccounts,
		Roles:           roles,
		RoleBindings:    roleBindings,
	}
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

	encodedBody := &bytes.Buffer{}
	checksum := sha256.New()
	if err := json.NewEncoder(io.MultiWriter(encodedBody, checksum)).Encode(ConvertDataReadingsToCyberarkPayload(payload)); err != nil {
		return err
	}

	presignedUploadURL, err := c.retrievePresignedUploadURL(ctx, hex.EncodeToString(checksum.Sum(nil)), opts)
	if err != nil {
		return fmt.Errorf("while retrieving presigned upload URL: %v", err)
	}

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
