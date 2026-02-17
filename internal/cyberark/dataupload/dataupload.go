package dataupload

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"k8s.io/apimachinery/pkg/runtime"

	arkapi "github.com/jetstack/preflight/internal/cyberark/api"
	"github.com/jetstack/preflight/internal/cyberark/identity"
	"github.com/jetstack/preflight/pkg/version"
)

const (
	// maxRetrievePresignedUploadURLBodySize is the maximum allowed size for a response body from the
	// Retrieve Presigned Upload URL service.
	maxRetrievePresignedUploadURLBodySize = 10 * 1024

	// apiPathSnapshotLinks is the URL path of the snapshot-links endpoint of the inventory API.
	// This endpoint returns an AWS presigned URL.
	// TODO(wallrj): Link to CyberArk API documentation when it is published.
	apiPathSnapshotLinks = "/ingestions/kubernetes/snapshot-links"
)

type CyberArkClient struct {
	baseURL    string
	httpClient *http.Client

	tenantUUID string

	authenticateRequest identity.RequestAuthenticator
}

// New creates a new CyberArkClient. The tenant UUID is best sourced from service discovery along with the base URL.
func New(httpClient *http.Client, baseURL string, tenantUUID string, authenticateRequest identity.RequestAuthenticator) *CyberArkClient {
	return &CyberArkClient{
		baseURL:    baseURL,
		httpClient: httpClient,

		tenantUUID: tenantUUID,

		authenticateRequest: authenticateRequest,
	}
}

// Snapshot is the JSON that the CyberArk Discovery and Context API expects to
// be uploaded to the AWS presigned URL.
type Snapshot struct {
	// AgentVersion is the version of the Venafi Kubernetes Agent which is uploading this snapshot.
	AgentVersion string `json:"agent_version"`
	// ClusterID is the unique ID of the Kubernetes cluster which this snapshot was taken from.
	ClusterID string `json:"cluster_id"`
	// ClusterName is the name of the Kubernetes cluster which this snapshot was taken from.
	ClusterName string `json:"cluster_name"`
	// ClusterDescription is an optional description of the Kubernetes cluster which this snapshot was taken from.
	ClusterDescription string `json:"cluster_description,omitempty"`
	// K8SVersion is the version of Kubernetes which the cluster is running.
	K8SVersion string `json:"k8s_version"`
	// OIDCConfig contains OIDC configuration data from the API server's
	// `/.well-known/openid-configuration` endpoint
	OIDCConfig map[string]any `json:"openid_configuration,omitempty"`
	// OIDCConfigError contains any error encountered while fetching the OIDC configuration
	OIDCConfigError string `json:"openid_configuration_error,omitempty"`
	// JWKS contains JWKS data from the API server's `/openid/v1/jwks` endpoint
	JWKS map[string]any `json:"jwks,omitempty"`
	// JWKSError contains any error encountered while fetching the JWKS
	JWKSError string `json:"jwks_error,omitempty"`
	// Secrets is a list of Secret resources in the cluster. Not all Secret
	// types are included and only a subset of the Secret data is included.
	Secrets []runtime.Object `json:"secrets"`
	// ServiceAccounts is a list of ServiceAccount resources in the cluster.
	ServiceAccounts []runtime.Object `json:"serviceaccounts"`
	// ConfigMaps is a list of ConfigMap resources in the cluster.
	ConfigMaps []runtime.Object `json:"configmaps"`
	// ExternalSecrets is a list of ExternalSecret resources in the cluster.
	ExternalSecrets []runtime.Object `json:"externalsecrets"`
	// SecretStores is a list of SecretStore resources in the cluster.
	SecretStores []runtime.Object `json:"secretstores"`
	// ClusterExternalSecrets is a list of ClusterExternalSecret resources in the cluster.
	ClusterExternalSecrets []runtime.Object `json:"clusterexternalsecrets"`
	// ClusterSecretStores is a list of ClusterSecretStore resources in the cluster.
	ClusterSecretStores []runtime.Object `json:"clustersecretstores"`
	// Roles is a list of Role resources in the cluster.
	Roles []runtime.Object `json:"roles"`
	// ClusterRoles is a list of ClusterRole resources in the cluster.
	ClusterRoles []runtime.Object `json:"clusterroles"`
	// RoleBindings is a list of RoleBinding resources in the cluster.
	RoleBindings []runtime.Object `json:"rolebindings"`
	// ClusterRoleBindings is a list of ClusterRoleBinding resources in the cluster.
	ClusterRoleBindings []runtime.Object `json:"clusterrolebindings"`
	// Jobs is a list of Job resources in the cluster.
	Jobs []runtime.Object `json:"jobs"`
	// CronJobs is a list of CronJob resources in the cluster.
	CronJobs []runtime.Object `json:"cronjobs"`
	// Deployments is a list of Deployment resources in the cluster.
	Deployments []runtime.Object `json:"deployments"`
	// Statefulsets is a list of StatefulSet resources in the cluster.
	Statefulsets []runtime.Object `json:"statefulsets"`
	// Daemonsets is a list of DaemonSet resources in the cluster.
	Daemonsets []runtime.Object `json:"daemonsets"`
	// Pods is a list of Pod resources in the cluster.
	Pods []runtime.Object `json:"pods"`
}

