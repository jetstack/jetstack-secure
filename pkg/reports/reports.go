package reports

import (
	"fmt"
	"strings"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
	"github.com/jetstack/preflight/pkg/rules"
	"github.com/jetstack/preflight/pkg/version"
)

// NewClusterSummary builds a summary for a current cluster based on a
// freshly generated slice of reports.
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

// NewReportSet generates a summarized ReportSet from the supplied reports
// all reports should have the same Cluster and Timestamp
func NewReportSet(reports []api.Report) (api.ReportSet, error) {
	if len(reports) < 1 {
		return api.ReportSet{}, fmt.Errorf("you must supply at least one report")
	}

	clusters := map[string]int{}
	timestamps := map[api.Time]int{}
	for _, r := range reports {
		clusters[r.Cluster]++
		timestamps[r.Timestamp]++
	}

	if len(clusters) > 1 {
		return api.ReportSet{}, fmt.Errorf("reports must be for the same cluster")
	}

	if len(timestamps) > 1 {
		return api.ReportSet{}, fmt.Errorf("reports must have the same timestamp")
	}

	reportSet := api.ReportSet{
		Cluster:   reports[0].Cluster,
		Timestamp: reports[0].Timestamp,
		Reports:   []*api.ReportSummary{},
	}

	for _, report := range reports {
		if report.Cluster != reportSet.Cluster {
			return reportSet, fmt.Errorf("reports must be for the same cluster")
		}
		if report.Timestamp != reportSet.Timestamp {
			return reportSet, fmt.Errorf("reports must be for the same timestamp")
		}

		summary := report.Summarize()
		reportSet.Reports = append(reportSet.Reports, &summary)

		reportSet.SuccessCount += summary.SuccessCount
		reportSet.FailureCount += summary.FailureCount
	}

	return reportSet, nil
}

// NewReport creates a report from a policy manifest and a results collection
func NewReport(pm *packaging.PolicyManifest, rc *results.ResultCollection) (api.Report, error) {
	report := api.Report{
		// TODO: we are omitting ID, Timestamp and Cluster for now, but it will get fixed with #1
		PreflightVersion: version.PreflightVersion,
		Package:          pm.ID,
		PackageInformation: api.PackageInformation{
			Namespace:     pm.Namespace,
			ID:            pm.ID,
			Version:       pm.PackageVersion,
			SchemaVersion: pm.SchemaVersion,
		},
		Name:        pm.Name,
		Description: pm.Description,
		Sections:    make([]api.ReportSection, len(pm.Sections)),
	}

	missingRules := []string{}
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
				if !rule.Manual {
					missing = true
					missingRules = append(missingRules, rule.ID)
				}
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

	if len(missingRules) > 0 {
		return report, &MissingRegoDefinitionError{
			pkg: report.Package,
			ids: missingRules,
		}
	}
	return report, nil
}

// MissingRegoDefinitionError error to be returned when a rule from the PolicyManifest was not found in Rego.
type MissingRegoDefinitionError struct {
	pkg string
	ids []string
}

func (e *MissingRegoDefinitionError) Error() string {
	return fmt.Sprintf("the following rules from the package %q are missing their Rego definitions: %s", e.pkg, strings.Join(e.ids, ", "))
}
