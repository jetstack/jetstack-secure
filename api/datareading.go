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
	ClusterID     string      `json:"cluster_id,omitempty"`
	DataGatherer  string      `json:"data-gatherer"`
	Timestamp     Time        `json:"timestamp"`
	Data          interface{} `json:"data"`
	SchemaVersion string      `json:"schema_version"`
}

// UnmarshalJSON implements the json.Unmarshaler interface for DataReading.
// It handles the dynamic parsing of the Data field based on the DataGatherer.
func (o *DataReading) UnmarshalJSON(data []byte) error {
	var tmp struct {
		ClusterID     string          `json:"cluster_id,omitempty"`
		DataGatherer  string          `json:"data-gatherer"`
		Timestamp     Time            `json:"timestamp"`
		Data          json.RawMessage `json:"data"`
		SchemaVersion string          `json:"schema_version"`
	}

	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()

	if err := d.Decode(&tmp); err != nil {
		return err
	}
	o.ClusterID = tmp.ClusterID
	o.DataGatherer = tmp.DataGatherer
	o.Timestamp = tmp.Timestamp
	o.SchemaVersion = tmp.SchemaVersion

	{
		var discoveryData DiscoveryData
		d := json.NewDecoder(bytes.NewReader(tmp.Data))
		d.DisallowUnknownFields()
		if err := d.Decode(&discoveryData); err == nil {
			o.Data = &discoveryData
			return nil
		}
	}
	{
		var dynamicData DynamicData
		d := json.NewDecoder(bytes.NewReader(tmp.Data))
		d.DisallowUnknownFields()
		if err := d.Decode(&dynamicData); err == nil {
			o.Data = &dynamicData
			return nil
		}
	}
	{
		var genericData map[string]interface{}
		d := json.NewDecoder(bytes.NewReader(tmp.Data))
		d.DisallowUnknownFields()
		if err := d.Decode(&genericData); err == nil {
			o.Data = genericData
			return nil
		}
	}
	return fmt.Errorf("failed to parse DataReading.Data for gatherer %s", o.DataGatherer)
}

// GatheredResource wraps the raw k8s resource that is sent to the jetstack secure backend
type GatheredResource struct {
	// Resource is a reference to a k8s object that was found by the informer
	// should be of type unstructured.Unstructured, raw Object
	Resource  interface{} `json:"resource"`
	DeletedAt Time        `json:"deleted_at,omitempty"`
}

func (v GatheredResource) MarshalJSON() ([]byte, error) {
	dateString := ""
	if !v.DeletedAt.IsZero() {
		dateString = v.DeletedAt.Format(TimeFormat)
	}

	data := struct {
		Resource  interface{} `json:"resource"`
		DeletedAt string      `json:"deleted_at,omitempty"`
	}{
		Resource:  v.Resource,
		DeletedAt: dateString,
	}

	return json.Marshal(data)
}

func (v *GatheredResource) UnmarshalJSON(data []byte) error {
	var tmpResource struct {
		Resource  *unstructured.Unstructured `json:"resource"`
		DeletedAt Time                       `json:"deleted_at,omitempty"`
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

// DynamicData is the DataReading.Data returned by the k8s.DataGathererDynamic
// gatherer
type DynamicData struct {
	Items []*GatheredResource `json:"items"`
}

// DiscoveryData is the DataReading.Data returned by the k8s.ConfigDiscovery
// gatherer
type DiscoveryData struct {
	ServerVersion *version.Info `json:"server_version"`
}
