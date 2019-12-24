package packaging

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/juju/errors"
	"github.com/open-policy-agent/opa/tester"
	util "github.com/open-policy-agent/opa/util/test"
)

// TestPackage executes the test for a package
func TestPackage(ctx context.Context, pkg Package) (int, error) {
	files := make(map[string]string, len(pkg.RegoText())+len(pkg.RegoTestsText()))
	for name, content := range pkg.RegoText() {
		files[name] = content
	}

	for name, content := range pkg.RegoTestsText() {
		files[name] = content
	}

	tmpDir, cleanup, err := util.MakeTempFS("", "preflight_test", files)
	if err != nil {
		return 0, fmt.Errorf("Cannot create temporary files: %v", err)
	}
	defer cleanup()

	fullPaths := []string{}
	for name := range files {
		fullPaths = append(fullPaths, path.Join(tmpDir, name))
	}
	modules, _, err := tester.Load(fullPaths, nil)

	runner := tester.NewRunner().
		EnableTracing(true).
		EnableFailureLine(true).
		SetModules(modules).
		SetTimeout(time.Second * 5)

	ch, err := runner.RunTests(ctx, nil)
	if err != nil {
		return 0, errors.Trace(err)
	}

	numFailures := 0

	for tr := range ch {
		if !tr.Pass() {
			numFailures++
		}
	}

	return numFailures, nil
}
