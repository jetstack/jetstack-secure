package api

import "time"

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
	ClusterID    string      `json:"cluster_id,omitempty"`
	DataGatherer string      `json:"data-gatherer"`
	Timestamp    Time        `json:"timestamp"`
	Data         interface{} `json:"data"`
}
