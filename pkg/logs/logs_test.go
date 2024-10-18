package logs_test

import (
	"bytes"
	"context"
	"errors"
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
			fs := pflag.NewFlagSet("test-logs", pflag.ExitOnError)
			logs.AddFlags(fs)
			fs.Parse(strings.Split(flags, " "))
			logs.Initialize()

			// Remember to not use `assert.Equal` from within the sub-process
			// since (for some reason) `assert.Equal` doesn't cause the test
			// sub-process to exit with 1. Instead, it exits with 0, which means
			// we can't notice the error from the parent process.
			require.Equal(t, "", log.Prefix(), "logs.Initialize should remove the `vCert: ` prefix from the global log logger")
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
			name:        "help",
			flags:       "-h",
			expectError: true,
			expectStdout: `
pflag: help requested
`,
			expectStderr: `
Usage of test-logs:
      --logging-format string   Sets the log format. Permitted formats: "json", "text". (default "json")
  -v, --v Level                 number for the log level verbosity
      --vmodule pattern=N,...   comma-separated list of pattern=N settings for file-filtered logging (only works for text log format)
`,
		},
		{
			name:        "unrecognized-flag",
			flags:       "--foo",
			expectError: true,
			expectStdout: `
unknown flag: --foo
`,
			expectStderr: `
unknown flag: --foo
Usage of test-logs:
      --logging-format string   Sets the log format. Permitted formats: "json", "text". (default "json")
  -v, --v Level                 number for the log level verbosity
      --vmodule pattern=N,...   comma-separated list of pattern=N settings for file-filtered logging (only works for text log format)
`,
		},
		{
			name:        "logging-format-unrecognized",
			flags:       "--logging-format=foo",
			expectError: true,
			expectStderr: `
Error in logging configuration: format: Invalid value: "foo": Unsupported log format
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
			flags: "--v=3",
			expectStdout: `
{"ts":0000000000000.000,"caller":"logs/logs.go:000","msg":"log Print","source":"vcert","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"slog Info","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"slog Warn","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"klog Info","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"klog Warning","v":0}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"klog InfoS","v":0,"key":"value"}
{"ts":0000000000000.000,"logger":"foo","caller":"logs/logs_test.go:000","msg":"Contextual Info Level 3","v":3,"key":"value"}
`,
			expectStderr: `
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"slog Error"}
{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"klog Error","err":"fake-error"}
{"ts":0000000000000.000,"logger":"foo","caller":"logs/logs_test.go:000","msg":"Contextual error","key":"value","err":"fake-error"}
`,
		},
		{
			name:  "vmodule-level-3",
			flags: "--logging-format=text --vmodule=logs_test=3",
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
				require.Equal(t, 2, target.ExitCode(), "Flag parsing failures should always result in exit code 2")
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
	timestampRegexpKlog   = regexp.MustCompile(`\d{4} \d{2}:\d{2}:\d{2}\.\d{6}   \d{5}`)
	timestampRegexpJSON   = regexp.MustCompile(`"ts":\d+\.\d+`)
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
