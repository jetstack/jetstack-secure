package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/datagatherer"
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
func Run(cmd *cobra.Command, args []string) {
	logs.Log.Printf("Preflight agent version: %s (%s)", version.PreflightVersion, version.Commit)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	file, err := os.Open(Flags.ConfigFilePath)
	if err != nil {
		logs.Log.Fatalf("Failed to load config file for agent from: %s", Flags.ConfigFilePath)
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	if err != nil {
		logs.Log.Fatalf("Failed to read config file: %s", err)
	}

	cfg, err := ParseConfig(b, Flags.StrictMode)
	if err != nil {
		logs.Log.Fatalf("Failed to parse config file: %s", err)
	}

	config, preflightClient, err := getConfiguration(logs.Log, cfg, Flags)
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
	if Flags.Prometheus {
		logs.Log.Printf("Prometheus was enabled.\nRunning prometheus server on port :8081")
		go func() {
			prometheus.MustRegister(metricPayloadSize)
			metricsServer := http.NewServeMux()
			metricsServer.Handle("/metrics", promhttp.Handler())
			err := http.ListenAndServe(":8081", metricsServer)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				logs.Log.Fatalf("failed to run prometheus server: %s", err)
			}
		}()
	}

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
	var wg sync.WaitGroup

	// load datagatherer config and boot each one
	for _, dgConfig := range config.DataGatherers {
		kind := dgConfig.Kind
		if dgConfig.DataPath != "" {
			kind = "local"
			logs.Log.Fatalf("running data gatherer %s of type %s as Local, data-path override present: %s", dgConfig.Name, dgConfig.Kind, dgConfig.DataPath)
		}

		newDg, err := dgConfig.Config.NewDataGatherer(ctx)
		if err != nil {
			logs.Log.Fatalf("failed to instantiate %q data gatherer  %q: %v", kind, dgConfig.Name, err)
		}

		logs.Log.Printf("starting %q datagatherer", dgConfig.Name)

		// start the data gatherers and wait for the cache sync
		if err := newDg.Run(ctx.Done()); err != nil {
			logs.Log.Printf("failed to start %q data gatherer %q: %v", kind, dgConfig.Name, err)
		}

		// bootCtx is a context with a timeout to allow the informer 5
		// seconds to perform an initial sync. It may fail, and that's fine
		// too, it will backoff and retry of its own accord. Initial boot
		// will only be delayed by a max of 5 seconds.
		bootCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// wait for the informer to complete an initial sync, we do this to
		// attempt to have an initial set of data for the first upload of
		// the run.
		if err := newDg.WaitForCacheSync(bootCtx.Done()); err != nil {
			// log sync failure, this might recover in future
			logs.Log.Printf("failed to complete initial sync of %q data gatherer %q: %v", kind, dgConfig.Name, err)
		}

		// regardless of success, this dataGatherers has been given a
		// chance to sync its cache and we will now continue as normal. We
		// assume at the informers will either recover or the log messages
		// above will help operators correct the issue.
		dataGatherers[dgConfig.Name] = newDg
	}

	// wait for initial sync period to complete. if unsuccessful, then crash
	// and restart.
	c := make(chan struct{})
	go func() {
		defer close(c)
		logs.Log.Printf("waiting for datagatherers to complete inital syncs")
		wg.Wait()
	}()
	select {
	case <-c:
		logs.Log.Printf("datagatherers inital sync completed")
	case <-time.After(60 * time.Second):
		logs.Log.Fatalf("datagatherers inital sync failed due to timeout of 60 seconds")
	}

	// begin the datagathering loop, periodically sending data to the
	// configured output using data in datagatherer caches or refreshing from
	// APIs each cycle depending on datagatherer implementation
	for {
		// if period is set in the config, then use that if not already set
		if Flags.Period == 0 && config.Period > 0 {
			logs.Log.Printf("Using period from config %s", config.Period)
			Flags.Period = config.Period
		}

		gatherAndOutputData(config, preflightClient, dataGatherers)

		if Flags.OneShot {
			break
		}

		time.Sleep(Flags.Period)
	}
}

func gatherAndOutputData(config Config, preflightClient client.Client, dataGatherers map[string]datagatherer.DataGatherer) {
	var readings []*api.DataReading

	// Input/OutputPath flag overwrites agent.yaml configuration
	if Flags.InputPath == "" {
		Flags.InputPath = config.InputPath
	}
	if Flags.OutputPath == "" {
		Flags.OutputPath = config.OutputPath
	}

	if Flags.InputPath != "" {
		logs.Log.Printf("Reading data from local file: %s", Flags.InputPath)
		data, err := ioutil.ReadFile(Flags.InputPath)
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

	if Flags.OutputPath != "" {
		data, err := json.MarshalIndent(readings, "", "  ")
		if err != nil {
			logs.Log.Fatal("failed to marshal JSON")
		}
		err = ioutil.WriteFile(Flags.OutputPath, data, 0644)
		if err != nil {
			logs.Log.Fatalf("failed to output to local file: %s", err)
		}
		logs.Log.Printf("Data saved to local file: %s", Flags.OutputPath)
	} else {
		backOff := backoff.NewExponentialBackOff()
		backOff.InitialInterval = 30 * time.Second
		backOff.MaxInterval = 3 * time.Minute
		backOff.MaxElapsedTime = Flags.BackoffMaxTime
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

func gatherData(config Config, dataGatherers map[string]datagatherer.DataGatherer) []*api.DataReading {
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

	if Flags.StrictMode && dgError.ErrorOrNil() != nil {
		logs.Log.Fatalf("halting datagathering in strict mode due to error: %s", dgError.ErrorOrNil())
	}

	return readings
}

func postData(config Config, preflightClient client.Client, readings []*api.DataReading) error {
	baseURL := config.Server

	logs.Log.Println("Posting data to:", baseURL)

	if Flags.VenafiCloudMode {
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
		path := config.Endpoint.Path
		if path == "" {
			path = "/api/v1/datareadings"
		}
		res, err := preflightClient.Post(path, bytes.NewBuffer(data))

		if err != nil {
			return fmt.Errorf("failed to post data: %+v", err)
		}
		if code := res.StatusCode; code < 200 || code >= 300 {
			errorContent := ""
			body, _ := ioutil.ReadAll(res.Body)
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
