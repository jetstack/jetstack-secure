package exporter

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
)

// Exporter consumes policy manifests, intermediate JSON, and results, and exports them to a buffer in a certain format
type Exporter interface {
	Export(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, results *results.ResultCollection) (*bytes.Buffer, error)
	FileExtension() string
}

const (
	// FormatCLI is the Comand Line Interface format
	FormatCLI = "cli"
	// FormatJSON is a JSON encoded format that contains the results enriched with metadata
	FormatJSON = "json"
	// FormatRaw is the raw output out of the evaluation, without metadata, encoded in JSON
	FormatRaw = "raw"
	// FormatMD is the markdown report format
	FormatMD = "markdown"
	// FormatHTML is the HTML report format
	FormatHTML = "html"
	// FormatIntermediate is the intermediate JSON format
	FormatIntermediate = "intermediate"
)

func ruleToResult(ruleID string) string {
	return fmt.Sprintf("preflight_%s", strings.ReplaceAll(ruleID, ".", "_"))
}

func resultToRule(resultID string) string {
	return strings.TrimPrefix(strings.ReplaceAll(resultID, "_", "."), "preflight.")
}
