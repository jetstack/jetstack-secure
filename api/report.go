package api

// Report contains the fields of a Preflight report
type Report struct {
	// Unique ID of the report
	ID string `json:"id"`
	// Timestamp indicates when the report was generated
	Timestamp Time `json:"timestamp"`
	// Cluster indicates which was the target of the report
	Cluster string `json:"cluster"`
	// Package indicates which package was used for the report
	Package     string          `json:"package"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Sections    []ReportSection `json:"sections,omitempty"`
}

// ReportSection contains the fields of a section inside a Report
type ReportSection struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Rules       []ReportRule `json:"rules,omitempty"`
}

// ReportRule contains the fields of a rule inside a Report
type ReportRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Manual      bool        `json:"manual,omitempty"`
	Remediation string      `json:"remediation,omitempty"`
	Links       []string    `json:"links,omitempty"`
	Success     bool        `json:"success"`
	Value       interface{} `json:"value,omitempty"`
	Missing     bool        `json:"missing"`
}

// ReportMetadata contains metadata about a report
type ReportMetadata struct {
	Cluster   string `json:"cluster"`
	Timestamp Time   `json:"timestamp"`
	Package   string `json:"package"`
	ID        string `json:"id"`
}
