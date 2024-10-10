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
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	"github.com/jetstack/preflight/pkg/logs"
	"github.com/jetstack/preflight/pkg/version"

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
func Run(cmd *cobra.Command, args []string) {
	logs.Log.Printf("Preflight agent version: %s (%s)", version.PreflightVersion, version.Commit)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	file, err := os.Open(Flags.ConfigFilePath)
	if err != nil {
		logs.Log.Fatalf("Failed to load config file for agent from: %s", Flags.ConfigFilePath)
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		logs.Log.Fatalf("Failed to read config file: %s", err)
	}

	cfg, err := ParseConfig(b)
	if err != nil {
		logs.Log.Fatalf("Failed to parse config file: %s", err)
	}

	config, preflightClient, err := ValidateAndCombineConfig(logs.Log, cfg, Flags)
	if err != nil {
		logs.Log.Fatalf("While evaluating configuration: %v", err)
	}

	if Flags.Profiling {
		logs.Log.Printf("pprof profiling was enabled.\nRunning profiling on port :6060")
		go func() {
			err := http.ListenAndServe(":6060", nil)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				logs.Log.Fatalf("failed to run pprof profiler: %s", err)
			}
		}()
	}

	go func() {
		server := http.NewServeMux()

		if Flags.Prometheus {
			logs.Log.Printf("Prometheus was enabled.\nRunning prometheus on port :8081")
			prometheus.MustRegister(metricPayloadSize)
			server.Handle("/metrics", promhttp.Handler())
		}

		// Health check endpoint. Since we haven't figured a good way of knowning
		// what "ready" means for the agent, we just return 200 OK inconditionally.
		// The goal is to satisfy some Kubernetes distributions, like OpenShift,
		// that require a liveness and health probe to be present for each pod.
		server.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		server.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		err := http.ListenAndServe(":8081", server)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logs.Log.Fatalf("failed to run the health check server: %s", err)
		}
	}()

	_, isVenConn := preflightClient.(*client.VenConnClient)
	if isVenConn {
		go func() {
			err := preflightClient.(manager.Runnable).Start(ctx)
			if err != nil {
				logs.Log.Fatalf("failed to start a controller-runtime component: %v", err)
			}

			// The agent must stop if the controller-runtime component stops.
			cancel()
		}()
	}

	dataGatherers := map[string]datagatherer.DataGatherer{}
	group, gctx := errgroup.WithContext(ctx)

	defer func() {
		// TODO: replace Fatalf log calls with Errorf and return the error
		cancel()
		if err := group.Wait(); err != nil {
			logs.Log.Fatalf("failed to wait for controller-runtime component to stop: %v", err)
		}
	}()

	// Data gatherers are loaded depending on what the Kubernetes API supports.
	// First, let's do a /api discovery to see what the API supports.
	discoveryClient, err := k8s.NewDiscoveryClient("")
	if err != nil {
		logs.Log.Fatalf("failed to create a discovery client: %v", err)
	}

	apigroups, resources, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		logs.Log.Fatalf("failed to get server groups and resources: %v", err)
	}

	// Loop that creates and removes resources dynamically. Since there is no
	// way to watch the /api endpoint, 

	logs.Log.Printf("API groups: %v", apigroups)
	logs.Log.Printf("API resources: %v", resources)

	// load datagatherer config and boot each one
	for _, dgConfig := range config.DataGatherers {
		kind := dgConfig.Kind
		if dgConfig.DataPath != "" {
			kind = "local"
			logs.Log.Fatalf("running data gatherer %s of type %s as Local, data-path override present: %s", dgConfig.Name, dgConfig.Kind, dgConfig.DataPath)
		}

		newDg, err := dgConfig.Config.NewDataGatherer(gctx)
		if err != nil {
			logs.Log.Fatalf("failed to instantiate %q data gatherer  %q: %v", kind, dgConfig.Name, err)
		}

		logs.Log.Printf("starting %q datagatherer", dgConfig.Name)

		// start the data gatherers and wait for the cache sync
		group.Go(func() error {
			if err := newDg.Run(gctx.Done()); err != nil {
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
	for _, dgConfig := range config.DataGatherers {
		dg := dataGatherers[dgConfig.Name]
		// wait for the informer to complete an initial sync, we do this to
		// attempt to have an initial set of data for the first upload of
		// the run.
		if err := dg.WaitForCacheSync(bootCtx.Done()); err != nil {
			// log sync failure, this might recover in future
			logs.Log.Printf("failed to complete initial sync of %q data gatherer %q: %v", dgConfig.Kind, dgConfig.Name, err)
		}
	}

	// begin the datagathering loop, periodically sending data to the
	// configured output using data in datagatherer caches or refreshing from
	// APIs each cycle depending on datagatherer implementation
	for {
		gatherAndOutputData(config, preflightClient, dataGatherers)

		if config.OneShot {
			break
		}

		time.Sleep(config.Period)
	}
}

func gatherAndOutputData(config CombinedConfig, preflightClient client.Client, dataGatherers map[string]datagatherer.DataGatherer) {
	var readings []*api.DataReading

	if config.InputPath != "" {
		logs.Log.Printf("Reading data from local file: %s", config.InputPath)
		data, err := os.ReadFile(config.InputPath)
		if err != nil {
			logs.Log.Fatalf("failed to read local data file: %s", err)
		}
		err = json.Unmarshal(data, &readings)
		if err != nil {
			logs.Log.Fatalf("failed to unmarshal local data file: %s", err)
		}
	} else {
		readings = gatherData(config, dataGatherers)
	}

	if config.OutputPath != "" {
		data, err := json.MarshalIndent(readings, "", "  ")
		if err != nil {
			logs.Log.Fatal("failed to marshal JSON")
		}
		err = os.WriteFile(config.OutputPath, data, 0644)
		if err != nil {
			logs.Log.Fatalf("failed to output to local file: %s", err)
		}
		logs.Log.Printf("Data saved to local file: %s", config.OutputPath)
	} else {
		backOff := backoff.NewExponentialBackOff()
		backOff.InitialInterval = 30 * time.Second
		backOff.MaxInterval = 3 * time.Minute
		backOff.MaxElapsedTime = config.BackoffMaxTime
		post := func() error {
			return postData(config, preflightClient, readings)
		}
		err := backoff.RetryNotify(post, backOff, func(err error, t time.Duration) {
			logs.Log.Printf("retrying in %v after error: %s", t, err)
		})
		if err != nil {
			logs.Log.Fatalf("Exiting due to fatal error uploading: %v", err)
		}
	}
}

func gatherData(config CombinedConfig, dataGatherers map[string]datagatherer.DataGatherer) []*api.DataReading {
	var readings []*api.DataReading

	var dgError *multierror.Error
	for k, dg := range dataGatherers {
		dgData, count, err := dg.Fetch()
		if err != nil {
			dgError = multierror.Append(dgError, fmt.Errorf("error in datagatherer %s: %w", k, err))

			continue
		}

		if count >= 0 {
			logs.Log.Printf("successfully gathered %d items from %q datagatherer", count, k)
		} else {
			logs.Log.Printf("successfully gathered data from %q datagatherer", k)
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
		logs.Log.Fatalf("halting datagathering in strict mode due to error: %s", dgError.ErrorOrNil())
	}

	return readings
}

func postData(config CombinedConfig, preflightClient client.Client, readings []*api.DataReading) error {
	baseURL := config.Server

	logs.Log.Println("Posting data to:", baseURL)

	if config.AuthMode == VenafiCloudKeypair || config.AuthMode == VenafiCloudVenafiConnection {
		// orgID and clusterID are not required for Venafi Cloud auth
		err := preflightClient.PostDataReadingsWithOptions(readings, client.Options{
			ClusterName:        config.ClusterID,
			ClusterDescription: config.ClusterDescription,
		})
		if err != nil {
			return fmt.Errorf("post to server failed: %+v", err)
		}
		logs.Log.Println("Data sent successfully.")

		return nil
	}

	if config.OrganizationID == "" {
		data, err := json.Marshal(readings)
		if err != nil {
			logs.Log.Fatalf("Cannot marshal readings: %+v", err)
		}

		// log and collect metrics about the upload size
		metric := metricPayloadSize.With(
			prometheus.Labels{"organization": config.OrganizationID, "cluster": config.ClusterID},
		)
		metric.Set(float64(len(data)))
		logs.Log.Printf("Data readings upload size: %d", len(data))
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
		logs.Log.Println("Data sent successfully.")
		return err
	}

	if config.ClusterID == "" {
		return fmt.Errorf("post to server failed: missing clusterID from agent configuration")
	}

	err := preflightClient.PostDataReadings(config.OrganizationID, config.ClusterID, readings)
	if err != nil {
		return fmt.Errorf("post to server failed: %+v", err)
	}
	logs.Log.Println("Data sent successfully.")

	return nil
}
