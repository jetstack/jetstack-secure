package reports

import (
	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
	"github.com/jetstack/preflight/pkg/rules"
	"github.com/jetstack/preflight/pkg/version"
)

// ConstructReport creates a report from a policy manifest and a results collection
func ConstructReport(pm *packaging.PolicyManifest, rc *results.ResultCollection) (api.Report, error) {
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
