package dataupload

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/transport"

	arkapi "github.com/jetstack/preflight/internal/cyberark/api"
	"github.com/jetstack/preflight/pkg/version"
)

const (
	successBearerToken = "success-token"

	successClusterID = "ffffffff-ffff-ffff-ffff-ffffffffffff"
)

type uploadValues struct {
	ClusterID string
	FileSize  int64
}

type mockDataUploadServer struct {
	t         testing.TB
	serverURL string

	mux *http.ServeMux

	expectedUploadValues      map[string]uploadValues
	expectedUploadValuesMutex sync.Mutex
}

// MockDataUploadServer starts a server which mocks the CyberArk
// Discovery and Context API, and an HTTP client with the CA certs needed to
// connect to it.
//
// The returned URL can be supplied to the `dataupload.New` function as the base
// URL for the discoverycontext API.
//
// The returned HTTP client has a transport which logs requests and responses
// depending on log level of the logger supplied in the context.
//
// The mock server will return a successful response when the cluster ID matches
// successClusterID. Other cluster IDs can be used to trigger various failure
// responses.
func MockDataUploadServer(t testing.TB) (string, *http.Client) {
	mux := http.NewServeMux()
	mds := &mockDataUploadServer{
		t: t,

		expectedUploadValues: make(map[string]uploadValues),
	}

	mux.HandleFunc("POST "+apiPathSnapshotLinks, mds.handleSnapshotLinks)

	// The path includes random data to ensure that each request is treated separately by the mock server, allowing us to track data across calls.
	// It also ensures that the client isn't using some pre-saved path and is actually using the presigned URL returned by the mock server in the previous step, which is important for test validity.
	mux.HandleFunc("PUT /presigned-upload/{randData}", mds.handlePresignedUpload)

	server := httptest.NewTLSServer(mds)
	t.Cleanup(server.Close)

	mds.mux = mux
	mds.serverURL = server.URL

	httpClient := server.Client()
	httpClient.Transport = transport.NewDebuggingRoundTripper(httpClient.Transport, transport.DebugByContext)
	return server.URL, httpClient
}

func (mds *mockDataUploadServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mds.t.Log(r.Method, r.RequestURI)

	mds.mux.ServeHTTP(w, r)
}

// randHex reads 8 random bytes and returns them as a hex string. It is used to generate
// unique paths per-request to ensure that file size is tracked across calls.
func randHex() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		panic("failed to read random bytes: " + err.Error())
	}

	return hex.EncodeToString(b)
}

