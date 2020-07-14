package agent

import (
	"context"
	"fmt"

	"github.com/jetstack/preflight/pkg/datagatherer"
)

type dummyConfig struct {
	Param1            string `yaml:"param-1"`
	FailedAttempts    int    `yaml:"failed-attempts"`
	wantOnCreationErr bool
}

func (c *dummyConfig) NewDataGatherer(ctx context.Context) (datagatherer.DataGatherer, error) {
	if c.wantOnCreationErr {
		return nil, fmt.Errorf("an error")
	}
	return &dummyDataGatherer{
		Param1:         c.Param1,
		FailedAttempts: c.FailedAttempts,
	}, nil
}

type dummyDataGatherer struct {
	Param1         string
	attemptNumber  int
	FailedAttempts int
}

func (c *dummyDataGatherer) Fetch() (interface{}, error) {
	var err error
	if c.attemptNumber < c.FailedAttempts {
		err = fmt.Errorf("First %d attempts will fail", c.FailedAttempts)
	}
	if c.Param1 == "foo" {
		err = fmt.Errorf("Param1 cannot be foo")
	}
	c.attemptNumber++
	return nil, err
}
