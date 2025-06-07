package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	clientgocorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"github.com/jetstack/preflight/pkg/kubeconfig"
	"github.com/jetstack/preflight/pkg/logs"
	"github.com/jetstack/preflight/pkg/version"
)

var Flags AgentCmdFlags

// schema version of the data sent by the agent.
// The new default version is v2.
// In v2 the agent posts data readings using api.gathereredResources
// Any requests without a schema version set will be interpreted
// as using v1 by the backend. In v1 the agent sends
// raw resource data of unstructuredList
const schemaVersion string = "v2.0.0"

// Run starts the agent process
func Run(cmd *cobra.Command, args []string) (returnErr error) {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	log := klog.FromContext(ctx).WithName("Run")

	log.Info("Starting", "version", version.PreflightVersion, "commit", version.Commit)

	file, err := os.Open(Flags.ConfigFilePath)
	if err != nil {
		return fmt.Errorf("Failed to load config file for agent from: %s", Flags.ConfigFilePath)
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("Failed to read config file: %s", err)
	}

	cfg, err := ParseConfig(b)
	if err != nil {
		return fmt.Errorf("Failed to parse config file: %s", err)
	}

	config, preflightClient, err := ValidateAndCombineConfig(log, cfg, Flags)
	if err != nil {
		return fmt.Errorf("While evaluating configuration: %v", err)
	}

	group, gctx := errgroup.WithContext(ctx)
	defer func() {
		cancel()
		if groupErr := group.Wait(); groupErr != nil {
			returnErr = multierror.Append(
				returnErr,
				fmt.Errorf("failed to wait for controller-runtime component to stop: %v", groupErr),
			)
		}
	}()

	{
		server := http.NewServeMux()
		const serverAddress = ":8081"
		log := log.WithName("APIServer").WithValues("addr", serverAddress)

		if Flags.Profiling {
			log.Info("Profiling endpoints enabled", "path", "/debug/pprof")
			server.HandleFunc("/debug/pprof/", pprof.Index)
			server.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			server.HandleFunc("/debug/pprof/profile", pprof.Profile)
			server.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			server.HandleFunc("/debug/pprof/trace", pprof.Trace)
		}

		if Flags.Prometheus {
			log.Info("Metrics endpoints enabled", "path", "/metrics")
			prometheus.MustRegister(metricPayloadSize)
			server.Handle("/metrics", promhttp.Handler())
		}

		// Health check endpoint. Since we haven't figured a good way of knowning
		// what "ready" means for the agent, we just return 200 OK unconditionally.
		// The goal is to satisfy some Kubernetes distributions, like OpenShift,
		// that require a liveness and health probe to be present for each pod.
		log.Info("Healthz endpoints enabled", "path", "/healthz")
		server.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		log.Info("Readyz endpoints enabled", "path", "/readyz")
		server.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		group.Go(func() error {
			err := listenAndServe(
				klog.NewContext(gctx, log),
				&http.Server{
					Addr:    serverAddress,
					Handler: server,
					BaseContext: func(_ net.Listener) context.Context {
						return gctx
					},
				},
			)
			if err != nil {
				return fmt.Errorf("APIServer: %s", err)
			}
			return nil
		})
	}

	_, isVenConn := preflightClient.(*client.VenConnClient)
	if isVenConn {
		group.Go(func() error {
			err := preflightClient.(manager.Runnable).Start(gctx)
			if err != nil {
				return fmt.Errorf("failed to start a controller-runtime component: %v", err)
			}

			// The agent must stop if the controller-runtime component stops.
			cancel()
			return nil
		})
	}

	// To help users notice issues with the agent, we show the error messages in
	// the agent pod's events.
	eventf, err := newEventf(log, config.InstallNS)
	if err != nil {
		return fmt.Errorf("failed to create event recorder: %v", err)
	}

	dataGatherers := map[string]datagatherer.DataGatherer{}

	// load datagatherer config and boot each one
	for _, dgConfig := range config.DataGatherers {
		kind := dgConfig.Kind
		if dgConfig.DataPath != "" {
			kind = "local"
			return fmt.Errorf("running data gatherer %s of type %s as Local, data-path override present: %s", dgConfig.Name, dgConfig.Kind, dgConfig.DataPath)
		}

		newDg, err := dgConfig.Config.NewDataGatherer(gctx)
		if err != nil {
			return fmt.Errorf("failed to instantiate %q data gatherer  %q: %v", kind, dgConfig.Name, err)
		}

		dynDg, isDynamicGatherer := newDg.(*k8s.DataGathererDynamic)
		if isDynamicGatherer {
			dynDg.ExcludeAnnotKeys = config.ExcludeAnnotationKeysRegex
			dynDg.ExcludeLabelKeys = config.ExcludeLabelKeysRegex
		}

		log.V(logs.Debug).Info("Starting DataGatherer", "name", dgConfig.Name)

		// start the data gatherers and wait for the cache sync
		group.Go(func() error {
			// Most implementations of `DataGatherer.Run` return immediately.
			// Only the Dynamic DataGatherer starts an informer which runs and
			// blocks until the supplied channel is closed.
			// For this reason, we must allow these errgroup Go routines to exit
			// without cancelling the other Go routines in the group.
			if err := newDg.Run(gctx); err != nil {
				return fmt.Errorf("failed to start %q data gatherer %q: %v", kind, dgConfig.Name, err)
			}
			return nil
		})

		// regardless of success, this dataGatherers has been given a
		// chance to sync its cache and we will now continue as normal. We
		// assume at the informers will either recover or the log messages
		// above will help operators correct the issue.
		dataGatherers[dgConfig.Name] = newDg
	}

	// Wait for 5 seconds for all informers to sync. If they fail to sync
	// we continue (as we have no way to know if they will recover or not).
	//
	// bootCtx is a context with a timeout to allow the informer 5
	// seconds to perform an initial sync. It may fail, and that's fine
	// too, it will backoff and retry of its own accord. Initial boot
	// will only be delayed by a max of 5 seconds.
	bootCtx, bootCancel := context.WithTimeout(gctx, 5*time.Second)
	defer bootCancel()

	var timedoutDGs []string
	for _, dgConfig := range config.DataGatherers {
		dg := dataGatherers[dgConfig.Name]
		// wait for the informer to complete an initial sync, we do this to
		// attempt to have an initial set of data for the first upload of
		// the run.
		if err := dg.WaitForCacheSync(bootCtx); err != nil {
			// log sync failure, this might recover in future
			if errors.Is(err, k8s.ErrCacheSyncTimeout) {
				timedoutDGs = append(timedoutDGs, dgConfig.Name)
			} else {
				log.V(logs.Info).Info("Failed to sync cache for datagatherer", "kind", dgConfig.Kind, "name", dgConfig.Name, "error", err)
			}
		}
	}
	if len(timedoutDGs) > 0 {
		log.V(logs.Info).Info("Skipping datagatherers for CRDs that can't be found in Kubernetes", "datagatherers", timedoutDGs)
	}
	// begin the datagathering loop, periodically sending data to the
	// configured output using data in datagatherer caches or refreshing from
	// APIs each cycle depending on datagatherer implementation.
	// If any of the go routines exit (with nil or error) the main context will
	// be cancelled, which will cause this blocking loop to exit
	// instead of waiting for the time period.
	for {
		if err := gatherAndOutputData(klog.NewContext(ctx, log), eventf, config, preflightClient, dataGatherers); err != nil {
			return err
		}

		if config.OneShot {
			break
		}

		select {
		case <-gctx.Done():
			return nil
		case <-time.After(config.Period):
		}
	}
	return nil
}

