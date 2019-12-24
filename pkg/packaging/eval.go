package packaging

import (
	"context"
	"fmt"

	"github.com/jetstack/preflight/pkg/results"
	"github.com/open-policy-agent/opa/rego"
)

// EvalPackage evaluates the rules in a package given an input.
func EvalPackage(ctx context.Context, pkg Package, input interface{}) (*results.ResultCollection, error) {
	allResults := rego.ResultSet{}
	for file, text := range pkg.RegoText() {
		// Execute Open Policy Agent rules engine
		r := rego.New(
			rego.Query(pkg.PolicyManifest().RootQuery),
			rego.Module(file, text),
			rego.Input(input),
		)

		rs, err := r.Eval(ctx)
		if err != nil {
			return nil, fmt.Errorf("cannot evaluate rules in %q`: %v", file, err)
		}

		allResults = append(allResults, rs...)
	}

	rc, err := results.NewResultCollectionFromRegoResultSet(&allResults)
	if err != nil {
		return nil, fmt.Errorf("cannot read results from rego: %s", err)
	}

	return rc, nil
}
