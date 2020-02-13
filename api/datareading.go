package api

// DataReading is the output of a DataGatherer.
type DataReading struct {
	DataGatherer string      `json:"data-gatherer"`
	Timestamp    Time        `json:"timestamp"`
	Data         interface{} `json:"data"`
}
