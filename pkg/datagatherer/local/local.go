package local

import (
	"context"
	"io/ioutil"
)

type LocalDataGatherer struct {
	dataPath string
}

type LocalDataGathererConfig struct {
	DataPath string `mapstructure:"data-path"`
}

// NewLocalDataGatherer returns a LocalDatagatherer with the dataPath provided.
func NewLocalDataGatherer(ctx context.Context, config *LocalDataGathererConfig) *LocalDataGatherer {
	return &LocalDataGatherer{
		dataPath: config.DataPath,
	}
}

// Fetch loads and returns the data from the LocalDatagatherer's dataPath
func (g *LocalDataGatherer) Fetch() (interface{}, error) {
	dataBytes, err := ioutil.ReadFile(g.dataPath)
	if err != nil {
		return nil, err
	}
	return dataBytes, nil
}
