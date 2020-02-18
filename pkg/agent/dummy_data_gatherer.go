package agent

import (
	"context"
	"fmt"

	"github.com/jetstack/preflight/pkg/datagatherer"
)

type dummyConfig struct {
	Param1            string `yaml:"param-1"`
	wantOnCreationErr bool
}

func (c *dummyConfig) NewDataGatherer(ctx context.Context) (datagatherer.DataGatherer, error) {
	if c.wantOnCreationErr {
		return nil, fmt.Errorf("an error")
	}
	return &dummyDataGatherer{
		Param1: c.Param1,
	}, nil
}

type dummyDataGatherer struct {
	Param1 string
}

func (c *dummyDataGatherer) Fetch() (interface{}, error) {
	return nil, nil
}
