package dataupload

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/jetstack/preflight/pkg/version"

	_ "embed"
)

const (
	successBearerToken = "success-token"

	successClusterID = "success-cluster-id"
)

type mockDataUploadServer struct {
	Server *httptest.Server
}

// MockDataUploadServer returns a mocked data upload server with default values.
func MockDataUploadServer() *mockDataUploadServer {
	mds := &mockDataUploadServer{}
	mds.Server = httptest.NewTLSServer(mds)
	return mds
}

func (mds *mockDataUploadServer) Close() {
	mds.Server.Close()
}

func (mds *mockDataUploadServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case apiPathSnapshotLinks:
		mds.handleSnapshotLinks(w, r)
		return
	case "/presigned-upload":
		mds.handlePresignedUpload(w, r)
		return
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (mds *mockDataUploadServer) handleSnapshotLinks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"message":"method not allowed"}`))
		return
	}

	if r.Header.Get("User-Agent") != version.UserAgent() {
		http.Error(w, "should set user agent on all requests", http.StatusInternalServerError)
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

	decoder := json.NewDecoder(r.Body)
	var req struct {
		ClusterID    string `json:"cluster_id"`
		Checksum     string `json:"checksum_sha256"`
		AgentVersion string `json:"agent_version"`
	}
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

	// Write response body
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	presignedURL := mds.Server.URL + "/presigned-upload"
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
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"message":"method not allowed"}`))
		return
	}

	if r.Header.Get("User-Agent") != version.UserAgent() {
		http.Error(w, "should set user agent on all requests", http.StatusInternalServerError)
		return
	}

	amzChecksum := r.Header.Get("X-Amz-Checksum-Sha256")
	if amzChecksum == "" {
		http.Error(w, "should set x-amz-checksum-sha256 header on all requests", http.StatusInternalServerError)
		return
	}

	checksum := sha256.New()
	_, _ = io.Copy(checksum, r.Body)

	// AWS S3 responds with a BadDigest error if the request body has a
	// different checksum than the checksum supplied in the request header.
	if amzChecksum != base64.StdEncoding.EncodeToString(checksum.Sum(nil)) {
		w.Header().Set("Content-Type", "application/xml")
		http.Error(w, amzExampleChecksumError, http.StatusBadRequest)
	}
	// AWS S3 responds with an empty body if the PUT succeeds
	w.WriteHeader(http.StatusOK)
}
