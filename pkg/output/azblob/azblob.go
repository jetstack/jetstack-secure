package azblob

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/exporter"
	"github.com/jetstack/preflight/pkg/output"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// Config is the configuration for the AZBlob output.
type Config struct {
	Format        string
	ContainerName string
	AccountName   string
	AccountKey    string
}

// NewOutput creates a new AZBlobOutput
func (c *Config) NewOutput(ctx context.Context) (output.Output, error) {
	credential, err := azblob.NewSharedKeyCredential(c.AccountName, c.AccountKey)
	if err != nil {
		return nil, err
	}

	container, err := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", c.AccountName, c.ContainerName))
	if err != nil {
		return nil, err
	}

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

	return &Output{
		credential: credential,
		container:  *container,
		exporter:   e,
	}, nil
}

// Output writes to an Azure Blob Storage bucket
type Output struct {
	credential azblob.Credential
	container  url.URL
	exporter   exporter.Exporter
}

// Write exports data in the specified format and writes it to the speficied bucket
func (o *Output) Write(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, rc *results.ResultCollection, cluster string, timestamp time.Time) error {
	buffer, err := o.exporter.Export(ctx, policyManifest, intermediateJSON, rc)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("%s/%s/%s%s", cluster, timestamp.Format(api.TimeFormat), policyManifest.ID, o.exporter.FileExtension())

	return writeBufferToPath(o, ctx, path, buffer)
}

// WriteIndex exports clusterSummary data in the specified format
func (o *Output) WriteIndex(ctx context.Context, cluster string, timestamp time.Time, clusterSummary *api.ClusterSummary) error {
	buffer, err := o.exporter.ExportIndex(ctx, clusterSummary)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("index/%s%s", cluster, o.exporter.FileExtension())

	return writeBufferToPath(o, ctx, path, buffer)
}

func writeBufferToPath(o *Output, ctx context.Context, path string, buffer *bytes.Buffer) error {
	pipeline := azblob.NewPipeline(o.credential, azblob.PipelineOptions{})
	containerURL := azblob.NewContainerURL(o.container, pipeline)
	blobURL := containerURL.NewBlockBlobURL(path)

	_, err := azblob.UploadStreamToBlockBlob(ctx, buffer, blobURL, azblob.UploadStreamToBlockBlobOptions{
		// values chosen arbitrarily
		BufferSize: 2 * 1024 * 1024,
		MaxBuffers: 3,
	})

	if err != nil {
		return err
	}

	return nil
}
