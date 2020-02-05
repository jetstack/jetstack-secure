package local

import "io/ioutil"

type LocalDataGatherer struct {
	dataPath string
}

// NewLocalDataGatherer returns a LocalDatagatherer with the dataPath provided.
func NewLocalDataGatherer(dataPath string) *LocalDataGatherer {
	return &LocalDataGatherer{
		dataPath: dataPath,
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
