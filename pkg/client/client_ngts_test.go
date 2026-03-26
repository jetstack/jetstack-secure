package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jetstack/preflight/api"
)

const fakePrivKeyPEM = `-----BEGIN PRIVATE KEY-----
MHcCAQEEIFptpPXOvEWDrYkiMhyEH1+FB1GwtwX2tyXH4KtBO6g7oAoGCCqGSM49
AwEHoUQDQgAE/BsIwagYc4YUjSSFyqcStj2qliAkdVGlMoJbMuXupzQ9Qs4TX5Pl
dFjz6J/j6Gu4fLPqXmM61Hj6kiuRHx5eHQ==
-----END PRIVATE KEY-----
`

func withFile(t testing.TB, content string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "file")
	if err != nil {
		t.Fatalf("failed to create temporary file: %v", err)
	}
	defer f.Close()

	_, err = f.WriteString(content)
	if err != nil {
		t.Fatalf("failed to write to temporary file: %v", err)
	}

	return f.Name()
}

func TestNewNGTSClient(t *testing.T) {
	// Create a temporary key file
	keyFile := withFile(t, fakePrivKeyPEM)

	tests := []struct {
		name        string
		credentials *NGTSServiceAccountCredentials
		baseURL     string
		tsgID       string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid credentials and tsg id",
			credentials: &NGTSServiceAccountCredentials{
				ClientID:       "test-client-id",
				PrivateKeyFile: keyFile,
			},
			baseURL: "https://test.ngts.example.com",
			tsgID:   "test-tsg-id",
			wantErr: false,
		},
		{
			name: "missing tsg id",
			credentials: &NGTSServiceAccountCredentials{
				ClientID:       "test-client-id",
				PrivateKeyFile: keyFile,
			},
			baseURL:     "https://test.ngts.example.com",
			tsgID:       "",
			wantErr:     true,
			errContains: "tsgID cannot be empty",
		},
		{
			name: "missing clientID without file",
			credentials: &NGTSServiceAccountCredentials{
				ClientID:       "",
				PrivateKeyFile: keyFile,
			},
			baseURL:     "https://test.ngts.example.com",
			tsgID:       "test-tsg-id",
			wantErr:     true,
			errContains: "client_id cannot be empty",
		},
		{
			name: "default URL when empty",
			credentials: &NGTSServiceAccountCredentials{
				ClientID:       "test-client-id",
				PrivateKeyFile: keyFile,
			},
			baseURL: "",
			tsgID:   "test-tsg-id",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &api.AgentMetadata{
				Version:   "test-version",
				ClusterID: "test-cluster",
			}

			client, err := NewNGTSClient(metadata, tt.credentials, tt.baseURL, tt.tsgID, nil)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, client)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, client)
			assert.Equal(t, tt.tsgID, client.tsgID)
			if tt.baseURL != "" {
				assert.Equal(t, tt.baseURL, client.baseURL.String())
				return
			}

			assert.Equal(t, fmt.Sprintf(ngtsProdURLFormat, tt.tsgID), client.baseURL.String())
		})
	}
}

func TestNGTSClient_LoadClientIDFromFile(t *testing.T) {
	// Create a temporary directory for the secret files
	tmpDir := t.TempDir()

	// Create the private key file
	keyFile := tmpDir + "/privatekey.pem"
	err := os.WriteFile(keyFile, []byte(fakePrivKeyPEM), 0600)
	require.NoError(t, err)

	// Create the clientID file in the same directory
	clientIDFile := tmpDir + "/clientID"
	err = os.WriteFile(clientIDFile, []byte("test-client-from-file\n"), 0600)
	require.NoError(t, err)

	tests := []struct {
		name        string
		credentials *NGTSServiceAccountCredentials
		wantErr     bool
		wantClient  string
	}{
		{
			name: "load clientID from file",
			credentials: &NGTSServiceAccountCredentials{
				ClientID:       "", // Empty - should be loaded from file
				PrivateKeyFile: keyFile,
			},
			wantErr:    false,
			wantClient: "test-client-from-file",
		},
		{
			name: "explicit clientID takes precedence",
			credentials: &NGTSServiceAccountCredentials{
				ClientID:       "explicit-client-id",
				PrivateKeyFile: keyFile,
			},
			wantErr:    false,
			wantClient: "explicit-client-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &api.AgentMetadata{
				Version:   "test-version",
				ClusterID: "test-cluster",
			}

			client, err := NewNGTSClient(metadata, tt.credentials, "https://test.example.com", "test-tsg", nil)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, client)
			assert.Equal(t, tt.wantClient, client.credentials.ClientID)
		})
	}
}

