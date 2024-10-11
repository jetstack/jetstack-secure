package logs

import (
	"errors"
	"log"
	"log/slog"
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

var Log *log.Logger

func init() {
	Log = slog.NewLogLogger(slog.Default().Handler(), slog.LevelDebug)
}

type LogOptions struct {
	Format logFormat
	Level  int
}

const (
	LogFormatText logFormat = "text"
	LogFormatJSON logFormat = "json"
)

type logFormat string

// String is used both by fmt.Print and by Cobra in help text
func (e *logFormat) String() string {
	if len(*e) == 0 {
		return string(LogFormatText)
	}
	return string(*e)
}

// Set must have pointer receiver to avoid changing the value of a copy
func (e *logFormat) Set(v string) error {
	switch v {
	case "text", "json":
		*e = logFormat(v)
		return nil
	default:
		return errors.New(`must be one of "text" or "json"`)
	}
}

// Type is only used in help text
func (e *logFormat) Type() string {
	return "string"
}

func SetupFlags(fs *pflag.FlagSet, logOptions *LogOptions) {
	var nfs cliflag.NamedFlagSets

	lfs := nfs.FlagSet("Logging")
	lfs.Var(&logOptions.Format,
		"log-format",
		"Log format (text or json)")

	lfs.IntVarP(&logOptions.Level,
		"log-level", "v", 1,
		"Log level (1-5).")

	for _, f := range nfs.FlagSets {
		fs.AddFlagSet(f)
	}
}

func (o *LogOptions) Initialize() logr.Logger {
	opts := &slog.HandlerOptions{
		// To avoid a breaking change in application configuration,
		// we negate the (configured) logr verbosity level to get the corresponding slog level
		Level: slog.Level(-o.Level),
	}
	var handler slog.Handler = slog.NewTextHandler(os.Stdout, opts)
	if o.Format == LogFormatJSON {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))

	log := logr.FromSlogHandler(handler)
	klog.SetLogger(log)
	return log
}
