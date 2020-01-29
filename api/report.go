package api

// Report contains the fields of a Preflight report
type Report struct {
	// Unique ID of the report.
	ID string `json:"id"`
	// PreflightVersion indicates the version of preflight this report was generated with.
	PreflightVersion string `json:"preflight-version"`
	// Timestamp indicates when the report was generated.
	Timestamp Time `json:"timestamp"`
	// Cluster indicates which was the target of the report.
	Cluster string `json:"cluster"`
	// Package indicates which package was used for the report. (deprecated)
	Package string `json:"package"`
	// PackageInformation contains all the information about the package that was used to generate the report.
	PackageInformation PackageInformation `json:"package-information"`
	// Name is the name of the package that was used for this report.
	Name string `json:"name"`
	// Description is the description of the package that was used for this report.
	Description string `json:"description,omitempty"`
	// Sections contains the sections of the package that was used for this report.
	Sections []ReportSection `json:"sections,omitempty"`
}

// Summarize produces as ReportSummary from a Report
func (r *Report) Summarize() ReportSummary {
	var successes, failures int
	for _, section := range r.Sections {
		for _, rule := range section.Rules {
			if rule.Success {
				successes++
			} else {
				failures++
			}
		}
	}

	return ReportSummary{
		ID:           r.ID,
		Package:      r.Package,
		Cluster:      r.Cluster,
		Timestamp:    r.Timestamp,
		FailureCount: failures,
		SuccessCount: successes,
	}
}

// PackageInformation contains all the details to identify a package.
type PackageInformation struct {
	// Namespace the package belongs to.
	Namespace string `json:"namespace"`
	// ID is the ID of the package.
	ID string `json:"id"`
	// Version is the version of the package.
	Version string `json:"version"`
}

// ReportSection contains the fields of a section inside a Report
type ReportSection struct {
	// ID is the ID of the section.
	ID string `json:"id"`
	// Name is the name of the section.
	Name string `json:"name"`
	// Description is the description of the section.
	Description string `json:"description,omitempty"`
	// Rules contain all the rules in the section.
	Rules []ReportRule `json:"rules,omitempty"`
}

// ReportRule contains the fields of a rule inside a Report
type ReportRule struct {
	// ID is the id of the rule.
	ID string `json:"id"`
	// Name is a shortname for the rule.
	Name string `json:"name"`
	// Description is a text describing what the rule is about.
	Description string `json:"description,omitempty"`
	// Manual indicated whether the rule can be evaluated automatically by Preflight or requires manual intervention.
	Manual bool `json:"manual,omitempty"`
	// Remediation is a text describing how to fix a failure of the rule.
	Remediation string `json:"remediation,omitempty"`
	// Links contains useful links related to the rule.
	Links []string `json:"links,omitempty"`
	// Success indicates whether the check was a success or not.
	Success bool `json:"success"`
	// Value contains the raw result of the check.
	Value interface{} `json:"value,omitempty"`
	// Violations contains the list of messages from the execution of the check
	Violations interface{} `json:"violations"`
	// Missing indicates whether the Rego rule was missing or not.
	Missing bool `json:"missing"`
}

// ReportMetadata contains metadata about a report
type ReportMetadata struct {
	// Unique ID of the report.
	ID string `json:"id"`
	// Timestamp indicates when the report was generated.
	Timestamp Time `json:"timestamp"`
	// Cluster indicates which was the target of the report.
	Cluster string `json:"cluster"`
	// Package indicates which package was used for the report. (deprecated)
	Package string `json:"package"`
	// PackageInformation contains all the information about the package that was used to generate the report.
	PackageInformation PackageInformation `json:"package-information"`
	// PreflightVersion indicates the version of preflight this report was generated with.
	PreflightVersion string `json:"preflight-version"`
}
