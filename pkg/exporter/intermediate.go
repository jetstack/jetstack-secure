package exporter

import (
	"bytes"
	"context"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
)

// IntermediateExporter is an Exporter that outputs intermediate JSON
type IntermediateExporter struct {
}

// NewIntermediateExporter creates a new IntermediateExporter
func NewIntermediateExporter() *IntermediateExporter {
	return &IntermediateExporter{}
}

// Export writes the intermediate JSON to a buffer
func (e *IntermediateExporter) Export(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, rc *results.ResultCollection) (*bytes.Buffer, error) {
	writer := bytes.NewBuffer([]byte{})
	_, err := writer.Write(intermediateJSON)
	if err != nil {
		return nil, err
	}
	return writer, nil
}

// ExportIndex formats the supplied cluster summary
func (e *IntermediateExporter) ExportIndex(ctx context.Context, clusterSummary *api.ClusterSummary) (*bytes.Buffer, error) {
	return bytes.NewBuffer([]byte("there is no intermediateJSON for cluster report summaries")), nil
}

// FileExtension returns the file extension for this exporter's format
func (e *IntermediateExporter) FileExtension() string {
	return ".intermediate.json"
}
