package api

import "time"

// DataReadingsPost is the payload in the upload request.
type DataReadingsPost struct {
	AgentMetadata *AgentMetadata `json:"agent_metadata"`
	// DataGatherTime represents the time that the data readings were gathered
	DataGatherTime time.Time      `json:"data_gather_time"`
	DataReadings   []*DataReading `json:"data_readings"`
	SchemaVersion  string         `json:"schema_version"`
}

// DataReading is the output of a DataGatherer.
type DataReading struct {
	// ClusterID is optional as it can be infered from the agent
	// token when using basic authentication.
	ClusterID    string      `json:"cluster_id,omitempty"`
	DataGatherer string      `json:"data-gatherer"`
	Timestamp    Time        `json:"timestamp"`
	Data         interface{} `json:"data"`
}

// GatheredResource wraps the raw k8s resource that is sent to the jetstack secure backend
type GatheredResource struct {
	// Resource is a reference to a k8s object that was found by the informer
	// should be of type unstructured.Unstructured, raw Object
	Resource   interface{}               `json:"resource"`
	Properties *GatheredResourceMetadata `json:"item_metadata,omitempty"`
}

// GatheredResourceMetadata boundles additional platform metadata for a
// gathered k8s resource
type GatheredResourceMetadata struct {
	DeletedAt *Time `json:"deletedAt,omitempty"`
}
