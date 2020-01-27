package azblob

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/exporter"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// Output writes to an Azure Blob Storage bucket
type Output struct {
	credential azblob.Credential
	container  url.URL
	exporter   exporter.Exporter
}

// NewOutput creates a new Output
func NewOutput(ctx context.Context, format, containerName, accountName, accountKey string) (*Output, error) {
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, err
	}

	container, err := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", accountName, containerName))
	if err != nil {
		return nil, err
	}

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

	return &Output{
		credential: credential,
		container:  *container,
		exporter:   e,
	}, nil
}

// Write exports data in the specified format and writes it to the speficied bucket
func (o *Output) Write(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, rc *results.ResultCollection, cluster string, timestamp time.Time) error {
	buffer, err := o.exporter.Export(ctx, policyManifest, intermediateJSON, rc)
	if err != nil {
		return err
	}

	pipeline := azblob.NewPipeline(o.credential, azblob.PipelineOptions{})
	containerURL := azblob.NewContainerURL(o.container, pipeline)
	blobURL := containerURL.NewBlockBlobURL(fmt.Sprintf("%s/%s/%s%s", cluster, timestamp.Format(api.TimeFormat), policyManifest.ID, o.exporter.FileExtension()))

	_, err = azblob.UploadStreamToBlockBlob(ctx, buffer, blobURL, azblob.UploadStreamToBlockBlobOptions{
		// values chosen arbitrarily
		BufferSize: 2 * 1024 * 1024,
		MaxBuffers: 3,
	})
	if err != nil {
		return err
	}

	return nil
}
