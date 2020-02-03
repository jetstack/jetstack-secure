package output

import (
	"context"
	"log"
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/output/azblob"
	"github.com/jetstack/preflight/pkg/output/gcs"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
)

// Output consumes policy manifests, intermediate JSON, and results, exports them in a specified format, and writes them to an output
type Output interface {
	Write(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, rc *results.ResultCollection, cluster string, timestamp time.Time) error
	WriteIndex(ctx context.Context, cluster string, timestamp time.Time, clusterSummary *api.ClusterSummary) error
}

// TODO: Change config to use new struct
// type OutputsConfig struct {
// 	Local  []*LocalOutputConfig
// 	CLI    []*CLIOutputConfig
// 	GCS    []*gcs.GCSOutputConfig
// 	AZBlob []*azblob.AZBlobOutputConfig
// }

type OutputConfig struct {
	Type   string
	Format string
	// For local
	Path string
	// For GCS
	BucketName      string
	CredentialsPath string
	// For AZBlob
	ContainerName string
	AccountName   string
	AccountKey    string
}

func NewOutputs(ctx context.Context, config []*OutputConfig) []Output {
	outputs := make([]Output, 0)
	for _, outputConfig := range config {
		if outputConfig.Type == "local" {
			output, err := NewLocalOutput(ctx, &LocalOutputConfig{
				Format: outputConfig.Format,
				Path:   outputConfig.Path,
			})
			if err != nil {
				log.Fatalf("Could not create local output: %s", err)
			}
			outputs = append(outputs, output)
		} else if outputConfig.Type == "cli" {
			output, err := NewCLIOutput(ctx, &CLIOutputConfig{
				Format: outputConfig.Format,
			})
			if err != nil {
				log.Fatalf("Could not create CLI output: %s", err)
			}
			outputs = append(outputs, output)
		} else if outputConfig.Type == "gcs" {
			output, err := gcs.NewGCSOutput(ctx, &gcs.GCSOutputConfig{
				Format:          outputConfig.Format,
				CredentialsPath: outputConfig.CredentialsPath,
				BucketName:      outputConfig.BucketName,
			})
			if err != nil {
				log.Fatalf("Could not create GCS output: %s", err)
			}
			outputs = append(outputs, output)
		} else if outputConfig.Type == "azblob" {
			output, err := azblob.NewAZBlobOutput(ctx, &azblob.AZBlobOutputConfig{
				Format:        outputConfig.Format,
				ContainerName: outputConfig.ContainerName,
				AccountName:   outputConfig.AccountName,
				AccountKey:    outputConfig.AccountKey,
			})
			if err != nil {
				log.Fatalf("Could not create AZBlob output: %s", err)
			}
			outputs = append(outputs, output)
		} else {
			log.Fatalf("Output type not recognised: %s", outputConfig.Type)
		}
	}
	return outputs
}
