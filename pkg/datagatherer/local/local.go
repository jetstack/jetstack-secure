package local

import (
	"context"
	"fmt"
	"io/ioutil"
	"reflect"

	"github.com/jetstack/preflight/pkg/datagatherer"
)

// Config is the configuration for a local DataGatherer.
type Config struct {
	// DataPath is the path to file containing the data to load.
	DataPath string `yaml:"data-path"`
}

// validate validates the configuration.
func (c *Config) validate() error {
	if c.DataPath == "" {
		return fmt.Errorf("invalid configuration: DataPath cannot be empty")
	}
	return nil
}

// DataGatherer is a data-gatherer that loads data from a local file.
type DataGatherer struct {
	dataPath string
}

// NewDataGatherer returns a new DataGatherer.
func (c *Config) NewDataGatherer(ctx context.Context) (datagatherer.DataGatherer, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	return &DataGatherer{
		dataPath: c.DataPath,
	}, nil
}

// Run starts the data gatherer's informers for resource collection.
// Returns error if the data gatherer informer wasn't initialized
func (g *DataGatherer) Run(stopCh <-chan struct{}) error {
	return fmt.Errorf("data gatherer's informer was not initialized")
}

// WaitForCacheSync waits for the data gatherer's informers cache to sync.
func (g *DataGatherer) WaitForCacheSync(stopCh <-chan struct{}) error {
	return fmt.Errorf("timed out waiting for caches to sync")
}

func (g *DataGatherer) Equals(old datagatherer.DataGatherer) bool {
	dg, ok := old.(*DataGatherer)
	if !ok {
		return false
	}
	return !reflect.DeepEqual(g, dg)
}

// Fetch loads and returns the data from the LocalDatagatherer's dataPath
func (g *DataGatherer) Fetch() (interface{}, error) {
	dataBytes, err := ioutil.ReadFile(g.dataPath)
	if err != nil {
		return nil, err
	}
	return dataBytes, nil
}
