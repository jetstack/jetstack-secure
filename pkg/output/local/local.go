package local

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
	"github.com/jetstack/preflight/pkg/output"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
)

// Config is the configuration for the local output.
type Config struct {
	Format string
	Path   string
}

// NewOutput creates a new LocalOutput.
func (c *Config) NewOutput(ctx context.Context) (output.Output, error) {
	var e exporter.Exporter
	switch c.Format {
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

	o := &Output{
		path:     c.Path,
		exporter: e,
	}
	return o, nil
}

// Output writes to a local file
type Output struct {
	path     string
	exporter exporter.Exporter
}

// Write exports data in the specified format and writes it to the specified local file
func (o *Output) Write(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, rc *results.ResultCollection, cluster string, timestamp time.Time) error {
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
func (o *Output) WriteIndex(ctx context.Context, cluster string, timestamp time.Time, clusterSummary *api.ClusterSummary) error {
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