// Creates an event recorder for the agent's Pod object. Expects the env var
// POD_NAME to contain the pod name. Note that the RBAC rule allowing sending
// events is attached to the pod's service account, not the impersonated service
// account (venafi-connection).
func newEventf(log logr.Logger, installNS string) (Eventf, error) {
	restcfg, err := kubeconfig.LoadRESTConfig("")
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %v", err)
	}
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	var eventf Eventf
	if os.Getenv("POD_NAME") == "" {
		eventf = func(eventType, reason, msg string, args ...interface{}) {}
		log.Error(nil, "Error messages will not show in the pod's events because the POD_NAME environment variable is empty")
	} else {
		podName := os.Getenv("POD_NAME")

		eventClient, err := kubernetes.NewForConfig(restcfg)
		if err != nil {
			return eventf, fmt.Errorf("failed to create event client: %v", err)
		}
		broadcaster := record.NewBroadcaster()
		broadcaster.StartRecordingToSink(&clientgocorev1.EventSinkImpl{Interface: eventClient.CoreV1().Events(installNS)})
		eventRec := broadcaster.NewRecorder(scheme, corev1.EventSource{Component: "venafi-kubernetes-agent", Host: os.Getenv("POD_NODE")})
		eventf = func(eventType, reason, msg string, args ...interface{}) {
			eventRec.Eventf(&corev1.Pod{ObjectMeta: v1.ObjectMeta{Name: podName, Namespace: installNS, UID: types.UID(os.Getenv("POD_UID"))}}, eventType, reason, msg, args...)
		}
	}

	return eventf, nil
}

