package api

import (
	"encoding/base64"
	"net/http"
	"net/url"

	"github.com/jetstack/preflight/pkg/version"
)

// Integrations working with the Identity Security Platform, should add metadata
// in their API calls, to provide insights into how customers utilize each API.
//
// - IntegrationName (in): The vendor integration name (required)
// - IntegrationType (it): Integration Type	(required)
// - IntegrationVersion (iv): The plugin version being used (required)
// - VendorName (vn): Vendor name (required)
// - VendorVersion (vv): Version of the vendor product in which the plugin is used (if applicable)

const (
	// TelemetryHeaderKey is the name of the HTTP header to use for telemetry
	TelemetryHeaderKey = "X-Cybr-Telemetry"
)

var (
	telemetryValues       url.Values
	telemetryValueEncoded string
)

func init() {
	telemetryValues = url.Values{}
	telemetryValues.Set("in", "cyberark-disco-agent")
	telemetryValues.Set("vn", "CyberArk")
	telemetryValues.Set("it", "KubernetesAgent")
	telemetryValues.Set("iv", version.PreflightVersion)
	telemetryValueEncoded = base64.URLEncoding.EncodeToString([]byte(telemetryValues.Encode()))
}

// SetTelemetryRequestHeader adds the x-cybr-telemetry header to the given HTTP
// request, with information about this integration.
func SetTelemetryRequestHeader(req *http.Request) {
	req.Header.Set(TelemetryHeaderKey, telemetryValueEncoded)
}
