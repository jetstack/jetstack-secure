package gcs

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/storage"
	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/exporter"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
	"google.golang.org/api/option"
)

// Output writes to a Google Cloud Storage bucket
type Output struct {
	bucket   *storage.BucketHandle
	exporter exporter.Exporter
}

// NewOutput creates a new Output
func NewOutput(ctx context.Context, format, bucketName, credentialsPath string) (*Output, error) {
	c, err := storage.NewClient(ctx, option.WithCredentialsFile(credentialsPath))
	if err != nil {
		log.Fatalf("Failed to open Google Cloud Storage connection: %s", err)
	}
	s := c.Bucket(bucketName)
	var e exporter.Exporter
	switch format {
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

	o := &Output{
		bucket:   s,
		exporter: e,
	}
	return o, nil
}

// Write exports data in the specified format and writes it to the specified GCS bucket
func (o *Output) Write(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, rc *results.ResultCollection, cluster string, timestamp time.Time) error {
	buffer, err := o.exporter.Export(ctx, policyManifest, intermediateJSON, rc)
	if err != nil {
		return err
	}
	object := o.bucket.Object(fmt.Sprintf("%s/%s/%s%s", cluster, timestamp.Format(api.TimeFormat), policyManifest.ID, o.exporter.FileExtension()))
	writer := object.NewWriter(ctx)
	defer func() { err = writer.Close() }()

	_, err = writer.Write(buffer.Bytes())
	if err != nil {
		return err
	}

	return nil
}

// WriteIndex exports clusterSummary data in the specified format
func (o *Output) WriteIndex(ctx context.Context, cluster string, timestamp time.Time, clusterSummary *api.ClusterSummary) (err error) {
	buffer, err := o.exporter.ExportIndex(ctx, clusterSummary)
	if err != nil {
		return err
	}
	object := o.bucket.Object(fmt.Sprintf("index/%s%s", cluster, o.exporter.FileExtension()))
	writer := object.NewWriter(ctx)
	defer func() { err = writer.Close() }()

	_, err = writer.Write(buffer.Bytes())
	if err != nil {
		return err
	}

	// make sure we return err, so we cover the case of errors happening in the deferred closing.
	return err
}
