package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/logs"
	"github.com/jetstack/preflight/pkg/version"

	"net/http/pprof"
	_ "net/http/pprof"
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
func Run(cmd *cobra.Command, args []string) (runErr error) {
	log := Flags.LogOptions.Initialize()

	ctx, cancel := context.WithCancel(klog.NewContext(context.Background(), log))
	defer cancel()

	log.Info("starting", "version", version.PreflightVersion, "commit", version.Commit)

	file, err := os.Open(Flags.ConfigFilePath)
	if err != nil {
		return fmt.Errorf("Failed to load config file for agent from %q: %s", Flags.ConfigFilePath, err)
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
		return fmt.Errorf("Failed to evaluate configuration: %s", err)
	}

	serverMux := http.NewServeMux()

	if Flags.Profiling {
		log.Info("pprof profiling enabled")
		serverMux.HandleFunc("/debug/pprof/", pprof.Index)
		serverMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		serverMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		serverMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		serverMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	}

	if Flags.Prometheus {
		log.Info("Prometheus metrics enabled")
		prometheus.MustRegister(metricPayloadSize)
		serverMux.Handle("/metrics", promhttp.Handler())
	}

	// Health check endpoint. Since we haven't figured a good way of knowning
	// what "ready" means for the agent, we just return 200 OK inconditionally.
	// The goal is to satisfy some Kubernetes distributions, like OpenShift,
	// that require a liveness and health probe to be present for each pod.
	serverMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	serverMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	serverPort := ":8081"
	server := http.Server{Addr: serverPort, Handler: serverMux}

	group, gCTX := errgroup.WithContext(ctx)
	defer func() {
		cancel()
		err = utilerrors.NewAggregate([]error{
			runErr,
			group.Wait(),
		})
	}()

	group.Go(func() error {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("failed to HTTP server: %s", err)
		}
		return nil
	})
	group.Go(func() error {
		<-gCTX.Done()
		ctx, cancel := context.WithTimeout(context.WithoutCancel(gCTX), time.Second*3)
		defer cancel()
		return server.Shutdown(ctx)
	})

	_, isVenConn := preflightClient.(*client.VenConnClient)
	if isVenConn {
		group.Go(func() error {
			return preflightClient.(manager.Runnable).Start(gCTX)
		})
	}

	dataGatherers := map[string]datagatherer.DataGatherer{}

	// load datagatherer config and boot each one
	for _, dgConfig := range config.DataGatherers {
		kind := dgConfig.Kind
		if dgConfig.DataPath != "" {
			kind = "local"
			logs.Log.Fatalf("running data gatherer %s of type %s as Local, data-path override present: %s", dgConfig.Name, dgConfig.Kind, dgConfig.DataPath)
		}

		newDg, err := dgConfig.Config.NewDataGatherer(gCTX)
		if err != nil {
			return fmt.Errorf("failed to instantiate %q data gatherer  %q: %v", kind, dgConfig.Name, err)
		}

		log.Info("starting datagatherer", "name", dgConfig.Name)

		// start the data gatherers and wait for the cache sync
		group.Go(func() error {
			if err := newDg.Run(gCTX.Done()); err != nil {
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
	bootCtx, bootCancel := context.WithTimeout(gCTX, 5*time.Second)
	defer bootCancel()
	for _, dgConfig := range config.DataGatherers {
		dg := dataGatherers[dgConfig.Name]
		// wait for the informer to complete an initial sync, we do this to
		// attempt to have an initial set of data for the first upload of
		// the run.
		if err := dg.WaitForCacheSync(bootCtx.Done()); err != nil {
			// log sync failure, this might recover in future
			log.Error(err, "failed to complete initial sync of data gatherer", "kind", dgConfig.Kind, "name", dgConfig.Name)
		}
	}

	// begin the datagathering loop, periodically sending data to the
	// configured output using data in datagatherer caches or refreshing from
	// APIs each cycle depending on datagatherer implementation
	for {
		if err := gatherAndOutputData(gCTX, config, preflightClient, dataGatherers); err != nil {
			return err
		}

		if config.OneShot {
			break
		}

		time.Sleep(config.Period)
	}
	return nil
}

func gatherAndOutputData(ctx context.Context, config CombinedConfig, preflightClient client.Client, dataGatherers map[string]datagatherer.DataGatherer) error {
	log := klog.FromContext(ctx)
	var readings []*api.DataReading

	if config.InputPath != "" {
		log.Info("Reading data from local file", "path", config.InputPath)
		data, err := os.ReadFile(config.InputPath)
		if err != nil {
			fmt.Errorf("failed to read local data file: %s", err)
		}
		err = json.Unmarshal(data, &readings)
		if err != nil {
			return fmt.Errorf("failed to unmarshal local data file: %s", err)
		}
	} else {

		if readings, err := gatherData(ctx, config, dataGatherers); err != nil {
			return err
		} else {
			readings = readings
		}
	}

	if config.OutputPath != "" {
		data, err := json.MarshalIndent(readings, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON")
		}
		err = os.WriteFile(config.OutputPath, data, 0644)
		if err != nil {
			return fmt.Errorf("failed to output to local file: %s", err)
		}
		log.Info("Data saved to local file", "path", config.OutputPath)
	} else {
		backOff := backoff.NewExponentialBackOff()
		backOff.InitialInterval = 30 * time.Second
		backOff.MaxInterval = 3 * time.Minute
		backOff.MaxElapsedTime = config.BackoffMaxTime
		post := func() error {
			return postData(ctx, config, preflightClient, readings)
		}
		err := backoff.RetryNotify(post, backOff, func(err error, t time.Duration) {
			log.Error(err, "retrying", "in", t)
		})
		if err != nil {
			return fmt.Errorf("Exiting due to fatal error uploading: %v", err)
		}
	}
	return nil
}

func gatherData(ctx context.Context, config CombinedConfig, dataGatherers map[string]datagatherer.DataGatherer) ([]*api.DataReading, error) {
	log := klog.FromContext(ctx).WithName("gather-data")
	var readings []*api.DataReading

	var dgError *multierror.Error
	for k, dg := range dataGatherers {
		dgData, count, err := dg.Fetch(ctx)
		if err != nil {
			dgError = multierror.Append(dgError, fmt.Errorf("error in datagatherer %s: %w", k, err))

			continue
		}

		log.Info("success", "count", count, "name", k)
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
	log := klog.FromContext(ctx)

	baseURL := config.Server

	log.Info("Posting data", "URL", baseURL)

	if config.AuthMode == VenafiCloudKeypair || config.AuthMode == VenafiCloudVenafiConnection {
		// orgID and clusterID are not required for Venafi Cloud auth
		err := preflightClient.PostDataReadingsWithOptions(readings, client.Options{
			ClusterName:        config.ClusterID,
			ClusterDescription: config.ClusterDescription,
		})
		if err != nil {
			return fmt.Errorf("post to server failed: %+v", err)
		}
		log.Info("Data sent successfully")

		return nil
	}

	if config.OrganizationID == "" {
		data, err := json.Marshal(readings)
		if err != nil {
			return fmt.Errorf("Cannot marshal readings: %+v", err)
		}

		// log and collect metrics about the upload size
		metric := metricPayloadSize.With(
			prometheus.Labels{"organization": config.OrganizationID, "cluster": config.ClusterID},
		)
		metric.Set(float64(len(data)))
		log.Info("Data readings upload size: %d", len(data))
		path := config.EndpointPath
		if path == "" {
			path = "/api/v1/datareadings"
		}
		res, err := preflightClient.Post(path, bytes.NewBuffer(data))

		if err != nil {
			return fmt.Errorf("failed to post data: %+v", err)
		}
		if code := res.StatusCode; code < 200 || code >= 300 {
			errorContent := ""
			body, _ := io.ReadAll(res.Body)
			if err == nil {
				errorContent = string(body)
			}
			defer res.Body.Close()

			return fmt.Errorf("received response with status code %d. Body: [%s]", code, errorContent)
		}
		log.Info("Data sent successfully")
		return err
	}

	if config.ClusterID == "" {
		return fmt.Errorf("post to server failed: missing clusterID from agent configuration")
	}

	err := preflightClient.PostDataReadings(config.OrganizationID, config.ClusterID, readings)
	if err != nil {
		return fmt.Errorf("post to server failed: %+v", err)
	}
	log.Info("Data sent successfully")

	return nil
}
