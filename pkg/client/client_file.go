package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/api"
)

// FileClient writes the supplied readings to a file, in JSON format.
type FileClient struct {
	path string
}

func NewFileClient(path string) Client {
	return &FileClient{
		path: path,
	}
}

func (o *FileClient) PostDataReadingsWithOptions(ctx context.Context, readings []*api.DataReading, _ Options) error {
	log := klog.FromContext(ctx)
	data, err := json.MarshalIndent(readings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %s", err)
	}
	err = os.WriteFile(o.path, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %s", err)
	}
	log.Info("Data saved to local file", "outputPath", o.path)
	return nil
}
