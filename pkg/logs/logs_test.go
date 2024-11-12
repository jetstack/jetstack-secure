package logs_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	_ "github.com/Venafi/vcert/v5"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/pkg/logs"
)

// TestLogs demonstrates how the logging flags affect the logging output.
//
// The test executes itself with as a sub-process to avoid mutating the global
// logging configuration.
//
// Inspired by:
// - https://stackoverflow.com/a/67945462
// - https://go.dev/src/flag/flag_test.go (TestExitCode)
func TestLogs(t *testing.T) {
	if flags, found := os.LookupEnv("GO_CHILD_FLAG"); found {
		if _, found := os.LookupEnv("GO_CHILD_SKIP_INITIALIZE"); !found {
			fs := pflag.NewFlagSet("test-logs", pflag.ContinueOnError)
			fs.SetOutput(io.Discard)
			logs.AddFlags(fs)
			if err := fs.Parse(strings.Split(flags, " ")); err != nil {
				exitCode := 0
				if errors.Is(err, pflag.ErrHelp) {
					fmt.Fprint(os.Stdout, fs.FlagUsages())
					os.Exit(exitCode)
				} else {
					exitCode := 1
					klog.ErrorS(err, "Exiting due to error", "exit-code", exitCode)
					klog.FlushAndExit(time.Second, exitCode)
				}
			}
			if err := logs.Initialize(); err != nil {
				exitCode := 1
				klog.ErrorS(err, "Exiting due to error", "exit-code", exitCode)
				klog.FlushAndExit(time.Second, exitCode)
			}
		}

		log.Print("log Print")
		slog.Info("slog Info")
		slog.Warn("slog Warn")
		slog.Error("slog Error")
		klog.Info("klog Info")
		klog.Warning("klog Warning")
		klog.ErrorS(errors.New("fake-error"), "klog Error")
		klog.InfoS("klog InfoS", "key", "value")
		logger := klog.FromContext(context.Background()).WithName("foo")
		logger.V(3).Info("Contextual Info Level 3", "key", "value")
		logger.Error(errors.New("fake-error"), "Contextual error", "key", "value")

		klog.FlushAndExit(time.Second, 0)
	}

	tests := []struct {
		name          string
		flags         string
		skipIntialize bool
		expectError   bool
		expectStdout  string
		expectStderr  string
	}{
		{
			name:  "help",
			flags: "-h",
			expectStdout: `
  -v, --log-level Level         number for the log level verbosity. 0=Info, 1=Debug, 2=Trace. Use 3-10 for even greater detail. (default: 0)
      --logging-format string   Sets the log format. Permitted formats: "json", "text". (default "text")
      --vmodule pattern=N,...   comma-separated list of pattern=N settings for file-filtered logging (only works for text log format)
`,
		},
		{
			name:        "unrecognized-flag",
			flags:       "--foo",
			expectError: true,
			expectStderr: `
E0000 00:00:00.000000   00000 logs_test.go:000] "Exiting due to error" err="unknown flag: --foo" exit-code=1
`,
		},
		{
			name:        "v-long-form-not-available",
			flags:       "--v=3",
			expectError: true,
			expectStderr: `
E0000 00:00:00.000000   00000 logs_test.go:000] "Exiting due to error" err="unknown flag: --v" exit-code=1
`,
		},
		{
			name:        "logging-format-unrecognized",
			flags:       "--logging-format=foo",
			expectError: true,
			expectStderr: `
E0000 00:00:00.000000   00000 logs_test.go:000] "Exiting due to error" err="Error in logging configuration: format: Invalid value: \"foo\": Unsupported log format" exit-code=1
`,
		},
		{
			name:          "original-defaults",
			flags:         "",
			skipIntialize: true,
			expectStderr: `
vCert: 0000/00/00 00:00:00 log Print
vCert: 0000/00/00 00:00:00 INFO slog Info
vCert: 0000/00/00 00:00:00 WARN slog Warn
vCert: 0000/00/00 00:00:00 ERROR slog Error
I0000 00:00:00.000000   00000 logs_test.go:000] klog Info
W0000 00:00:00.000000   00000 logs_test.go:000] klog Warning
E0000 00:00:00.000000   00000 logs_test.go:000] "klog Error" err="fake-error"
I0000 00:00:00.000000   00000 logs_test.go:000] "klog InfoS" key="value"
E0000 00:00:00.000000   00000 logs_test.go:000] "Contextual error" err="fake-error" logger="foo" key="value"
`,
		},
		{
			name:  "modified-defaults",
			flags: "",
			expectStdout: `
I0000 00:00:00.000000   00000 logs.go:000] "log Print" source="vcert"
I0000 00:00:00.000000   00000 logs_test.go:000] "slog Info"
I0000 00:00:00.000000   00000 logs_test.go:000] klog Info
I0000 00:00:00.000000   00000 logs_test.go:000] "klog InfoS" key="value"
`,
			expectStderr: `
W0000 00:00:00.000000   00000 logs_test.go:000] "slog Warn"
E0000 00:00:00.000000   00000 logs_test.go:000] "slog Error"
W0000 00:00:00.000000   00000 logs_test.go:000] klog Warning
E0000 00:00:00.000000   00000 logs_test.go:000] "klog Error" err="fake-error"
E0000 00:00:00.000000   00000 logs_test.go:000] "Contextual error" err="fake-error" logger="foo" key="value"
`,
		},
		{
			name:  "logging-format-json",
			flags: "--logging-format=json",
			expectStdout: `
{"ts":0000000000000.000,"caller":"logs/logs.go:000","msg":"log Print","source":"vcert","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"slog Info","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"slog Warn","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"klog Info","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"klog Warning","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"klog InfoS","v":0,"key":"value"}
`,
			expectStderr: `
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"slog Error"}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"klog Error","err":"fake-error"}
{"ts":0000000000000.000,"logger":"foo","caller":"logs/logs_test.go:000","msg":"Contextual error","key":"value","err":"fake-error"}
`,
		},
		{
			name:  "log-json-split-stream-false",
			flags: "--logging-format=json --log-json-split-stream=false",
			expectStderr: `
{"ts":0000000000000.000,"caller":"logs/logs.go:000","msg":"log Print","source":"vcert","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"slog Info","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"slog Warn","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"slog Error"}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"klog Info","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"klog Warning","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"klog Error","err":"fake-error"}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"klog InfoS","v":0,"key":"value"}
{"ts":0000000000000.000,"logger":"foo","caller":"logs/logs_test.go:000","msg":"Contextual error","key":"value","err":"fake-error"}
`,
		},
		{
			name:  "logging-format-text",
			flags: "--logging-format=text",
			expectStdout: `
I0000 00:00:00.000000   00000 logs.go:000] "log Print" source="vcert"
I0000 00:00:00.000000   00000 logs_test.go:000] "slog Info"
I0000 00:00:00.000000   00000 logs_test.go:000] klog Info
I0000 00:00:00.000000   00000 logs_test.go:000] "klog InfoS" key="value"
`,
			expectStderr: `
W0000 00:00:00.000000   00000 logs_test.go:000] "slog Warn"
E0000 00:00:00.000000   00000 logs_test.go:000] "slog Error"
W0000 00:00:00.000000   00000 logs_test.go:000] klog Warning
E0000 00:00:00.000000   00000 logs_test.go:000] "klog Error" err="fake-error"
E0000 00:00:00.000000   00000 logs_test.go:000] "Contextual error" err="fake-error" logger="foo" key="value"
`,
		},
		{
			name:  "log-text-split-stream-false",
			flags: "--logging-format=text --log-text-split-stream=false",
			expectStderr: `
I0000 00:00:00.000000   00000 logs.go:000] "log Print" source="vcert"
I0000 00:00:00.000000   00000 logs_test.go:000] "slog Info"
W0000 00:00:00.000000   00000 logs_test.go:000] "slog Warn"
E0000 00:00:00.000000   00000 logs_test.go:000] "slog Error"
I0000 00:00:00.000000   00000 logs_test.go:000] klog Info
W0000 00:00:00.000000   00000 logs_test.go:000] klog Warning
E0000 00:00:00.000000   00000 logs_test.go:000] "klog Error" err="fake-error"
I0000 00:00:00.000000   00000 logs_test.go:000] "klog InfoS" key="value"
E0000 00:00:00.000000   00000 logs_test.go:000] "Contextual error" err="fake-error" logger="foo" key="value"
`,
		},
		{
			name:  "v-level-3",
			flags: "-v=3",
			expectStdout: `
I0000 00:00:00.000000   00000 logs.go:000] "log Print" source="vcert"
I0000 00:00:00.000000   00000 logs_test.go:000] "slog Info"
I0000 00:00:00.000000   00000 logs_test.go:000] klog Info
I0000 00:00:00.000000   00000 logs_test.go:000] "klog InfoS" key="value"
I0000 00:00:00.000000   00000 logs_test.go:000] "Contextual Info Level 3" logger="foo" key="value"
`,
			expectStderr: `
W0000 00:00:00.000000   00000 logs_test.go:000] "slog Warn"
E0000 00:00:00.000000   00000 logs_test.go:000] "slog Error"
W0000 00:00:00.000000   00000 logs_test.go:000] klog Warning
E0000 00:00:00.000000   00000 logs_test.go:000] "klog Error" err="fake-error"
E0000 00:00:00.000000   00000 logs_test.go:000] "Contextual error" err="fake-error" logger="foo" key="value"
`,
		},
		{
			name:  "log-level-3",
			flags: "--log-level=3",
			expectStdout: `
I0000 00:00:00.000000   00000 logs.go:000] "log Print" source="vcert"
I0000 00:00:00.000000   00000 logs_test.go:000] "slog Info"
I0000 00:00:00.000000   00000 logs_test.go:000] klog Info
I0000 00:00:00.000000   00000 logs_test.go:000] "klog InfoS" key="value"
I0000 00:00:00.000000   00000 logs_test.go:000] "Contextual Info Level 3" logger="foo" key="value"
`,
			expectStderr: `
W0000 00:00:00.000000   00000 logs_test.go:000] "slog Warn"
E0000 00:00:00.000000   00000 logs_test.go:000] "slog Error"
W0000 00:00:00.000000   00000 logs_test.go:000] klog Warning
E0000 00:00:00.000000   00000 logs_test.go:000] "klog Error" err="fake-error"
E0000 00:00:00.000000   00000 logs_test.go:000] "Contextual error" err="fake-error" logger="foo" key="value"
`,
		},
		{
			name:  "vmodule-level-3",
			flags: "--vmodule=logs_test=3",
			expectStdout: `
I0000 00:00:00.000000   00000 logs.go:000] "log Print" source="vcert"
I0000 00:00:00.000000   00000 logs_test.go:000] "slog Info"
I0000 00:00:00.000000   00000 logs_test.go:000] klog Info
I0000 00:00:00.000000   00000 logs_test.go:000] "klog InfoS" key="value"
I0000 00:00:00.000000   00000 logs_test.go:000] "Contextual Info Level 3" logger="foo" key="value"
`,
			expectStderr: `
W0000 00:00:00.000000   00000 logs_test.go:000] "slog Warn"
E0000 00:00:00.000000   00000 logs_test.go:000] "slog Error"
W0000 00:00:00.000000   00000 logs_test.go:000] klog Warning
E0000 00:00:00.000000   00000 logs_test.go:000] "klog Error" err="fake-error"
E0000 00:00:00.000000   00000 logs_test.go:000] "Contextual error" err="fake-error" logger="foo" key="value"
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestLogs$", "-test.v")
			var (
				stdout bytes.Buffer
				stderr bytes.Buffer
			)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Env = append(
				os.Environ(),
				"GO_CHILD_FLAG="+test.flags,
			)
			if test.skipIntialize {
				cmd.Env = append(
					cmd.Env,
					"GO_CHILD_SKIP_INITIALIZE=true",
				)
			}
			err := cmd.Run()

			t.Logf("FLAGS\n%s\n", test.flags)
			// Remove the standard output generated by `-test.v`
			stdoutStr := strings.TrimPrefix(stdout.String(), "=== RUN   TestLogs\n")
			stderrStr := stderr.String()
			t.Logf("STDOUT\n%s\n", stdoutStr)
			t.Logf("STDERR\n%s\n", stderrStr)
			if test.expectError {
				var target *exec.ExitError
				require.ErrorAs(t, err, &target)
				require.Equal(t, 1, target.ExitCode(), "Flag parsing failures should always result in exit code 1")
				t.Logf("ERROR: %v", err)
			} else {
				require.NoError(t, err)
			}

			// This trick helps with the readability of the table test: we can
			// have the first "expected" log line at the same level as the other
			// lines.
			test.expectStdout = strings.TrimPrefix(test.expectStdout, "\n")
			test.expectStderr = strings.TrimPrefix(test.expectStderr, "\n")

			require.Equal(t, test.expectStdout, replaceWithStaticTimestamps(stdoutStr), "stdout doesn't match")
			require.Equal(t, test.expectStderr, replaceWithStaticTimestamps(stderrStr), "stderr doesn't match")
		})
	}
}

var (
	timestampRegexpStdLog = regexp.MustCompile(`\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`)
	timestampRegexpKlog   = regexp.MustCompile(`\d{4} \d{2}:\d{2}:\d{2}\.\d{6} +\d+`)
	timestampRegexpJSON   = regexp.MustCompile(`"ts":\d+\.?\d*`)
	fileAndLineRegexpJSON = regexp.MustCompile(`"caller":"([^"]+).go:\d+"`)
	fileAndLineRegexpKlog = regexp.MustCompile(` ([^:]+).go:\d+`)
)