func (mds *mockDataUploadServer) handleSnapshotLinks(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("User-Agent") != version.UserAgent() {
		http.Error(w, "should set user agent on all requests", http.StatusInternalServerError)
		return
	}

	if r.Header.Get(arkapi.TelemetryHeaderKey) == "" {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("should set telemetry header on all requests"))
		return
	}

	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "should send JSON on all requests", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("Authorization") != "Bearer "+successBearerToken {
		http.Error(w, "should authenticate using the correct bearer token", http.StatusInternalServerError)
		return
	}

	var req RetrievePresignedUploadURLRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request format"}`, http.StatusBadRequest)
		return
	}

	if req.AgentVersion != version.PreflightVersion {
		http.Error(w, fmt.Sprintf("post body contains unexpected agent version: %s", req.AgentVersion), http.StatusInternalServerError)
		return
	}

	// Simulate invalid JSON response for RetrievePresignedUploadURL step
	if req.ClusterID == "invalid-json-retrieve-presigned" {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"url":`)) // invalid JSON
		return
	}

	// Simulate invalid JSON response for RetrievePresignedUploadURL step
	if req.ClusterID == "invalid-response-post-data" {
		http.Error(w, "mock error", http.StatusInternalServerError)
		return
	}

	if req.ClusterID != successClusterID {
		http.Error(w, "post body contains cluster ID", http.StatusInternalServerError)
		return
	}

	if req.FileSize <= 0 {
		http.Error(w, "file size must be greater than 0", http.StatusInternalServerError)
		return
	}

	randomData := randHex()

	mds.expectedUploadValuesMutex.Lock()
	defer mds.expectedUploadValuesMutex.Unlock()

	uploadValues := uploadValues{
		ClusterID: req.ClusterID,
		FileSize:  req.FileSize,
	}

	mds.expectedUploadValues[randomData] = uploadValues

	presignedURL, err := url.JoinPath(mds.serverURL, "presigned-upload", randomData)
	if err != nil {
		http.Error(w, "failed to generate presigned URL", http.StatusInternalServerError)
		mds.t.Logf("failed to generate presigned URL: %v", err)
		return
	}

	// Write response body
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(struct {
		URL string `json:"url"`
	}{presignedURL})
}

// An example of a real checksum mismatch error from the AWS API when the
// request body does not match the checksum in the request header.
const amzExampleChecksumError = `<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>BadDigest</Code>
  <Message>The SHA256 you specified did not match the calculated checksum.</Message>
  <RequestId>THR2V1RX700Z8SC7</RequestId>
  <HostId>F0xSC0H93Xs0BlCx6RjasZgrtjNkNB7lF4+yz1AiPQHswpdEoqj3iTgEN8SUWgV2Qm/laPobVIMz9SYTNHqdoA==</HostId>
</Error>`

func (mds *mockDataUploadServer) handlePresignedUpload(w http.ResponseWriter, r *http.Request) {
	randData := r.PathValue("randData")
	if randData == "" {
		http.Error(w, "missing randData in path; should match that returned in presigned url", http.StatusInternalServerError)
		return
	}

	mds.expectedUploadValuesMutex.Lock()
	uploadValues, ok := mds.expectedUploadValues[randData]
	mds.expectedUploadValuesMutex.Unlock()

	if !ok {
		http.Error(w, "didn't find a prior call to generate presigned URL", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("User-Agent") != version.UserAgent() {
		http.Error(w, "should set user agent on all requests", http.StatusInternalServerError)
		return
	}

	if r.Header.Get(arkapi.TelemetryHeaderKey) != "" {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("should NOT set telemetry header on requests to presigned URL"))
		return
	}

	amzChecksum := r.Header.Get("X-Amz-Checksum-Sha256")
	if amzChecksum == "" {
		http.Error(w, "should set x-amz-checksum-sha256 header on all requests", http.StatusInternalServerError)
		return
	}

	sseHeader := r.Header.Get("X-Amz-Server-Side-Encryption")
	if sseHeader != "AES256" {
		http.Error(w, "should set x-amz-server-side-encryption header to AES256 on all requests", http.StatusInternalServerError)
		return
	}

	taggingHeader := r.Header.Get("X-Amz-Tagging")
	if taggingHeader == "" {
		http.Error(w, "should set x-amz-tagging header on all requests", http.StatusInternalServerError)
		return
	}

	tags, err := url.ParseQuery(taggingHeader)
	if err != nil {
		http.Error(w, "x-amz-tagging header should be encoded as a valid query string", http.StatusInternalServerError)
		return
	}

	if tags.Get("agent_version") != version.PreflightVersion {
		http.Error(w, fmt.Sprintf("x-amz-tagging should contain an agent_version tag with value %s", version.PreflightVersion), http.StatusInternalServerError)
		return
	}

	if tags.Get("tenant_id") == "" {
		// TODO: if we change setup a bit, we can check the tenant_id matches the expected tenant_id from the test config, but for now, just check it's set
		http.Error(w, "x-amz-tagging should contain a tenant_id tag", http.StatusInternalServerError)
		return
	}

	if tags.Get("upload_type") != "k8s_snapshot" {
		http.Error(w, "x-amz-tagging should contain an upload_type tag with value k8s_snapshot", http.StatusInternalServerError)
		return
	}

	if tags.Get("uploader_id") != uploadValues.ClusterID {
		http.Error(w, "x-amz-tagging should contain an uploader_id tag which matches the cluster ID sent in the RetrievePresignedUploadURL request", http.StatusInternalServerError)
		return
	}

	if tags.Get("username") == "" {
		// TODO: if we change setup a bit, we can check the username matches the expected username from the test config
		// but for now, just check it's set
		http.Error(w, "x-amz-tagging should contain a username tag", http.StatusInternalServerError)
		return
	}

	if tags.Get("vendor") != "k8s" {
		http.Error(w, "x-amz-tagging should contain a vendor tag with value k8s", http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	require.NoError(mds.t, err)

	if uploadValues.FileSize != int64(len(body)) {
		http.Error(w, fmt.Sprintf("file size in request body should match that sent in RetrievePresignedUploadURL request; expected %d, got %d", uploadValues.FileSize, len(body)), http.StatusInternalServerError)
		return
	}

	hash := sha256.New()
	_, err = hash.Write(body)
	require.NoError(mds.t, err)

	// AWS S3 responds with a BadDigest error if the request body has a
	// different checksum than the checksum supplied in the request header.
	if amzChecksum != base64.StdEncoding.EncodeToString(hash.Sum(nil)) {
		w.Header().Set("Content-Type", "application/xml")
		http.Error(w, amzExampleChecksumError, http.StatusBadRequest)
	}

	// Verifies that the new Snapshot format is used in the request body.
	var snapshot Snapshot
	d := json.NewDecoder(bytes.NewBuffer(body))
	d.DisallowUnknownFields()
	err = d.Decode(&snapshot)
	require.NoError(mds.t, err)
	assert.Equal(mds.t, successClusterID, snapshot.ClusterID)
	assert.Equal(mds.t, version.PreflightVersion, snapshot.AgentVersion)

	// AWS S3 responds with an empty body if the PUT succeeds
	w.WriteHeader(http.StatusOK)
}
