package output

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/exporter"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
)

// LocalOutput writes to a local file
type LocalOutput struct {
	path     string
	exporter exporter.Exporter
}

type LocalOutputConfig struct {
	Format string
	Path   string
}

// NewLocalOutput creates a new LocalOutput
func NewLocalOutput(ctx context.Context, config *LocalOutputConfig) (*LocalOutput, error) {
	if config.Format == "" {
		log.Fatal("Missing 'format' property in local output configuration.")
	}
	if config.Path == "" {
		log.Fatal("Missing 'path' property in local output configuration.")
	}
	var e exporter.Exporter
	switch config.Format {
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
		return nil, fmt.Errorf("format %q not supported", config.Format)
	}

	o := &LocalOutput{
		path:     config.Path,
		exporter: e,
	}
	return o, nil
}

// Write exports data in the specified format and writes it to the specified local file
func (o *LocalOutput) Write(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, rc *results.ResultCollection, cluster string, timestamp time.Time) error {
	buffer, err := o.exporter.Export(ctx, policyManifest, intermediateJSON, rc)
	if err != nil {
		return err
	}

	fullpath := path.Join(o.path, cluster, timestamp.Format(api.TimeFormat), fmt.Sprintf("%s%s", policyManifest.ID, o.exporter.FileExtension()))
	info, err := os.Stat(fullpath)

	if os.IsNotExist(err) {
		return writeBufferToPath(fullpath, buffer)
	} else if err != nil {
		return err
	} else if info.IsDir() {
		return fmt.Errorf("%q is an existing directory", fullpath)
	} else {
		return fmt.Errorf("%q is an existing file", fullpath)
	}
}

// WriteIndex exports clusterSummary data in the specified format
func (o *LocalOutput) WriteIndex(ctx context.Context, cluster string, timestamp time.Time, clusterSummary *api.ClusterSummary) error {
	buffer, err := o.exporter.ExportIndex(ctx, clusterSummary)
	if err != nil {
		return err
	}

	fullpath := path.Join(o.path, "index", fmt.Sprintf("%s%s", cluster, o.exporter.FileExtension()))
	info, err := os.Stat(fullpath)

	if os.IsNotExist(err) {
		return writeBufferToPath(fullpath, buffer)
	} else if err != nil {
		return err
	} else if info.IsDir() {
		return fmt.Errorf("%q is an existing directory", fullpath)
	} else {
		log.Printf("%q is an existing index, overwriting...", fullpath)
		return writeBufferToPath(fullpath, buffer)
	}
}

func writeBufferToPath(fullpath string, buffer *bytes.Buffer) error {
	err := os.MkdirAll(path.Dir(fullpath), 0744)
	if err != nil {
		return err
	}
	file, err := os.Create(fullpath)
	if err != nil {
		return err
	}

	_, err = file.Write(buffer.Bytes())
	return err
}
