package exporter

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/reports"
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
	report, err := reports.NewReport(policyManifest, rc)
	if err != nil {
		return nil, err
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
