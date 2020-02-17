package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/exporter"
	"github.com/jetstack/preflight/pkg/output"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
)

// Config is the configuration for the CLI output.
type Config struct {
	Format string
}

// NewOutput creates a new CLIOutput
func (c *Config) NewOutput(ctx context.Context) (output.Output, error) {
	var e exporter.Exporter
	switch c.Format {
	case "":
		// If no format is specified default to CLI format
		e = exporter.NewCLIExporter()
	case exporter.FormatCLI:
		e = exporter.NewCLIExporter()
	case exporter.FormatJSON:
		e = exporter.NewJSONExporter()
	case exporter.FormatRaw:
		e = exporter.NewRawExporter()
	case exporter.FormatMD:
		e = exporter.NewMarkdownExporter()
	case exporter.FormatHTML:
		e = exporter.NewHTMLExporter()
	case exporter.FormatIntermediate:
		e = exporter.NewIntermediateExporter()
	default:
		return nil, fmt.Errorf("format %q not supported", c.Format)
	}

	o := &CLIOutput{
		exporter: e,
	}
	return o, nil
}

// CLIOutput writes to stdout
type CLIOutput struct {
	exporter exporter.Exporter
}

// Write exports data in the specified format, or CLI format by default, and writes it to stdout
func (o *CLIOutput) Write(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, rc *results.ResultCollection, cluster string, timestamp time.Time) error {
	buffer, err := o.exporter.Export(ctx, policyManifest, intermediateJSON, rc)
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(buffer.Bytes())
	if err != nil {
		return err
	}

	return nil
}

// WriteIndex exports clusterSummary data in the specified format
func (o *CLIOutput) WriteIndex(ctx context.Context, cluster string, timestamp time.Time, clusterSummary *api.ClusterSummary) error {
	buffer, err := o.exporter.ExportIndex(ctx, clusterSummary)
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(buffer.Bytes())
	if err != nil {
		return err
	}

	return nil
}
