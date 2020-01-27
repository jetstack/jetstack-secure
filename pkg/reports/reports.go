package reports

import (
	"fmt"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
	"github.com/jetstack/preflight/pkg/rules"
	"github.com/jetstack/preflight/pkg/version"
)

// NewClusterSummary builds a summary for a current cluster based on a
// freshly generated report set
func NewClusterSummary(reports []api.Report) (api.ClusterSummary, error) {
	if len(reports) < 1 {
		return api.ClusterSummary{}, fmt.Errorf("you must supply at least one report")
	}

	reportSet, err := NewReportSet(reports)
	if err != nil {
		return api.ClusterSummary{}, fmt.Errorf("error constructing report set: %v", err)
	}

	return api.ClusterSummary{
		Cluster:         reportSet.Cluster,
		LatestReportSet: &reportSet,
	}, nil
}

// NewReportSet generates a summarized report set from the supplied reports
func NewReportSet(reports []api.Report) (api.ReportSet, error) {
	if len(reports) < 1 {
		return api.ReportSet{}, fmt.Errorf("you must supply at least one report")
	}

	reportSet := api.ReportSet{
		Cluster:   reports[0].Cluster,
		Timestamp: reports[0].Timestamp,
		Reports:   []*api.ReportSummary{},
	}

	for _, report := range reports {
		summary := SummarizeReport(report)
		reportSet.Reports = append(reportSet.Reports, &summary)
	}

	for _, summary := range reportSet.Reports {
		reportSet.SuccessCount += summary.SuccessCount
		reportSet.FailureCount += summary.FailureCount
	}

	return reportSet, nil
}

// SummarizeReport produces as ReportSummary from a Report
func SummarizeReport(report api.Report) api.ReportSummary {
	var successes, failures int
	for _, section := range report.Sections {
		for _, rule := range section.Rules {
			if rule.Success {
				successes++
			} else {
				failures++
			}
		}
	}

	return api.ReportSummary{
		ID:           report.ID,
		Package:      report.Package,
		Cluster:      report.Cluster,
		Timestamp:    report.Timestamp,
		FailureCount: failures,
		SuccessCount: successes,
	}
}

// NewReport creates a report from a policy manifest and a results collection
func NewReport(pm *packaging.PolicyManifest, rc *results.ResultCollection) (api.Report, error) {
	report := api.Report{
		// TODO: we are omitting ID, Timestamp and Cluster for now, but it will get fixed with #1
		PreflightVersion: version.PreflightVersion,
		Package:          pm.ID,
		PackageInformation: api.PackageInformation{
			Namespace: pm.Namespace,
			ID:        pm.ID,
			Version:   pm.PackageVersion,
		},
		Name:        pm.Name,
		Description: pm.Description,
		Sections:    make([]api.ReportSection, len(pm.Sections)),
	}

	for idxSection, section := range pm.Sections {
		report.Sections[idxSection] = api.ReportSection{
			ID:          section.ID,
			Name:        section.Name,
			Description: section.Description,
			Rules:       make([]api.ReportRule, len(section.Rules)),
		}

		for idxRule, rule := range section.Rules {
			links := make([]string, len(rule.Links))
			copy(links, rule.Links)

			results := rc.ByID()
			result := results[rules.RuleToResult(rule.ID)]

			if result == nil {
				supportsPrefix, err := pm.SupportsPreflightPrefix()
				if err != nil {
					return api.Report{}, err
				}

				if supportsPrefix {
					result = results[rules.LegacyRuleToResult(rule.ID)]
				}
			}

			var value interface{}
			violations := []string{}
			success := false
			missing := false

			switch {
			case result == nil:
				missing = true
			case result.IsFailureState():
				success = false
				violations = result.Violations
			case result.IsSuccessState():
				success = true
			default:
				value = result.Value
			}

			report.Sections[idxSection].Rules[idxRule] = api.ReportRule{
				ID:          rule.ID,
				Name:        rule.Name,
				Description: rule.Description,
				Manual:      rule.Manual,
				Remediation: rule.Remediation,
				Links:       links,
				Violations:  violations,
				Success:     success,
				Value:       value,
				Missing:     missing,
			}
		}
	}

	return report, nil
}
