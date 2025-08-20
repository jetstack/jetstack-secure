package cmd

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestAgentRunOneShot runs the agent in `--one-shot` mode and verifies that it exits
// after the first data gathering iteration.
func TestAgentRunOneShot(t *testing.T) {
	if _, found := os.LookupEnv("GO_CHILD"); found {
		// Silence the warning about missing pod name for event generation
		// TODO(wallrj): This should not be required when an `--input-file` has been supplied.
		t.Setenv("POD_NAME", "venafi-kubernetes-e2e")
		// Silence the error about missing kubeconfig.
		// TODO(wallrj): This should not be required when an `--input-file` has been supplied.
		t.Setenv("KUBECONFIG", "testdata/agent/one-shot/success/kubeconfig.yaml")

		os.Args = []string{
			"preflight",
			"agent",
			"--one-shot",
			// TODO(wallrj): This should not be required when an `--input-file` has been supplied.
			"--api-token=should-not-be-required",
			// TODO(wallrj): This should not be required when an `--input-file` has been supplied.
			"--install-namespace=default",
			"--agent-config-file=testdata/agent/one-shot/success/config.yaml",
			"--input-path=testdata/agent/one-shot/success/input.json",
			"--output-path=/dev/null",
			"-v=9",
		}
		Execute()
		return
	}
	t.Log("Running child process")
	ctx, cancel := context.WithTimeout(t.Context(), time.Second*3)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAgentRunOneShot$")
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(
		os.Environ(),
		"GO_CHILD=true",
	)
	err := cmd.Run()

	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	t.Logf("STDOUT\n%s\n", stdoutStr)
	t.Logf("STDERR\n%s\n", stderrStr)
	require.NoError(t, err, context.Cause(ctx))
}
