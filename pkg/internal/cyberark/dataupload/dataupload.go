package dataupload

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"k8s.io/client-go/transport"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/internal/cyberark"
	"github.com/jetstack/preflight/pkg/version"
)

const (
	// maxRetrievePresignedUploadURLBodySize is the maximum allowed size for a response body from the
	// Retrieve Presigned Upload URL service.
	maxRetrievePresignedUploadURLBodySize = 10 * 1024
)

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
// [AWS presigned URL]: https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-query-string-auth.html
//
// A SHA256 checksum header is included in the request, to verify that the payload
// has been received intact.
// Read [Checking object integrity for data uploads in Amazon S3](https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity-upload.html),
// to learn more.
//
// TODO(wallrj): There is a bug in the AWS backend:
// [S3 Presigned PutObjectCommand URLs ignore Sha256 Hash when uploading](https://github.com/aws/aws-sdk/issues/480)
// ...which means that the `x-amz-checksum-sha256` request header is optional.
// If you omit that header, it is possible to PUT any data.
// There is a work around listed in that issue which we have shared with the
// CyberArk API team.
func (c *CyberArkClient) PostDataReadingsWithOptions(ctx context.Context, payload api.DataReadingsPost, opts Options) error {
	if opts.ClusterName == "" {
		return fmt.Errorf("programmer mistake: the cluster name (aka `cluster_id` in the config file) cannot be left empty")
	}

	encodedBody := &bytes.Buffer{}
	hash := sha256.New()
	if err := json.NewEncoder(io.MultiWriter(encodedBody, hash)).Encode(payload); err != nil {
		return err
	}
	checksum := hash.Sum(nil)
	checksumHex := hex.EncodeToString(checksum)
	checksumBase64 := base64.StdEncoding.EncodeToString(checksum)
	presignedUploadURL, err := c.retrievePresignedUploadURL(ctx, checksumHex, opts)
	if err != nil {
		return fmt.Errorf("while retrieving snapshot upload URL: %s", err)
	}

	// The snapshot-links endpoint returns an AWS presigned URL which only supports the PUT verb.
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignedUploadURL, encodedBody)
	if err != nil {
		return err
	}
	req.Header.Set("X-Amz-Checksum-Sha256", checksumBase64)
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
	uploadURL, err := url.JoinPath(c.baseURL, cyberark.EndpointSnapshotLinks)
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