func TestNGTSClient_PostDataReadingsWithOptions(t *testing.T) {
	keyFile := withFile(t, fakePrivKeyPEM)

	// Create a test server that simulates NGTS backend
	var receivedRequest *http.Request
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequest = r

		// First request is for access token
		if r.URL.Path == ngtsAccessTokenEndpoint {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(ngtsAccessTokenResponse{
				AccessToken: "test-access-token",
				Type:        "bearer",
				ExpiresIn:   3600,
			})
			return
		}

		// Second request is for data upload
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		receivedBody = body

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "success"}`))
	}))
	defer server.Close()

	credentials := &NGTSServiceAccountCredentials{
		ClientID:       "test-client-id",
		PrivateKeyFile: keyFile,
	}

	metadata := &api.AgentMetadata{
		Version:   "test-version",
		ClusterID: "test-cluster",
	}

	tsgID := "test-tsg-123"
	client, err := NewNGTSClient(metadata, credentials, server.URL, tsgID, nil)
	require.NoError(t, err)

	// Test data upload
	readings := []*api.DataReading{
		{
			DataGatherer: "test-gatherer",
			Timestamp:    api.Time{},
			Data:         &api.DynamicData{},
		},
	}

	opts := Options{
		ClusterName:        "test-cluster",
		ClusterDescription: "Test cluster description",
	}

	err = client.PostDataReadingsWithOptions(context.Background(), readings, opts)
	require.NoError(t, err)

	// Verify the upload request
	assert.NotNil(t, receivedRequest)
	assert.Equal(t, "/"+ngtsUploadEndpoint, receivedRequest.URL.Path)
	assert.Contains(t, receivedRequest.URL.RawQuery, "name=test-cluster")
	assert.Equal(t, "Bearer test-access-token", receivedRequest.Header.Get("Authorization"))

	// Verify the payload
	var payload api.DataReadingsPost
	err = json.Unmarshal(receivedBody, &payload)
	require.NoError(t, err)
	assert.Equal(t, 1, len(payload.DataReadings))
}

func TestNGTSClient_AuthenticationFlow(t *testing.T) {
	keyFile := withFile(t, fakePrivKeyPEM)

	authCallCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == ngtsAccessTokenEndpoint {
			authCallCount++
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(ngtsAccessTokenResponse{
				AccessToken: "test-access-token",
				Type:        "bearer",
				ExpiresIn:   3600,
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	credentials := &NGTSServiceAccountCredentials{
		ClientID:       "test-client-id",
		PrivateKeyFile: keyFile,
	}

	metadata := &api.AgentMetadata{
		Version:   "test-version",
		ClusterID: "test-cluster",
	}

	client, err := NewNGTSClient(metadata, credentials, server.URL, "test-tsg", nil)
	require.NoError(t, err)

	// Make multiple requests - should only authenticate once
	readings := []*api.DataReading{{DataGatherer: "test", Data: &api.DynamicData{}}}
	opts := Options{ClusterName: "test"}

	for range 3 {
		err = client.PostDataReadingsWithOptions(context.Background(), readings, opts)
		require.NoError(t, err)
	}

	// Should only authenticate once since token is cached
	assert.Equal(t, 1, authCallCount)
}

func TestNGTSClient_ErrorHandling(t *testing.T) {
	keyFile := withFile(t, fakePrivKeyPEM)

	tests := []struct {
		name           string
		serverHandler  http.HandlerFunc
		expectedErrMsg string
	}{
		{
			name: "authentication failure",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == ngtsAccessTokenEndpoint {
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte(`{"error": "invalid_client"}`))
					return
				}
				w.WriteHeader(http.StatusOK)
			},
			expectedErrMsg: "failed to obtain NGTS access token",
		},
		{
			name: "upload failure",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == ngtsAccessTokenEndpoint {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(ngtsAccessTokenResponse{
						AccessToken: "test-token",
						Type:        "bearer",
						ExpiresIn:   3600,
					})
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error": "internal server error"}`))
			},
			expectedErrMsg: "NGTS upload failed with status code 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.serverHandler)
			defer server.Close()

			credentials := &NGTSServiceAccountCredentials{
				ClientID:       "test-client-id",
				PrivateKeyFile: keyFile,
			}

			metadata := &api.AgentMetadata{Version: "test", ClusterID: "test"}
			client, err := NewNGTSClient(metadata, credentials, server.URL, "test-tsg", nil)
			require.NoError(t, err)

			readings := []*api.DataReading{{DataGatherer: "test", Data: &api.DynamicData{}}}
			opts := Options{ClusterName: "test"}

			err = client.PostDataReadingsWithOptions(context.Background(), readings, opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrMsg)
		})
	}
}
