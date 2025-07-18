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
)

type CyberArkClient struct {
	baseURL string
	client  *http.Client

	authenticateRequest func(req *http.Request) error
}

type Options struct {
	ClusterName        string
	ClusterDescription string
}

func NewCyberArkClient(trustedCAs *x509.CertPool, baseURL string, authenticateRequest func(req *http.Request) error) (*CyberArkClient, error) {
	cyberClient := &http.Client{}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if trustedCAs != nil {
		tr.TLSClientConfig.RootCAs = trustedCAs
	}
	cyberClient.Transport = transport.DebugWrappers(tr)

	return &CyberArkClient{
		baseURL:             baseURL,
		client:              cyberClient,
		authenticateRequest: authenticateRequest,
	}, nil
}

func (c *CyberArkClient) PostDataReadingsWithOptions(ctx context.Context, payload api.DataReadingsPost, opts Options) error {
	if opts.ClusterName == "" {
		return fmt.Errorf("programmer mistake: the cluster name (aka `cluster_id` in the config file) cannot be left empty")
	}

	encodedBody := &bytes.Buffer{}
	checksum := sha256.New()
	if err := json.NewEncoder(io.MultiWriter(encodedBody, checksum)).Encode(payload); err != nil {
		return err
	}

	presignedUploadURL, err := c.retrievePresignedUploadURL(ctx, hex.EncodeToString(checksum.Sum(nil)), opts)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, presignedUploadURL, encodedBody)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
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
	uploadURL, err := url.JoinPath(c.baseURL, "/api/data/kubernetes/upload")
	if err != nil {
		return "", err
	}

	request := struct {
		ClusterID          string `json:"cluster_id"`
		ClusterDescription string `json:"cluster_description"`
		Checksum           string `json:"checksum_sha256"`
	}{
		ClusterID:          opts.ClusterName,
		ClusterDescription: opts.ClusterDescription,
		Checksum:           checksum,
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
		return "", fmt.Errorf("failed to authenticate request")
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
