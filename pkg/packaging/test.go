package packaging

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/juju/errors"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/tester"
)

// TestPackage executes the test for a package
func TestPackage(ctx context.Context, pkg Package, verbose bool, timeout time.Duration) (int, int, error) {
	modules := make(map[string]*ast.Module)

	for name, content := range pkg.RegoText() {
		parsed, err := ast.ParseModule(name, content)
		if err != nil {
			return 0, 0, fmt.Errorf("Error parsing %s: %v", name, err)
		}
		modules[name] = parsed
	}

	for name, content := range pkg.RegoTestsText() {
		parsed, err := ast.ParseModule(name, content)
		if err != nil {
			return 0, 0, fmt.Errorf("Error parsing %s: %v", name, err)
		}
		modules[name] = parsed
	}

	runner := tester.NewRunner().
		EnableTracing(true).
		EnableFailureLine(true).
		SetModules(modules).
		SetTimeout(timeout)

	ch, err := runner.RunTests(ctx, nil)
	if err != nil {
		return 0, 0, errors.Trace(err)
	}

	reporter := tester.PrettyReporter{
		Verbose:     verbose,
		FailureLine: true,
		Output:      os.Stdout,
	}

	numFail, numTotal := 0, 0

	dup := make(chan *tester.Result)

	go func() {
		defer close(dup)

		for tr := range ch {
			if !tr.Pass() {
				numFail++
			}
			numTotal++
			dup <- tr
		}
	}()

	if err := reporter.Report(dup); err != nil {
		return numFail, numTotal, err
	}

	return numFail, numTotal, nil
}
