package exporter

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
)

// JSONExporter is an Exporter that outputs the results enriched with metadata in a JSON format
type JSONExporter struct {
}

// NewJSONExporter creates a new JSONExporter
func NewJSONExporter() *JSONExporter {
	return &JSONExporter{}
}

// Export writes a report with the evaluated results enriched with the metadata from the policy manifest in a JSON format.
func (e *JSONExporter) Export(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, rc *results.ResultCollection) (*bytes.Buffer, error) {
	report := api.Report{
		// TODO: we are omitting ID, Timestamp and Cluster for now, but it will get fixed with #1
		Package:     policyManifest.ID,
		Name:        policyManifest.Name,
		Description: policyManifest.Description,
		Sections:    make([]api.ReportSection, len(policyManifest.Sections)),
	}

	for idxSection, section := range policyManifest.Sections {
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
			result := results[ruleToResult(rule.ID)]
			var value interface{}
			success := false
			missing := false

			switch {
			case result == nil:
				missing = true
			case result.IsFailureState():
				success = false
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
				Success:     success,
				Value:       value,
				Missing:     missing,
			}
		}
	}

	b, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}

	return bytes.NewBuffer(b), nil
}

// FileExtension returns the file extension for this exporter's format
func (e *JSONExporter) FileExtension() string {
	return ".json"
}
