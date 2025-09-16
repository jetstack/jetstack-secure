package logs

import (
	"bytes"
	"fmt"
	"log"
	"log/slog"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/component-base/featuregate"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
	ctrlruntimelog "sigs.k8s.io/controller-runtime/pkg/log"

	_ "k8s.io/component-base/logs/json/register"
)

// venafi-kubernetes-agent follows [Kubernetes Logging Conventions] and writes
// logs in [Kubernetes text logging format] by default. It does not support
// named levels (aka. severity), instead it uses arbitrary levels. Errors and
// warnings are logged to stderr and Info messages to stdout, because that is
// how some cloud logging systems (notably Google Cloud Logs Explorer) assign a
// severity (INFO or ERROR) in the UI. The agent's and vcert's logs are written
// logged as Info messages with level=0.
//
// Further reading:
//  - [Kubernetes logging conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md)
//  - [Kubernetes text logging format](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#text-logging-format)
//  - [Why not named levels, like Info/Warning/Error?](https://github.com/go-logr/logr?tab=readme-ov-file#why-not-named-levels-like-infowarningerror)
//  - [GKE logs best practices](https://cloud.google.com/kubernetes-engine/docs/concepts/about-logs#best_practices)
//  - [Structured Logging KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-instrumentation/1602-structured-logging/README.md)
//  - [Examples of using k8s.io/component-base/logs](https://github.com/kubernetes/kubernetes/tree/master/staging/src/k8s.io/component-base/logs/example),
//    upon which this code was based.

var (

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

const (
	// Standard log verbosity levels.
	// Use these instead of integers in venafi-kubernetes-agent code.
	Info  = 0
	Debug = 1
	Trace = 2
)

func init() {
	runtime.Must(logsapi.AddFeatureGates(features))
	// Turn on ALPHA options to enable the split-stream logging options.
	runtime.Must(features.OverrideDefault(logsapi.LoggingAlphaOptions, true))
}

// AddFlags adds log related flags to the supplied flag set.
//
// The split-stream options are enabled by default, so that errors are logged to
// stderr and info to stdout, allowing cloud logging systems to assign a
// severity INFO or ERROR to the messages.
func AddFlags(fs *pflag.FlagSet) {
	var tfs pflag.FlagSet
	logsapi.AddFlags(configuration, &tfs)
	features.AddFlag(&tfs)
	tfs.VisitAll(func(f *pflag.Flag) {
		if !visibleFlagNames.Has(f.Name) {
			_ = tfs.MarkHidden(f.Name)
		}

		// The original usage string includes details about how
		// JSON logging is only available when BETA logging features are
		// enabled, but that's not relevant here because the feature is enabled
		// by default.
		if f.Name == "logging-format" {
			f.Usage = `Sets the log format. Permitted formats: "json", "text".`
		}
		if f.Name == "log-text-split-stream" {
			f.DefValue = "true"
			runtime.Must(f.Value.Set("true"))
		}
		if f.Name == "log-json-split-stream" {
			f.DefValue = "true"
			runtime.Must(f.Value.Set("true"))
		}

		// Since `--v` (which is the long form of `-v`) isn't the standard in
		// our projects (it only exists in cert-manager, webhook, and such),
		// let's rename it to the more common `--log-level`, which appears in
		// openshift-routes, csi-driver, trust-manager, and approver-policy.
		// More details at:
		// https://github.com/jetstack/jetstack-secure/pull/596#issuecomment-2421708181
		if f.Name == "v" {
			f.Name = "log-level"
			f.Shorthand = "v"
			f.Usage = fmt.Sprintf("%s. 0=Info, 1=Debug, 2=Trace. Use 6-9 for increasingly verbose HTTP request logging. (default: 0)", f.Usage)
		}
	})
	fs.AddFlagSet(&tfs)
}

// Initialize uses k8s.io/component-base/logs, to configure the following global
// loggers: log, slog, and klog. All are configured to write in the same format.
func Initialize() error {
	// This configures the global logger in klog *and* slog, if compiled with Go
	// >= 1.21.
	logs.InitLogs()
	if err := logsapi.ValidateAndApply(configuration, features); err != nil {
		return fmt.Errorf("Error in logging configuration: %s", err)
	}

	// Thanks to logs.InitLogs, slog.Default now uses klog as its backend. Thus,
	// the client-go library, which relies on klog.Info, has the same logger as
	// the agent, which still uses log.Printf.
	slog := slog.Default()

	// Let's make sure the VCert library, which is the only library we import to
	// be using the global log.Default, also uses the common slog logger.
	vcertLog := log.Default()
	vcertLog.SetOutput(LogToSlogWriter{Slog: slog, Source: "vcert"})

	// The venafi-connection-lib client uses various controller-runtime packages
	// which emit log messages. Make sure those log messages are not discarded.
	ctrlruntimelog.SetLogger(klog.Background().WithValues("source", "controller-runtime"))

	return nil
}

type LogToSlogWriter struct {
	Slog   *slog.Logger
	Source string
}

func (w LogToSlogWriter) Write(p []byte) (n int, err error) {
	// log.Printf writes a newline at the end of the message, so we need to trim
	// it.
	p = bytes.TrimSuffix(p, []byte("\n"))

	message := string(p)
	if strings.Contains(message, "error") ||
		strings.Contains(message, "failed") {
		w.Slog.With("source", w.Source).Error(message)
	} else {
		w.Slog.With("source", w.Source).Info(message)
	}
	return len(p), nil
}
