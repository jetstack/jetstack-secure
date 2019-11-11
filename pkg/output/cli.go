package output

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jetstack/preflight/pkg/exporter"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
)

// CLIOutput writes to stdout
type CLIOutput struct {
	exporter exporter.Exporter
}

// NewCLIOutput creates a new CLIOutput
func NewCLIOutput(format string) (*CLIOutput, error) {
	if format == "" {
		format = "cli"
	}
	var e exporter.Exporter
	switch format {
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
		return nil, fmt.Errorf("format %q not supported", format)
	}

	o := &CLIOutput{
		exporter: e,
	}
	return o, nil
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
