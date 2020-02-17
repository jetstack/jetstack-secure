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
	Fetch() (interface{}, error)
}
