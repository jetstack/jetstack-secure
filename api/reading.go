package api

// Reading is the output of a datagatherer.
type Reading struct {
	DataGatherer string      `json:"data-gatherer"`
	Timestamp    Time        `json:"timestamp"`
	Data         interface{} `json:"data"`
}
