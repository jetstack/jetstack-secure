package cmd

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestAgentRunOneShot runs the agent in `--one-shot` mode and verifies that it exits
// after the first data gathering iteration.
func TestAgentRunOneShot(t *testing.T) {
	if _, found := os.LookupEnv("GO_CHILD"); found {
		os.Args = []string{
			"preflight",
			"agent",
			"--one-shot",
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

func TestOutputModes(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	repoRoot, err := filepath.Abs(filepath.Join(cwd, ".."))
	require.NoError(t, err)

	type testCase struct {
		args []string
	}

	tests := map[string]testCase{
		"machinehub": {
			args: []string{
				"--agent-config-file", "examples/machinehub/config.yaml",
				"--input-path", "examples/machinehub/input.json",
				"--machine-hub",
			},
		},
	}

	for name, testSpec := range tests {
		t.Run(name, func(t *testing.T) {
			if _, found := os.LookupEnv("GO_CHILD"); found {
				os.Args = append([]string{
					"preflight",
					"agent",
					"--log-level", "6",
					"--one-shot",
				}, testSpec.args...)

				Execute()
				return
			}
			t.Log("Running child process")
			ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)
			defer cancel()

			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+t.Name()+"$")
			var (
				stdout bytes.Buffer
				stderr bytes.Buffer
			)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Dir = repoRoot

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
		})
	}
}
