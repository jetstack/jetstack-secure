package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jetstack/preflight/api"
)

// A 401 on upload must drop the cached token so the next upload re-authenticates,
// rather than resending the rejected token until it expires on its own.
func TestVenafiCloudClient_ReauthenticatesOn401(t *testing.T) {
	keyFile := withFile(t, fakePrivKeyPEM)

	var tokenRequests, uploadRequests int
	failNextUpload := true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == accessTokenEndpoint {
			tokenRequests++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(accessTokenInformation{
				AccessToken: fmt.Sprintf("token-%d", tokenRequests),
				Type:        "bearer",
				ExpiresIn:   3600,
			})
			return
		}

		uploadRequests++
		if failNextUpload {
			failNextUpload = false
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`unauthorized`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	credentials := &VenafiSvcAccountCredentials{ClientID: "test-client-id", PrivateKeyFile: keyFile}
	metadata := &api.AgentMetadata{Version: "test-version", ClusterID: "test-cluster"}

	client, err := NewVenafiCloudClient(metadata, credentials, server.URL, "uploader-1", "")
	require.NoError(t, err)

	readings := []*api.DataReading{{DataGatherer: "g", Timestamp: api.Time{}, Data: &api.DynamicData{}}}
	opts := Options{ClusterName: "test-cluster"}

	// First upload is rejected with 401. The token (still far from expiry) must be dropped.
	err = client.PostDataReadingsWithOptions(t.Context(), readings, opts)
	require.Error(t, err)
	require.Equal(t, 1, tokenRequests, "one token minted for the first upload")

	// Second upload: the dropped token forces a fresh authentication, so it succeeds.
	// Without the 401 handling the cached token would be reused (no second token request).
	err = client.PostDataReadingsWithOptions(t.Context(), readings, opts)
	require.NoError(t, err)
	require.Equal(t, 2, tokenRequests, "the 401 forced a re-authentication")
	require.Equal(t, 2, uploadRequests)
}