// Replaces the klog and JSON timestamps with a static timestamp to make it
// easier to assert the logs. It also replaces the line number with 000 as it
// often changes.
//
//	I1018 15:12:57.953433   22183 logs.go:000] log
//	{"ts":1729258473588.828,"caller":"log/log.go:000","msg":"log Print","v":0}
//	2024/10/18 15:40:50 log Print
//
// to the fixed:
//
//	I0000 00:00:00.000000   00000 logs.go:000] log
//	{"ts":0000000000000.000,"caller":"log/log.go:000","msg":"log Print","v":0}
//	0000/00/00 00:00:00 log Print
func replaceWithStaticTimestamps(input string) string {
	input = timestampRegexpKlog.ReplaceAllString(input, "0000 00:00:00.000000   00000")
	input = timestampRegexpJSON.ReplaceAllString(input, `"ts":0000000000000.000`)
	input = timestampRegexpStdLog.ReplaceAllString(input, "0000/00/00 00:00:00")
	input = fileAndLineRegexpJSON.ReplaceAllString(input, `"caller":"$1.go:000"`)
	input = fileAndLineRegexpKlog.ReplaceAllString(input, " $1.go:000")
	return input
}

func Test_replaceWithStaticTimestamps(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "klog",
			input:    `I1018 15:20:42.861239    2386 logs_test.go:13] "Contextual Info Level 3" logger="foo" key="value"`,
			expected: `I0000 00:00:00.000000   00000 logs_test.go:000] "Contextual Info Level 3" logger="foo" key="value"`,
		},
		{
			name:     "json-with-nanoseconds",
			input:    `{"ts":1729270111728.125,"caller":"logs/logs_test.go:000","msg":"slog Warn","v":0}`,
			expected: `{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"slog Warn","v":0}`,
		},
		{
			name:     "json-might-not-have-nanoseconds",
			input:    `{"ts":1729270111728,"caller":"logs/logs_test.go:000","msg":"slog Info","v":0}`,
			expected: `{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"slog Info","v":0}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, replaceWithStaticTimestamps(test.input))
		})
	}
}

func TestLogToSlogWriter(t *testing.T) {
	// This test makes sure that all the agent's remaining Log calls are correctly
	// translated to slog.Error calls where appropriate.
	//
	// This list was generated using:
	//  git grep -i "log\.\(print\|fatal\)" pkg/ cmd/  | fgrep -e error -e failed
	given := strings.TrimPrefix(`
failed to complete initial sync of %q data gatherer %q: %v
error messages will not show in the pod's events because the POD_NAME environment variable is empty
retrying in %v after error: %s
datagatherer informer for %q has failed and is backing off due to error: %s
this is a happy log that should show as INFO`, "\n")
	expect := strings.TrimPrefix(`
level=ERROR msg="failed to complete initial sync of %!q(MISSING) data gatherer %!q(MISSING): %!v(MISSING)" source=agent
level=ERROR msg="error messages will not show in the pod's events because the POD_NAME environment variable is empty" source=agent
level=ERROR msg="retrying in %!v(MISSING) after error: %!s(MISSING)" source=agent
level=ERROR msg="datagatherer informer for %!q(MISSING) has failed and is backing off due to error: %!s(MISSING)" source=agent
level=INFO msg="this is a happy log that should show as INFO" source=agent
`, "\n")

	gotBuf := &bytes.Buffer{}
	slogHandler := slog.NewTextHandler(gotBuf, &slog.HandlerOptions{
		// Remove the timestamp from the logs so that we can compare them.
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "time" {
				return slog.Attr{}
			}
			return a
		},
	})
	slogLogger := slog.New(slogHandler)

	logger := log.New(&bytes.Buffer{}, "", 0)
	logger.SetOutput(logs.LogToSlogWriter{Slog: slogLogger, Source: "agent"})

	for _, line := range strings.Split(given, "\n") {
		// Simulate the current agent's logs.
		logger.Printf(line)
	}

	assert.Equal(t, expect, gotBuf.String())
}
