package exporter

import (
	"bytes"
	"context"

	"github.com/gomarkdown/markdown"
	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
)

// HTMLExporter is an Exporter that outputs a report in HTML format
type HTMLExporter struct {
}

// NewHTMLExporter creates a new HTMLExporter
func NewHTMLExporter() *HTMLExporter {
	return &HTMLExporter{}
}

// Export writes the HTML report to a buffer
func (e *HTMLExporter) Export(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, results *results.ResultCollection) (*bytes.Buffer, error) {
	mdExporter := NewMarkdownExporter()
	md, err := mdExporter.Export(ctx, policyManifest, intermediateJSON, results)
	if err != nil {
		return nil, err
	}
	htmlBytes := markdown.ToHTML(md.Bytes(), nil, nil)
	writer := bytes.NewBuffer([]byte{})
	_, err = writer.Write(htmlBytes)
	if err != nil {
		return nil, err
	}

	return writer, nil
}

// ExportIndex formats the supplied cluster summary
func (e *HTMLExporter) ExportIndex(ctx context.Context, clusterSummary *api.ClusterSummary) (*bytes.Buffer, error) {
	mdExporter := NewMarkdownExporter()
	md, err := mdExporter.ExportIndex(ctx, clusterSummary)
	if err != nil {
		return nil, err
	}
	htmlBytes := markdown.ToHTML(md.Bytes(), nil, nil)
	writer := bytes.NewBuffer([]byte{})
	_, err = writer.Write(htmlBytes)
	if err != nil {
		return nil, err
	}

	return writer, nil
}

// FileExtension returns the file extension for this exporter's format
func (e *HTMLExporter) FileExtension() string {
	return ".html"
}