// Like Printf but for sending events to the agent's Pod object.
type Eventf func(eventType, reason, msg string, args ...interface{})

func gatherAndOutputData(ctx context.Context, eventf Eventf, config CombinedConfig, preflightClient client.Client, dataGatherers map[string]datagatherer.DataGatherer) error {
	log := klog.FromContext(ctx).WithName("gatherAndOutputData")
	var readings []*api.DataReading

	if config.InputPath != "" {
		log.V(logs.Debug).Info("Reading data from local file", "inputPath", config.InputPath)
		data, err := os.ReadFile(config.InputPath)
		if err != nil {
			return fmt.Errorf("failed to read local data file: %s", err)
		}
		err = json.Unmarshal(data, &readings)
		if err != nil {
			return fmt.Errorf("failed to unmarshal local data file: %s", err)
		}
	} else {
		var err error
		readings, err = gatherData(klog.NewContext(ctx, log), config, dataGatherers)
		if err != nil {
			return err
		}
	}

	if config.OutputPath != "" {
		data, err := json.MarshalIndent(readings, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %s", err)
		}
		err = os.WriteFile(config.OutputPath, data, 0644)
		if err != nil {
			return fmt.Errorf("failed to output to local file: %s", err)
		}
		log.Info("Data saved to local file", "outputPath", config.OutputPath)
	} else {
		group, ctx := errgroup.WithContext(ctx)

		backOff := backoff.NewExponentialBackOff()
		backOff.InitialInterval = 30 * time.Second
		backOff.MaxInterval = 3 * time.Minute

		notificationFunc := backoff.Notify(func(err error, t time.Duration) {
			eventf("Warning", "PushingErr", "retrying in %v after error: %s", t, err)
			log.Info("Warning: PushingErr: retrying", "in", t, "reason", err)
		})

		if config.MachineHubMode {
			post := func() (any, error) {
				log.Info("machine hub mode not yet implemented")
				return struct{}{}, nil
			}

			group.Go(func() error {
				_, err := backoff.Retry(ctx, post, backoff.WithBackOff(backOff), backoff.WithNotify(notificationFunc), backoff.WithMaxElapsedTime(config.BackoffMaxTime))
				return err
			})
		}

		if config.TLSPKMode != Off {
			post := func() (any, error) {
				return struct{}{}, postData(klog.NewContext(ctx, log), config, preflightClient, readings)
			}

			group.Go(func() error {
				_, err := backoff.Retry(ctx, post, backoff.WithBackOff(backOff), backoff.WithNotify(notificationFunc), backoff.WithMaxElapsedTime(config.BackoffMaxTime))
				return err
			})
		}

		groupErr := group.Wait()
		if groupErr != nil {
			return fmt.Errorf("got a fatal error from one or more upload actions: %s", groupErr)
		}
	}
	return nil
}

