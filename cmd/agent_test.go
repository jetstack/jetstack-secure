package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	arktesting "github.com/jetstack/preflight/internal/cyberark/testing"
)

// TestOutputModes tests the different output modes of the agent command.
// It does this by running the agent command in a subprocess with the
// appropriate flags and configuration files.
// It assumes that the test is being run from the "cmd" directory and that
// the repository root is the parent directory of the current working directory.
func TestOutputModes(t *testing.T) {
	repoRoot := findRepoRoot(t)

	t.Run("localfile", func(t *testing.T) {
		runSubprocess(t, repoRoot, []string{
			"--agent-config-file", filepath.Join(repoRoot, "examples/localfile/config.yaml"),
			"--input-path", filepath.Join(repoRoot, "examples/localfile/input.json"),
			"--output-path", "/dev/null",
		})
	})

	t.Run("machinehub", func(t *testing.T) {
		arktesting.SkipIfNoEnv(t)

		t.Log("This test runs against a live service and has been known to flake. If you see timeout issues it's possible that the test is flaking and it could be unrelated to your changes.")

		runSubprocess(t, repoRoot, []string{
			"--agent-config-file", filepath.Join(repoRoot, "examples/machinehub/config.yaml"),
			"--input-path", filepath.Join(repoRoot, "examples/machinehub/input.json"),
			"--machine-hub",
		})
	})
}

// findRepoRoot returns the absolute path to the repository root.
// It assumes that the test is being run from the "cmd" directory.
func findRepoRoot(t *testing.T) string {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	repoRoot, err := filepath.Abs(filepath.Join(cwd, ".."))
	require.NoError(t, err)
	return repoRoot
}

// runSubprocess runs the current test in a subprocess with the given args.
// It sets the GO_CHILD environment variable to indicate to the subprocess
// that it should run the main function instead of the test function.
// It captures and logs the stdout and stderr of the subprocess.
// It fails the test if the subprocess exits with a non-zero status.
// It uses a timeout to avoid hanging indefinitely.
func runSubprocess(t *testing.T, repoRoot string, args []string) {
	if _, found := os.LookupEnv("GO_CHILD"); found {
		os.Args = append([]string{
			"preflight",
			"agent",
			"--log-level", "6",
			"--one-shot",
		}, args...)
		Execute()
		return
	}
	t.Log("Running child process", os.Args[0], "-test.run=^"+t.Name()+"$")
	ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+t.Name()+"$")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "GO_CHILD=true")
	err := cmd.Run()
	t.Logf("STDOUT\n%s\n", stdout.String())
	t.Logf("STDERR\n%s\n", stderr.String())
	require.NoError(t, err, fmt.Sprintf("Error: %v\nSTDERR: %s", err, stderr.String()))
}
