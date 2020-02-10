package api

// ClusterSummary contains a summary of the most recent status of a cluster.
type ClusterSummary struct {
	Cluster         string     `json:"cluster"`
	LatestReportSet *ReportSet `json:"latestReportSet"`
}

// ReportSet groups one or more reports of different packages with the same timestamp for the same cluster.
type ReportSet struct {
	Cluster      string           `json:"cluster"`
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
