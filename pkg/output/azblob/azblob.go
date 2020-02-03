package azblob

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/exporter"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// Output writes to an Azure Blob Storage bucket
type AZBlobOutput struct {
	credential azblob.Credential
	container  url.URL
	exporter   exporter.Exporter
}

type AZBlobOutputConfig struct {
	Format        string
	ContainerName string
	AccountName   string
	AccountKey    string
}

// NewAZBlobOutput creates a new Output
func NewAZBlobOutput(ctx context.Context, config *AZBlobOutputConfig) (*AZBlobOutput, error) {
	if config.Format == "" {
		log.Fatal("Missing 'format' property in azblob output configuration.")
	}
	if config.ContainerName == "" {
		log.Fatal("Missing 'container' property in azblob output configuration.")
	}
	if config.AccountName == "" {
		log.Fatal("Either the AZURE_STORAGE_ACCOUNT or AZURE_STORAGE_ACCESS_KEY environment variable is not set.")
	}
	if config.AccountKey == "" {
		log.Fatal("Either the AZURE_STORAGE_ACCOUNT or AZURE_STORAGE_ACCESS_KEY environment variable is not set.")
	}
	credential, err := azblob.NewSharedKeyCredential(config.AccountName, config.AccountKey)
	if err != nil {
		return nil, err
	}

	container, err := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", config.AccountName, config.ContainerName))
	if err != nil {
		return nil, err
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

	return &AZBlobOutput{
		credential: credential,
		container:  *container,
		exporter:   e,
	}, nil
}

// Write exports data in the specified format and writes it to the speficied bucket
func (o *AZBlobOutput) Write(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, rc *results.ResultCollection, cluster string, timestamp time.Time) error {
	buffer, err := o.exporter.Export(ctx, policyManifest, intermediateJSON, rc)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("%s/%s/%s%s", cluster, timestamp.Format(api.TimeFormat), policyManifest.ID, o.exporter.FileExtension())

	return writeBufferToPath(o, ctx, path, buffer)
}

// WriteIndex exports clusterSummary data in the specified format
func (o *AZBlobOutput) WriteIndex(ctx context.Context, cluster string, timestamp time.Time, clusterSummary *api.ClusterSummary) error {
	buffer, err := o.exporter.ExportIndex(ctx, clusterSummary)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("index/%s%s", cluster, o.exporter.FileExtension())

	return writeBufferToPath(o, ctx, path, buffer)
}

func writeBufferToPath(o *AZBlobOutput, ctx context.Context, path string, buffer *bytes.Buffer) error {
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
