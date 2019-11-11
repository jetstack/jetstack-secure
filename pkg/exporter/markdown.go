package exporter

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
)

// MarkdownExporter is an Exporter that outputs a report in markdown format
type MarkdownExporter struct {
}

// NewMarkdownExporter creates a new MarkdownExporter
func NewMarkdownExporter() *MarkdownExporter {
	return &MarkdownExporter{}
}

// Export writes the markdown report to a buffer
func (e *MarkdownExporter) Export(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, results *results.ResultCollection) (*bytes.Buffer, error) {
	resultsByID := results.ByID()

	writer := bytes.NewBuffer([]byte{})

	fmt.Fprintf(writer, "# %s - %s\n\n", policyManifest.ID, policyManifest.Name)
	fmt.Fprintf(writer, "%s\n\n", policyManifest.Description)
	for _, section := range policyManifest.Sections {
		fmt.Fprintf(writer, "## %s - %s\n\n", section.ID, section.Name)
		for _, rule := range section.Rules {
			icon := "!"
			var info interface{}
			result := resultsByID[ruleToResult(rule.ID)]
			if result != nil {
				if result.IsFailureState() {
					icon = "✗"
				} else if result.IsSuccessState() {
					icon = "✓"
				} else {
					icon = "+"
					info = result.Value
				}
			}

			fmt.Fprintf(writer, "### <a name=\"%s\"></a> %q - %s -> %s\n\n", rule.ID, rule.ID, rule.Name, icon)
			fmt.Fprintf(writer, "%s\n", rule.Description)
			if len(rule.Remediation) > 0 {
				fmt.Fprintf(writer, "Remediation: %s", rule.Remediation)
				fmt.Fprintln(writer)
			}
			if len(rule.Links) > 0 {
				fmt.Fprintf(writer, "Reference: ")
				for idx, link := range rule.Links {
					fmt.Fprintf(writer, "[%d](%s) ", idx, link)
				}
				fmt.Fprintln(writer)
			}
			if info != nil {
				fmt.Fprintln(writer)
				fmt.Fprintln(writer, "Information:")
				fmt.Fprintln(writer)
				fmt.Fprintln(writer, "```")
				fmt.Fprintf(writer, "%+v\n", info)
				fmt.Fprintln(writer, "```")
			}
			fmt.Fprintln(writer)
		}
	}

	failedRules := results.ListFailing()
	if len(failedRules) > 0 {
		fmt.Fprintln(writer, "_____")
		fmt.Fprintln(writer)
		fmt.Fprintf(writer, "%d rules failed: ", len(failedRules))
		links := make([]string, len(failedRules))
		for idx, f := range failedRules {
			id := resultToRule(f.ID)
			links[idx] = fmt.Sprintf("[%s](#%s)", id, id)
		}
		fmt.Fprintln(writer, strings.Join(links, ", "))
	}

	return writer, nil
}

// FileExtension returns the file extension for this exporter's format
func (e *MarkdownExporter) FileExtension() string {
	return ".md"
}
