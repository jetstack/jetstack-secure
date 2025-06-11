// Package datagatherer provides the DataGatherer interface.
package datagatherer

import "context"

// Config is the configuration of a DataGatherer.
type Config interface {
	// NewDataGatherer constructs a DataGatherer with an specific configuration.
	NewDataGatherer(ctx context.Context) (DataGatherer, error)
}

// DataGatherer is the interface for Data Gatherers. Data Gatherers are in charge of fetching data from a certain cloud provider API or Kubernetes component.
type DataGatherer interface {
	// Fetch retrieves data.
	// count is the number of items that were discovered. A negative count means the number
	// of items was indeterminate.
	Fetch() (data interface{}, count int, err error)
	// Run starts the data gatherer's informers for resource collection.
	// Returns error if the data gatherer informer wasn't initialized
	Run(ctx context.Context) error
	// WaitForCacheSync waits for the data gatherer's informers cache to sync.
	WaitForCacheSync(ctx context.Context) error
	// Delete, clear the cache of the DataGatherer if one is being used
	Delete() error
}
