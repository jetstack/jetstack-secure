package exporter

import (
	"bytes"
	"context"

	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
)

// RawExporter is an Exporter that outputs raw JSON results
type RawExporter struct {
}

// NewRawExporter creates a new RawExporter
func NewRawExporter() *RawExporter {
	return &RawExporter{}
}

// Export writes the results to a buffer
func (e *RawExporter) Export(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, rc *results.ResultCollection) (*bytes.Buffer, error) {
	writer := bytes.NewBuffer([]byte{})
	err := rc.Serialize(writer)
	if err != nil {
		return nil, err
	}
	return writer, nil
}

// FileExtension returns the file extension for this exporter's format
func (e *RawExporter) FileExtension() string {
	return ".raw.json"
}