func gatherData(ctx context.Context, config CombinedConfig, dataGatherers map[string]datagatherer.DataGatherer) ([]*api.DataReading, error) {
	log := klog.FromContext(ctx).WithName("gatherData")

	var readings []*api.DataReading

	var dgError *multierror.Error
	for k, dg := range dataGatherers {
		dgData, count, err := dg.Fetch()
		if err != nil {
			dgError = multierror.Append(dgError, fmt.Errorf("error in datagatherer %s: %w", k, err))

			continue
		}
		{
			// Not all datagatherers return a count.
			// If `count == -1` it means that the datagatherer does not support returning a count.
			log := log
			if count >= 0 {
				log = log.WithValues("count", count)
			}
			log.V(logs.Debug).Info("Successfully gathered", "name", k)
		}
		readings = append(readings, &api.DataReading{
			ClusterID:     config.ClusterID,
			DataGatherer:  k,
			Timestamp:     api.Time{Time: time.Now()},
			Data:          dgData,
			SchemaVersion: schemaVersion,
		})
	}

	if dgError != nil {
		dgError.ErrorFormat = func(es []error) string {
			points := make([]string, len(es))
			for i, err := range es {
				points[i] = fmt.Sprintf("* %s", err)
			}
			return fmt.Sprintf(
				"The following %d data gatherer(s) have failed:\n\t%s",
				len(es), strings.Join(points, "\n\t"))
		}
	}

	if config.StrictMode && dgError.ErrorOrNil() != nil {
		return nil, fmt.Errorf("halting datagathering in strict mode due to error: %s", dgError.ErrorOrNil())
	}

	return readings, nil
}

func postData(ctx context.Context, config CombinedConfig, preflightClient client.Client, readings []*api.DataReading) error {
	log := klog.FromContext(ctx).WithName("postData")
	baseURL := config.Server

	log.V(logs.Debug).Info("Posting data", "baseURL", baseURL)

	switch config.TLSPKMode { // nolint:exhaustive
	case VenafiCloudKeypair, VenafiCloudVenafiConnection:
		// orgID and clusterID are not required for Venafi Cloud auth
		err := preflightClient.PostDataReadingsWithOptions(ctx, readings, client.Options{
			ClusterName:        config.ClusterID,
			ClusterDescription: config.ClusterDescription,
		})
		if err != nil {
			return fmt.Errorf("post to server failed: %+v", err)
		}
		log.Info("Data sent successfully")

		return nil

	case JetstackSecureOAuth, JetstackSecureAPIToken:
		err := preflightClient.PostDataReadingsWithOptions(ctx, readings, client.Options{
			OrgID:     config.OrganizationID,
			ClusterID: config.ClusterID,
		})
		if err != nil {
			return fmt.Errorf("post to server failed: %+v", err)
		}
		log.Info("Data sent successfully")

		return err

	default:
		return fmt.Errorf("not implemented for mode %s", config.TLSPKMode)
	}
}

// listenAndServe starts the supplied HTTP server and stops it gracefully when
// the supplied context is cancelled.
// It returns when the graceful server shutdown is complete or when the server
// exits with an error.
// If the server fails to start, it returns the server error.
// If the server fails to shutdown gracefully, it returns the shutdown error.
// The server is given 3 seconds to shutdown gracefully before it is stopped
// forcefully.
func listenAndServe(ctx context.Context, server *http.Server) error {
	log := klog.FromContext(ctx).WithName("ListenAndServe")

	log.V(logs.Debug).Info("Starting")

	listenCTX, listenCancelCause := context.WithCancelCause(context.WithoutCancel(ctx))
	go func() {
		err := server.ListenAndServe()
		listenCancelCause(fmt.Errorf("ListenAndServe: %s", err))
	}()

	select {
	case <-listenCTX.Done():
		log.V(logs.Debug).Info("Shutdown skipped", "reason", "Server already stopped")
		return context.Cause(listenCTX)

	case <-ctx.Done():
		log.V(logs.Debug).Info("Shutting down")
	}

	shutdownCTX, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second*3)
	shutdownErr := server.Shutdown(shutdownCTX)
	shutdownCancel()
	if shutdownErr != nil {
		shutdownErr = fmt.Errorf("Shutdown: %s", shutdownErr)
	}

	closeErr := server.Close()
	if closeErr != nil {
		closeErr = fmt.Errorf("Close: %s", closeErr)
	}

	log.V(logs.Debug).Info("Shutdown complete")

	return errors.Join(shutdownErr, closeErr)
}
