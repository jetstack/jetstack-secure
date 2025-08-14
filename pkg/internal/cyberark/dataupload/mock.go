package dataupload

import (
	"bytes"
	"crypto/sha3"
	"encoding/hex"
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
		mds.handlePresignedUpload(w, r)
		return
	case "/presigned-upload":
		mds.handleUpload(w, r)
		return
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (mds *mockDataUploadServer) handlePresignedUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, err := w.Write([]byte(`{"message":"method not allowed"}`))
		if err != nil {
			panic(err)
		}
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
		Checksum     string `json:"checksum_sha3"`
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
		_, err := w.Write([]byte(`{"url":`)) // invalid JSON
		if err != nil {
			panic(err)
		}
		return
	}

	// Simulate invalid JSON response for RetrievePresignedUploadURL step
	if req.ClusterID == "invalid-response-post-data" {
		http.Error(w, "mock error", http.StatusInternalServerError)
		return
	}

	if req.ClusterID != successClusterID {
		http.Error(w, "post body does not contain cluster ID", http.StatusInternalServerError)
		return
	}

	// Write response body
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	presignedURL := mds.Server.URL + "/presigned-upload?checksum=" + req.Checksum
	_ = json.NewEncoder(w).Encode(struct {
		URL string `json:"url"`
	}{presignedURL})
}

func (mds *mockDataUploadServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, err := w.Write([]byte(`{"message":"method not allowed"}`))
		if err != nil {
			panic(err)
		}
		return
	}

	if r.Header.Get("User-Agent") != version.UserAgent() {
		http.Error(w, "should set user agent on all requests", http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	checksum := sha3.New256()
	_, err = checksum.Write(body)
	if err != nil {
		panic(err)
	}
	if r.URL.Query().Get("checksum") != hex.EncodeToString(checksum.Sum(nil)) {
		http.Error(w, "checksum is invalid", http.StatusInternalServerError)
	}

	var snapshot snapshot
	d := json.NewDecoder(bytes.NewBuffer(body))
	d.DisallowUnknownFields()
	if err := d.Decode(&snapshot); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusOK)
}
