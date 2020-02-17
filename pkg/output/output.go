package output

import (
	"context"
	"fmt"
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
)

type TypedConfig struct {
	Type   string
	Config Config
}

// NewOutput creates an Output from a TypedConfig.
func (tc *TypedConfig) NewOutput(ctx context.Context) (Output, error) {
	if err := tc.validate(); err != nil {
		return nil, err
	}
	return tc.Config.NewOutput(ctx)
}

func (tc *TypedConfig) validate() error {
	outputTypes := []string{"local", "cli", "gcs", "azblob"}
	for _, outputType := range outputTypes {
		if tc.Type == outputType {
			return nil
			// TODO: Implement validation for each output type's config
			// return tc.Config.Validate()
		}
	}
	return fmt.Errorf("invalid package source type %q", tc.Type)
}

// Config is the configuration of an Output. It acts as a factory for Outputs.
type Config interface {
	// NewOutput creates an Output based on the configuration.
	NewOutput(ctx context.Context) (Output, error)
}

// Output consumes policy manifests, intermediate JSON, and results, exports them in a specified format, and writes them to an output
type Output interface {
	Write(ctx context.Context, policyManifest *packaging.PolicyManifest, intermediateJSON []byte, rc *results.ResultCollection, cluster string, timestamp time.Time) error
	WriteIndex(ctx context.Context, cluster string, timestamp time.Time, clusterSummary *api.ClusterSummary) error
}
