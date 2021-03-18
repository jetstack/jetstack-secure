package agent

import (
	"context"
	"fmt"

	"github.com/jetstack/preflight/pkg/datagatherer"
)

type dummyConfig struct {
	AlwaysFail        bool `yaml:"always-fail"`
	FailedAttempts    int  `yaml:"failed-attempts"`
	wantOnCreationErr bool
}

func (c *dummyConfig) NewDataGatherer(ctx context.Context) (datagatherer.DataGatherer, error) {
	if c.wantOnCreationErr {
		return nil, fmt.Errorf("an error")
	}
	return &dummyDataGatherer{
		AlwaysFail:     c.AlwaysFail,
		FailedAttempts: c.FailedAttempts,
	}, nil
}

type dummyDataGatherer struct {
	AlwaysFail     bool
	attemptNumber  int
	FailedAttempts int
}

// Run starts the data gatherer's informers for resource collection.
// Returns error if the data gatherer informer wasn't initialized
func (g *dummyDataGatherer) Run(stopCh <-chan struct{}) error {
	return fmt.Errorf("data gatherer's informer was not initialized")
}

// WaitForCacheSync waits for the data gatherer's informers cache to sync.
func (g *dummyDataGatherer) WaitForCacheSync(stopCh <-chan struct{}) error {
	return fmt.Errorf("timed out waiting for caches to sync")
}

func (g *dummyDataGatherer) Equals(old datagatherer.DataGatherer) bool {
	return false
}

func (c *dummyDataGatherer) Fetch() (interface{}, error) {
	var err error
	if c.attemptNumber < c.FailedAttempts {
		err = fmt.Errorf("First %d attempts will fail", c.FailedAttempts)
	}
	if c.AlwaysFail {
		err = fmt.Errorf("This data gatherer will always fail")
	}
	c.attemptNumber++
	return nil, err
}
