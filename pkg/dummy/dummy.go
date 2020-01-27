package dummy

import (
	"context"
	"io"
	"log"

	"github.com/jetstack/preflight/pkg/results"
	"github.com/open-policy-agent/opa/rego"
)

type readObject interface {
	NewReader(context.Context) (io.ReadCloser, error)
}

type writeObject interface {
	NewWriter(context.Context) io.WriteCloser
}

// ReportOnDummy evaluates the dummy rego rules and writes the output.
func ReportOnDummy(
	ctx context.Context,
	rules string,
) (*results.ResultCollection, error) {
	// Execute Open Policy Agent rules engine
	r := rego.New(
		rego.Query("data.preflight._0_dummy"),
		rego.Module("0-dummy.rego", rules),
		rego.Input(map[string]interface{}{}))

	rs, err := r.Eval(ctx)
	if err != nil {
		return nil, err
	}
	log.Printf("Executed Dummy Preflight package")

	// Convert result from OPA and serialize
	rc, err := results.NewResultCollectionFromRegoResultSet(&rs, []string{})
	if err != nil {
		return nil, err
	}

	return rc, nil
}