// PutSnapshot PUTs the supplied snapshot to an [AWS presigned URL] which it obtains via the CyberArk inventory API.
// [AWS presigned URL]: https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-query-string-auth.html
//
// A SHA256 checksum header is included in the request, to verify that the payload
// has been received intact.
// Read [Checking object integrity for data uploads in Amazon S3](https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity-upload.html),
// to learn more.
func (c *CyberArkClient) PutSnapshot(ctx context.Context, snapshot Snapshot) error {
	if snapshot.ClusterID == "" {
		return fmt.Errorf("programmer mistake: the snapshot cluster ID cannot be left empty")
	}

	encodedBody := &bytes.Buffer{}
	hash := sha256.New()
	if err := json.NewEncoder(io.MultiWriter(encodedBody, hash)).Encode(snapshot); err != nil {
		return err
	}

	checksum := hash.Sum(nil)
	checksumHex := hex.EncodeToString(checksum)
	checksumBase64 := base64.StdEncoding.EncodeToString(checksum)

	presignedUploadURL, username, err := c.retrievePresignedUploadURL(ctx, checksumHex, snapshot.ClusterID, int64(encodedBody.Len()))
	if err != nil {
		return fmt.Errorf("while retrieving snapshot upload URL: %s", err)
	}

	// The snapshot-links endpoint returns an AWS presigned URL which only supports the PUT verb.
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignedUploadURL, encodedBody)
	if err != nil {
		return err
	}

	req.Header.Set("X-Amz-Checksum-Sha256", checksumBase64)
	req.Header.Set("X-Amz-Server-Side-Encryption", "AES256")

	q := url.Values{}

	q.Add("agent_version", snapshot.AgentVersion)
	q.Add("tenant_id", c.tenantUUID)
	q.Add("upload_type", "k8s_snapshot")
	q.Add("uploader_id", snapshot.ClusterID)
	q.Add("username", username)
	q.Add("vendor", "k8s")

	req.Header.Set("X-Amz-Tagging", q.Encode())

	version.SetUserAgent(req)

	res, err := c.httpClient.Do(req)
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

// RetrievePresignedUploadURLRequest is the JSON body sent to the inventory API to request a presigned upload URL.
type RetrievePresignedUploadURLRequest struct {
	ClusterID string `json:"cluster_id"`
	Checksum  string `json:"checksum_sha256"`

	// AgentVersion is the v-prefixed version of the agent uploading the snapshot.
	// Note that the backend relies on this version being v-prefixed semver.
	AgentVersion string `json:"agent_version"`

	// FileSize is the size of the data we'll upload in bytes
	FileSize int64 `json:"file_size"`
}

func (c *CyberArkClient) retrievePresignedUploadURL(ctx context.Context, checksum string, clusterID string, fileSize int64) (string, string, error) {
	uploadURL, err := url.JoinPath(c.baseURL, apiPathSnapshotLinks)
	if err != nil {
		return "", "", err
	}

	request := RetrievePresignedUploadURLRequest{
		ClusterID:    clusterID,
		Checksum:     checksum,
		AgentVersion: version.PreflightVersion,
		FileSize:     fileSize,
	}

	encodedBody := &bytes.Buffer{}
	if err := json.NewEncoder(encodedBody).Encode(request); err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, encodedBody)
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Content-Type", "application/json")

	username, err := c.authenticateRequest(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to authenticate request: %s", err)
	}

	version.SetUserAgent(req)

	// Add telemetry headers
	arkapi.SetTelemetryRequestHeader(req)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()

	if code := res.StatusCode; code < 200 || code >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 500))
		if len(body) == 0 {
			body = []byte(`<empty body>`)
		}
		return "", "", fmt.Errorf("received response with status code %d: %s", code, bytes.TrimSpace(body))
	}

	response := struct {
		URL string `json:"url"`
	}{}

	if err := json.NewDecoder(io.LimitReader(res.Body, maxRetrievePresignedUploadURLBodySize)).Decode(&response); err != nil {
		if err == io.ErrUnexpectedEOF {
			return "", "", fmt.Errorf("rejecting JSON response from server as it was too large or was truncated")
		}

		return "", "", fmt.Errorf("failed to parse JSON from otherwise successful request to start data upload: %s", err)
	}

	return response.URL, username, nil
}
