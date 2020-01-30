package exporter

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/gookit/color"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
	"github.com/jetstack/preflight/pkg/rules"
)

// CLIExporter is an Exporter that outputs a report in Command Line Interface format
type CLIExporter struct {
}

// NewCLIExporter creates a new CLIExporter
func NewCLIExporter() *CLIExporter {
	return &CLIExporter{}
}

// Export writes the CLI report to a buffer
func (e *CLIExporter) Export(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, results *results.ResultCollection) (*bytes.Buffer, error) {
	// formatters
	fmtTitle1 := color.New(color.OpBold, color.FgWhite).Render
	fmtTitle2 := color.New(color.OpBold, color.FgBlue).Render
	fmtTitle3 := color.New(color.OpBold, color.FgBlue).Render
	fmtError := color.FgRed.Render
	fmtImportant := color.OpBold.Render
	fmtConsole := color.New(color.BgGray, color.FgWhite).Render

	resultsByID := results.ByID()

	writer := bytes.NewBuffer([]byte{})

	fmt.Fprintln(writer, "_____")
	fmt.Fprintf(writer, "%s - %s\n\n", fmtTitle1(policyManifest.ID), fmtImportant(policyManifest.Name))
	fmt.Fprintf(writer, "  %s\n", policyManifest.Description)
	for _, section := range policyManifest.Sections {
		fmt.Fprintf(writer, "  %s - %s\n\n", fmtTitle2(section.ID), fmtImportant(section.Name))
		fmt.Fprintf(writer, "    %s\n", section.Description)
		for _, rule := range section.Rules {
			icon := color.FgYellow.Sprint("!")
			var info interface{}
			result := resultsByID[rules.RuleToResult(rule.ID)]
			if result != nil {
				if result.IsFailureState() {
					icon = color.FgRed.Sprint("✗")
				} else if result.IsSuccessState() {
					icon = color.FgGreen.Sprint("✓")
				} else {
					icon = color.FgBlue.Sprint("+")
					info = result.Value
				}
			}

			fmt.Fprintf(writer, "      %s - %s -> %s\n\n", fmtTitle3(rule.ID), fmtImportant(rule.Name), icon)
			fmt.Fprintf(writer, "        %s\n", rule.Description)
			if len(rule.Remediation) > 0 {
				fmt.Fprintf(writer, "        Remediation: %s", rule.Remediation)
				fmt.Fprintln(writer)
			}
			if len(rule.Links) > 0 {
				fmt.Fprintf(writer, "        Reference: \n")
				for idx, link := range rule.Links {
					fmt.Fprintf(writer, "          %d: %s \n", idx, link)
				}
				fmt.Fprintln(writer)
			}
			if info != nil {
				fmt.Fprintln(writer)
				fmt.Fprintln(writer, "        Information:")
				fmt.Fprintf(writer, "        %+v\n", fmtConsole(info))
			}
			fmt.Fprintln(writer)
		}
	}

	failedRules := results.ListFailing()
	if len(failedRules) > 0 {
		fmt.Fprintln(writer)
		fmt.Fprintf(writer, fmtError("%d rules failed: "), len(failedRules))
		links := make([]string, len(failedRules))
		for idx, f := range failedRules {
			id := rules.ResultToRule(f.ID)
			links[idx] = id
		}
		fmt.Fprintln(writer, strings.Join(links, ", "))
	}

	fmt.Fprintln(writer, "_____")
	return writer, nil
}

// ExportIndex formats the supplied cluster summary
func (e *CLIExporter) ExportIndex(ctx context.Context, clusterSummary *api.ClusterSummary) (*bytes.Buffer, error) {
	lines := []string{
		"Summary",
		"-------",
		fmt.Sprintf("cluster: %s", clusterSummary.Cluster),
		fmt.Sprintf("failures: %d", clusterSummary.LatestReportSet.FailureCount),
		fmt.Sprintf("successes: %d", clusterSummary.LatestReportSet.SuccessCount),
		"reports:",
	}

	for _, r := range clusterSummary.LatestReportSet.Reports {
		lines = append(lines, []string{
			fmt.Sprintf("  package: %s", r.Package),
			fmt.Sprintf("    failures: %d", r.FailureCount),
			fmt.Sprintf("    successes: %d", r.SuccessCount)}...)
	}
	lines = append(lines, "")

	return bytes.NewBuffer([]byte(strings.Join(lines, "\n"))), nil
}

// FileExtension returns the file extension for this exporter's format
func (e *CLIExporter) FileExtension() string {
	return ""
}
