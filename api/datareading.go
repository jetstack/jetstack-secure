package api

import (
	"time"

	jsoniter "github.com/json-iterator/go"
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
	// ClusterID is optional as it can be infered from the agent
	// token when using basic authentication.
	ClusterID     string      `json:"cluster_id,omitempty"`
	DataGatherer  string      `json:"data-gatherer"`
	Timestamp     Time        `json:"timestamp"`
	Data          interface{} `json:"data"`
	SchemaVersion string      `json:"schema_version"`
}

// GatheredResource wraps the raw k8s resource that is sent to the jetstack secure backend
type GatheredResource struct {
	// Resource is a reference to a k8s object that was found by the informer
	// should be of type unstructured.Unstructured, raw Object
	Resource  interface{}
	DeletedAt Time
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

	return jsoniter.Marshal(data)
}
