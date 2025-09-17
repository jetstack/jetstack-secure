package api

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// Test the SetTelemetryRequestHeader function
func TestSetTelemetryRequestHeader(t *testing.T) {
	// Create a new HTTP request
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com", nil)
	require.NoError(t, err, "failed to create HTTP request")

	// Call the function to set the telemetry header
	SetTelemetryRequestHeader(req)

	base64Value := req.Header.Get(TelemetryHeaderKey)
	// Check that the header is set
	require.NotEmpty(t, base64Value, "telemetry header should be set")

	queryString, err := base64.URLEncoding.DecodeString(base64Value)
	require.NoError(t, err, "failed to decode telemetry header value")

	values, err := url.ParseQuery(string(queryString))
	require.NoError(t, err, "failed to parse telemetry header value")
	require.Equal(t, telemetryValues, values, "telemetry header value should match expected values")
}
