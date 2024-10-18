package logs

import (
	"bytes"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/component-base/featuregate"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
	_ "k8s.io/component-base/logs/json/register"
)

// venafi-kubernetes-agent follows [Kubernetes Logging Conventions]
// and writes logs in [Kubernetes JSON logging format] by default.
// It does not support named levels (AKA severity), instead it uses arbitrary levels.
// Errors are logged to stderr and Info messages to stdout, because that is how
// some cloud logging systems (notably Google Cloud Logs Explorer) assign a
// severity (INFO or ERROR) in the UI.
// Messages logged using the legacy log module are all logged as Info messages
// with level=0.
//
// Further reading:
// - [Kubernetes logging conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md)
// - [Kubernetes JSON logging format](https://kubernetes.io/docs/concepts/cluster-administration/system-logs/#json-log-format)
// - [Why not named levels, like Info/Warning/Error?](https://github.com/go-logr/logr?tab=readme-ov-file#why-not-named-levels-like-infowarningerror)
// - [GKE logs best practices](https://cloud.google.com/kubernetes-engine/docs/concepts/about-logs#best_practices)
// - [Structured Logging KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-instrumentation/1602-structured-logging/README.md)
// - [Examples of using k8s.io/component-base/logs](https://github.com/kubernetes/kubernetes/tree/master/staging/src/k8s.io/component-base/logs/example),
//   upon which this code was based.

var (
	// This is the Agent's logger. For now, it is still a *log.Logger, but we
	// mean to migrate everything to slog with the klog backend. We avoid using
	// log.Default because log.Default is already used by the VCert library, and
	// we need to keep the agent's logger from the VCert's logger to be able to
	// remove the `vCert: ` prefix from the VCert logs.
	Log *log.Logger

	// All but the essential logging flags will be hidden to avoid overwhelming
	// the user. The hidden flags can still be used. For example if a user does
	// not like the split-stream behavior and a Venafi field engineer can
	// instruct them to patch --log-json-split-stream=false on to the Deployment
	// arguments.
	visibleFlagNames = sets.New[string]("v", "vmodule", "logging-format")
	// This default logging configuration will be updated with values from the
	// logging flags, even those that are hidden.
	configuration = logsapi.NewLoggingConfiguration()
	// Logging features will be added to this feature gate, but the
	// feature-gates flag will be hidden from the user.
	features = featuregate.NewFeatureGate()
)

func init() {
	runtime.Must(logsapi.AddFeatureGates(features))
	// Turn on ALPHA options to enable the split-stream logging options.
	runtime.Must(features.OverrideDefault(logsapi.LoggingAlphaOptions, true))
}

// AddFlags adds log related flags to the supplied flag set.
//
// The default logging format is changed to JSON. The default in Kubernetes
// component base is "text", for backwards compatibility, but that is not a
// concern for venafi-kubernetes-agent.
// The split-stream options are enabled by default, so that errors are logged to
// stderr and info to stdout, allowing cloud logging systems to assign an
// severity INFO or ERROR to the messages.
func AddFlags(fs *pflag.FlagSet) {
	var tfs pflag.FlagSet
	logsapi.AddFlags(configuration, &tfs)
	features.AddFlag(&tfs)
	tfs.VisitAll(func(f *pflag.Flag) {
		if !visibleFlagNames.Has(f.Name) {
			tfs.MarkHidden(f.Name)
		}
		// The default is "text" and the usage string includes details about how
		// JSON logging is only available when BETA logging features are
		// enabled, but that's not relevant here because the feature is enabled
		// by default.
		if f.Name == "logging-format" {
			f.Usage = `Sets the log format. Permitted formats: "json", "text".`
			f.DefValue = "json"
			runtime.Must(f.Value.Set("json"))
		}
		if f.Name == "log-text-split-stream" {
			f.DefValue = "true"
			runtime.Must(f.Value.Set("true"))
		}
		if f.Name == "log-json-split-stream" {
			f.DefValue = "true"
			runtime.Must(f.Value.Set("true"))
		}
	})
	fs.AddFlagSet(&tfs)
}

// Initialize uses k8s.io/component-base/logs, to configure the following global
// loggers: log, slog, and klog. All are configured to write in the same format.
func Initialize() {
	// This configures the global logger in klog *and* slog, if compiled
	// with Go >= 1.21.
	logs.InitLogs()
	if err := logsapi.ValidateAndApply(configuration, features); err != nil {
		fmt.Fprintf(os.Stderr, "Error in logging configuration: %v\n", err)
		os.Exit(2)
	}

	// Thanks to logs.InitLogs(), slog.Default() now uses klog as its backend.
	// Thus, the client-go library, which relies on klog.Info, has the same
	// logger as the agent, which still uses log.Printf.
	slog := slog.Default()

	Log = &log.Logger{}
	Log.SetOutput(logToSlogWriter{slog: slog, source: "agent"})

	// Let's make sure the VCert library, which is the only library we import to
	// be using the global log.Default, also uses the common slog logger.
	vcertLog := log.Default()
	vcertLog.SetOutput(logToSlogWriter{slog: slog, source: "vcert"})
	// This is a work around for a bug in vcert where it adds a `vCert: ` prefix
	// to the global log logger. It can be removed when this is fixed upstream
	// in vcert:  https://github.com/Venafi/vcert/pull/512
	vcertLog.SetPrefix("")
}

type logToSlogWriter struct {
	slog   *slog.Logger
	source string
}

func (w logToSlogWriter) Write(p []byte) (n int, err error) {
	// log.Printf writes a newline at the end of the message, so we need to trim
	// it.
	p = bytes.TrimSuffix(p, []byte("\n"))

	message := string(p)
	if isCritical(message) {
		w.slog.With("source", w.source).Error(message)
	} else {
		w.slog.With("source", w.source).Info(message)
	}
	return len(p), nil
}

func isCritical(msg string) bool {
	// You can implement more robust logic to detect critical log messages
	return strings.Contains(msg, "FATAL") || strings.Contains(msg, "ERROR")
}
