// Package datagatherer provides the DataGatherer interface.
package datagatherer

// DataGatherer is the interface for Data Gatherers. Data Gatherers are in charge of fetching data from a certain cloud provider API or Kubernetes component.
type DataGatherer interface {
	// Fetch retrieves data.
	Fetch() (interface{}, error)
}
