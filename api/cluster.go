package api

import "encoding/json"

// ClusterSummary contains a summary of the most recent status of a cluster.
type ClusterSummary struct {
	Cluster         string     `json:"cluster"`
	LatestReportSet *ReportSet `json:"latestReportSet"`
}

// ReportSet groups one or more reports of different packages with the same timestamp for the same cluster.
type ReportSet struct {
	Cluster      string           `json:"-"`
	Timestamp    Time             `json:"timestamp"`
	FailureCount int              `json:"failureCount"`
	SuccessCount int              `json:"successCount"`
	Reports      []*ReportSummary `json:"reports"`
}

// ReportSummary constains a summary of a report.
type ReportSummary struct {
	ID           string `json:"id"`
	Package      string `json:"package"`
	Cluster      string `json:"-"`
	Timestamp    Time   `json:"-"`
	FailureCount int    `json:"failureCount"`
	SuccessCount int    `json:"successCount"`
}

// UnmarshalJSON unmarshals a ClusterSummary.
func (c *ClusterSummary) UnmarshalJSON(data []byte) error {
	type Alias ClusterSummary
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	c.LatestReportSet.Cluster = c.Cluster

	for idx := range c.LatestReportSet.Reports {
		c.LatestReportSet.Reports[idx].Cluster = c.LatestReportSet.Cluster
		c.LatestReportSet.Reports[idx].Timestamp = c.LatestReportSet.Timestamp
	}

	return nil
}
