package isito

import (
	"context"
	"testing"
)

// TODO: Implement proper testing.
func TestStuff(t *testing.T) {
	c := Config{
		// Set this to your kubeconfig path for local testing against a cluster.
		KubeConfigPath: "",
	}

	dg, err := c.NewDataGatherer(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	results, err := dg.Fetch()
	if err != nil {
		t.Fatal(err)
	}

	t.Log(results)
}
