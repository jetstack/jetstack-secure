package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/version"
)

// DataReadingsPost is the payload in the upload request.
type DataReadingsPost struct {
	AgentMetadata *AgentMetadata `json:"agent_metadata"`
	// DataGatherTime represents the time that the data readings were gathered
	DataGatherTime time.Time      `json:"data_gather_time"`
	DataReadings   []*DataReading `json:"data_readings"`
}

// DataReading is the output of a DataGatherer.
type DataReading struct {
	// ClusterID is optional as it can be inferred from the agent
	// token when using basic authentication.
	ClusterID     string `json:"cluster_id,omitempty"`
	DataGatherer  string `json:"data-gatherer"`
	Timestamp     Time   `json:"timestamp"`
	Data          any    `json:"data"`
	SchemaVersion string `json:"schema_version"`
}

// UnmarshalJSON implements the json.Unmarshaler interface for DataReading.
// The function attempts to decode the Data field into known types in a prioritized order.
// Empty data is considered an error, because there is no way to discriminate between data types.
// TODO(wallrj): Add a discriminator field to DataReading to avoid this complex logic.
// E.g. "data_type": "discovery"|"dynamic"
func (o *DataReading) UnmarshalJSON(data []byte) error {
	var tmp struct {
		ClusterID     string          `json:"cluster_id,omitempty"`
		DataGatherer  string          `json:"data-gatherer"`
		Timestamp     Time            `json:"timestamp"`
		Data          json.RawMessage `json:"data"`
		SchemaVersion string          `json:"schema_version"`
	}

	// Decode the top-level fields of DataReading
	if err := jsonUnmarshalStrict(data, &tmp); err != nil {
		return fmt.Errorf("failed to parse DataReading: %s", err)
	}

	// Assign top-level fields to the DataReading object
	o.ClusterID = tmp.ClusterID
	o.DataGatherer = tmp.DataGatherer
	o.Timestamp = tmp.Timestamp
	o.SchemaVersion = tmp.SchemaVersion

	// Return an error if data is empty
	if len(tmp.Data) == 0 || bytes.Equal(tmp.Data, []byte("null")) || bytes.Equal(tmp.Data, []byte("{}")) {
		return fmt.Errorf("failed to parse DataReading.Data for gatherer %q: empty data", o.DataGatherer)
	}

	// Define a list of decoding attempts with prioritized types
	dataTypes := []struct {
		target any
		assign func(any)
	}{
		{&OIDCDiscoveryData{}, func(v any) { o.Data = v.(*OIDCDiscoveryData) }},
		{&DiscoveryData{}, func(v any) { o.Data = v.(*DiscoveryData) }},
		{&DynamicData{}, func(v any) { o.Data = v.(*DynamicData) }},
	}

	// Attempt to decode the Data field into each type
	for _, dataType := range dataTypes {
		if err := jsonUnmarshalStrict(tmp.Data, dataType.target); err == nil {
			dataType.assign(dataType.target)
			return nil
		}
	}

	// Return an error if no type matches
	return fmt.Errorf("failed to parse DataReading.Data for gatherer %q: unknown type", o.DataGatherer)
}

// jsonUnmarshalStrict unmarshals JSON data into the provided interface,
// disallowing unknown fields to ensure strict adherence to the expected structure.
func jsonUnmarshalStrict(data []byte, v any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

// GatheredResource wraps the raw k8s resource that is sent to the jetstack secure backend
type GatheredResource struct {
	// Resource is a reference to a k8s object that was found by the informer
	// should be of type unstructured.Unstructured, raw Object
	Resource  any
	DeletedAt Time
}

func (v GatheredResource) MarshalJSON() ([]byte, error) {
	dateString := ""
	if !v.DeletedAt.IsZero() {
		dateString = v.DeletedAt.Format(TimeFormat)
	}

	data := struct {
		Resource  any    `json:"resource"`
		DeletedAt string `json:"deleted_at,omitempty"`
	}{
		Resource:  v.Resource,
		DeletedAt: dateString,
	}

	return json.Marshal(data)
}

func (v *GatheredResource) UnmarshalJSON(data []byte) error {
	var tmpResource struct {
		Resource  *unstructured.Unstructured `json:"resource"`
		DeletedAt Time                       `json:"deleted_at"`
	}

	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()

	if err := d.Decode(&tmpResource); err != nil {
		return err
	}
	v.Resource = tmpResource.Resource
	v.DeletedAt = tmpResource.DeletedAt
	return nil
}

// DynamicData is the DataReading.Data returned by the k8sdynamic.DataGathererDynamic
// gatherer
type DynamicData struct {
	// Items is a list of GatheredResource
	Items []*GatheredResource `json:"items"`
}

// DiscoveryData is the DataReading.Data returned by the k8sdiscovery.DataGathererDiscovery
// gatherer
type DiscoveryData struct {
	// ClusterID is the unique ID of the Kubernetes cluster which this snapshot was taken from.
	// This is sourced from the kube-system namespace UID,
	// which is assumed to be stable for the lifetime of the cluster.
	// - https://github.com/kubernetes/kubernetes/issues/77487#issuecomment-489786023
	ClusterID string `json:"cluster_id"`
	// ServerVersion is the version information of the k8s apiserver
	// See https://godoc.org/k8s.io/apimachinery/pkg/version#Info
	ServerVersion *version.Info `json:"server_version"`
}

// OIDCDiscoveryData is the DataReading.Data returned by the oidc.OIDCDiscovery
// gatherer
type OIDCDiscoveryData struct {
	// OIDCConfig contains OIDC configuration data from the API server's
	// `/.well-known/openid-configuration` endpoint
	OIDCConfig map[string]any `json:"openid_configuration,omitempty"`
	// OIDCConfigError contains any error encountered while fetching the OIDC configuration
	OIDCConfigError string `json:"openid_configuration_error,omitempty"`

	// JWKS contains JWKS data from the API server's `/openid/v1/jwks` endpoint
	JWKS map[string]any `json:"jwks,omitempty"`
	// JWKSError contains any error encountered while fetching the JWKS
	JWKSError string `json:"jwks_error,omitempty"`
}
