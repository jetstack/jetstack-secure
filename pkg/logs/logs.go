package logs

import (
	"io"
	"log"
	"log/slog"
	"os"

	"github.com/spf13/pflag"
)

// Deprecated: Log is a `log` logger, which is being phased out.
var Log = log.Default()

// Write logs to stdout by default
var logOutput io.Writer = os.Stdout

// SetOutput changes the global log output writer
func SetOutput(w io.Writer) {
	logOutput = w
}

var logLevel int

// AddFlags adds log related flags to the supplied flag set
func AddFlags(fs *pflag.FlagSet) {
	fs.IntVarP(&logLevel,
		"log-level", "v", int(slog.LevelInfo),
		"Set the logging verbosity. 8: >= ERROR, 4: >= WARN, 0: >=INFO, -4: >= DEBUG")
}

// Initialize configures the global log and slog loggers to write JSON to stdout.
//
// Errors, warnings, info and debug messages are all logged to stdout.
// By default, log messages with level >= INFO will be written.
// If verbose logging is enabled, log messages with level >= DEBUG will be written.
//
// The log module doesn't support levels. All its messages will be logged in
// JSON format, at INFO level.
func Initialize() {
	// This is a work around to remove the `vcert: ` prefix which is added by the vcert module.
	// See https://github.com/Venafi/vcert/pull/512
	log.SetPrefix("")
	slog.SetLogLoggerLevel(slog.LevelInfo)

	level := slog.Level(logLevel)
	logger := slog.New(
		slog.NewJSONHandler(logOutput, &slog.HandlerOptions{
			Level: level,
		}),
	)
	// This sets both the global slog logger and the global legacy log logger.
	slog.SetDefault(logger)
}
