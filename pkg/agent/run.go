package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/hashicorp/go-multierror"
	"github.com/jetstack/preflight/pkg/logs"
	json "github.com/json-iterator/go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/version"
)

// ConfigFilePath is where the agent will try to load the configuration from
var ConfigFilePath string

// Period is the time waited between scans
var Period time.Duration

// OneShot flag causes agent to run once
var OneShot bool

// VenafiCloudMode flag determines which format to load for config and credential type
var VenafiCloudMode bool

// ClientID is the clientID in case of Venafi Cloud mode
var ClientID string

// PrivateKeyPath is the path for the service account private key in case of Venafi Cloud mode
var PrivateKeyPath string

// CredentialsPath is where the agent will try to loads the credentials. (Experimental)
var CredentialsPath string

// OutputPath is where the agent will write data to locally if specified
var OutputPath string

// InputPath is where the agent will read data from instead of gathering from clusters if specified
var InputPath string

// BackoffMaxTime is the maximum time for which data gatherers will be retried
var BackoffMaxTime time.Duration

// StrictMode flag causes the agent to fail at the first attempt
var StrictMode bool

// APIToken is an authentication token used for the backend API as an alternative to oauth flows.
var APIToken string

// VenConnName is the name of the VenafiConnection resource to use. Using this
// flag will enable Venafi Connection mode.
var VenConnName string

// VenConnNS is the namespace of the VenafiConnection resource to use. It is
// only useful when the VenafiConnection isn't in the same namespace as the
// agent.
//
// May be left empty to use the same namespace as the agent.
var VenConnNS string

// InstallNS is the namespace in which the agent is running in. Only needed when
// running the agent outside of Kubernetes.
//
// May be left empty when running in Kubernetes. In this case, the namespace is
// read from the file /var/run/secrets/kubernetes.io/serviceaccount/namespace.
var InstallNS string

// Profiling flag enabled pprof endpoints to run on the agent
var Profiling bool

// Prometheus flag enabled Prometheus metrics endpoint to run on the agent
var Prometheus bool

// schema version of the data sent by the agent.
// The new default version is v2.
// In v2 the agent posts data readings using api.gathereredResources
// Any requests without a schema version set will be interpreted
// as using v1 by the backend. In v1 the agent sends
// raw resource data of unstructuredList
const schemaVersion string = "v2.0.0"

const (
	inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

// Run starts the agent process
func Run(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config, preflightClient := getConfiguration()

	if Profiling {
		logs.Log.Printf("pprof profiling was enabled.\nRunning profiling on port :6060")
		go func() {
			err := http.ListenAndServe(":6060", nil)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				logs.Log.Fatalf("failed to run pprof profiler: %s", err)
			}
		}()
	}
	if Prometheus {
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
		if Period == 0 && config.Period > 0 {
			logs.Log.Printf("Using period from config %s", config.Period)
			Period = config.Period
		}

		gatherAndOutputData(config, preflightClient, dataGatherers)

		if OneShot {
			break
		}

		time.Sleep(Period)
	}
}

func getConfiguration() (Config, client.Client) {
	logs.Log.Printf("Preflight agent version: %s (%s)", version.PreflightVersion, version.Commit)
	file, err := os.Open(ConfigFilePath)
	if err != nil {
		logs.Log.Fatalf("Failed to load config file for agent from: %s", ConfigFilePath)
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	if err != nil {
		logs.Log.Fatalf("Failed to read config file: %s", err)
	}

	// If the ClientID of the service account is specified, then assume we are in Venafi Cloud mode.
	if ClientID != "" || VenConnName != "" {
		VenafiCloudMode = true
	}

	config, err := ParseConfig(b, VenafiCloudMode)
	if err != nil {
		logs.Log.Fatalf("Failed to parse config file: %s", err)
	}

	baseURL := config.Server
	if baseURL == "" {
		logs.Log.Printf("Using deprecated Endpoint configuration. User Server instead.")
		baseURL = fmt.Sprintf("%s://%s", config.Endpoint.Protocol, config.Endpoint.Host)
		_, err = url.Parse(baseURL)
		if err != nil {
			logs.Log.Fatalf("Failed to build URL: %s", err)
		}
	}

	if Period == 0 && config.Period == 0 && !OneShot {
		logs.Log.Fatalf("Failed to load period, must be set as flag or in config")
	}

	dump, err := config.Dump()
	if err != nil {
		logs.Log.Fatalf("Failed to dump config: %s", err)
	}

	logs.Log.Printf("Loaded config: \n%s", dump)

	var credentials client.Credentials
	if ClientID != "" {
		credentials = &client.VenafiSvcAccountCredentials{
			ClientID:       ClientID,
			PrivateKeyFile: PrivateKeyPath,
		}
	} else if CredentialsPath != "" {
		file, err = os.Open(CredentialsPath)
		if err != nil {
			logs.Log.Fatalf("Failed to load credentials from file %s", CredentialsPath)
		}
		defer file.Close()

		b, err = io.ReadAll(file)
		if err != nil {
			logs.Log.Fatalf("Failed to read credentials file: %v", err)
		}
		if VenafiCloudMode {
			credentials, err = client.ParseVenafiCredentials(b)
		} else {
			credentials, err = client.ParseOAuthCredentials(b)
		}
		if err != nil {
			logs.Log.Fatalf("Failed to parse credentials file: %s", err)
		}
	}

	venConnMode := VenConnName != ""

	if venConnMode && InstallNS == "" {
		InstallNS, err = getInClusterNamespace()
		if err != nil {
			log.Fatalf("could not guess which namespace the agent is running in: %s", err)
		}
	}
	if venConnMode && VenConnNS == "" {
		VenConnNS = InstallNS
	}

	agentMetadata := &api.AgentMetadata{
		Version:   version.PreflightVersion,
		ClusterID: config.ClusterID,
	}

	var preflightClient client.Client
	switch {
	case credentials != nil:
		preflightClient, err = createCredentialClient(credentials, config, agentMetadata, baseURL)
	case VenConnName != "":
		// Why wasn't this added to the createCredentialClient instead? Because
		// the --venafi-connection mode of authentication doesn't need any
		// secrets (or any other information for that matter) to be loaded from
		// disk (using --credentials-path). Everything is passed as flags.
		log.Println("Venafi Connection mode was specified, using Venafi Connection authentication.")

		// The venafi-cloud.upload_path was initially meant to let users
		// configure HTTP proxies, but it has never been used since HTTP proxies
		// don't rewrite paths. Thus, we've disabled the ability to change this
		// value with the new --venafi-connection flag, and this field is simply
		// ignored.
		if config.VenafiCloud != nil && config.VenafiCloud.UploadPath != "" {
			log.Printf(`ignoring venafi-cloud.upload_path. In Venafi Connection mode, this field is not needed.`)
		}

		// Regarding venafi-cloud.uploader_id, we found that it doesn't do
		// anything in the backend. Since the backend requires it for historical
		// reasons (but cannot be empty), we just ignore whatever the user has
		// set in the config file, and set it to an arbitrary value in the
		// client since it doesn't matter.
		if config.VenafiCloud.UploaderID != "" {
			log.Printf(`ignoring venafi-cloud.uploader_id. In Venafi Connection mode, this field is not needed.`)
		}

		cfg, err := loadRESTConfig("")
		if err != nil {
			log.Fatalf("failed to load kubeconfig: %v", err)
		}

		preflightClient, err = client.NewVenConnClient(cfg, agentMetadata, InstallNS, VenConnName, VenConnNS, nil)
	case APIToken != "":
		logs.Log.Println("An API token was specified, using API token authentication.")
		preflightClient, err = client.NewAPITokenClient(agentMetadata, APIToken, baseURL)
	default:
		logs.Log.Println("No credentials were specified, using with no authentication.")
		preflightClient, err = client.NewUnauthenticatedClient(agentMetadata, baseURL)
	}

	if err != nil {
		logs.Log.Fatalf("failed to create client: %v", err)
	}

	return config, preflightClient
}

func createCredentialClient(credentials client.Credentials, config Config, agentMetadata *api.AgentMetadata, baseURL string) (client.Client, error) {
	switch creds := credentials.(type) {
	case *client.VenafiSvcAccountCredentials:
		logs.Log.Println("Venafi Cloud mode was specified, using Venafi Service Account authentication.")
		// check if config has Venafi Cloud data, use config data if it's present
		uploaderID := creds.ClientID
		uploadPath := ""
		if config.VenafiCloud != nil {
			logs.Log.Println("Loading uploader_id and upload_path from \"venafi-cloud\" configuration.")
			uploaderID = config.VenafiCloud.UploaderID
			uploadPath = config.VenafiCloud.UploadPath
		}
		return client.NewVenafiCloudClient(agentMetadata, creds, baseURL, uploaderID, uploadPath)

	case *client.OAuthCredentials:
		logs.Log.Println("A credentials file was specified, using oauth authentication.")
		return client.NewOAuthClient(agentMetadata, creds, baseURL)
	default:
		return nil, errors.New("credentials file is in unknown format")
	}
}

func gatherAndOutputData(config Config, preflightClient client.Client, dataGatherers map[string]datagatherer.DataGatherer) {
	var readings []*api.DataReading

	// Input/OutputPath flag overwrites agent.yaml configuration
	if InputPath == "" {
		InputPath = config.InputPath
	}
	if OutputPath == "" {
		OutputPath = config.OutputPath
	}

	if InputPath != "" {
		logs.Log.Printf("Reading data from local file: %s", InputPath)
		data, err := ioutil.ReadFile(InputPath)
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

	if OutputPath != "" {
		data, err := json.MarshalIndent(readings, "", "  ")
		if err != nil {
			logs.Log.Fatal("failed to marshal JSON")
		}
		err = ioutil.WriteFile(OutputPath, data, 0644)
		if err != nil {
			logs.Log.Fatalf("failed to output to local file: %s", err)
		}
		logs.Log.Printf("Data saved to local file: %s", OutputPath)
	} else {
		backOff := backoff.NewExponentialBackOff()
		backOff.InitialInterval = 30 * time.Second
		backOff.MaxInterval = 3 * time.Minute
		backOff.MaxElapsedTime = BackoffMaxTime
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

	if StrictMode && dgError.ErrorOrNil() != nil {
		logs.Log.Fatalf("halting datagathering in strict mode due to error: %s", dgError.ErrorOrNil())
	}

	return readings
}

func postData(config Config, preflightClient client.Client, readings []*api.DataReading) error {
	baseURL := config.Server

	logs.Log.Println("Posting data to:", baseURL)

	if VenafiCloudMode {
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

// Inspired by the controller-runtime project.
func getInClusterNamespace() (string, error) {
	// Check whether the namespace file exists.
	// If not, we are not running in cluster so can't guess the namespace.
	_, err := os.Stat(inClusterNamespacePath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("not running in cluster, please use --install-namespace to specify the namespace in which the agent is running")
	}
	if err != nil {
		return "", fmt.Errorf("error checking namespace file: %w", err)
	}

	namespace, err := os.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %w", err)
	}
	return string(namespace), nil
}

func loadRESTConfig(path string) (*rest.Config, error) {
	switch path {
	// If the kubeconfig path is not provided, use the default loading rules
	// so we read the regular KUBECONFIG variable or create a non-interactive
	// client for agents running in cluster
	case "":
		loadingrules := clientcmd.NewDefaultClientConfigLoadingRules()
		cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingrules, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
		return cfg, nil
	// Otherwise use the explicitly named kubeconfig file.
	default:
		cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: path},
			&clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig from %s: %w", path, err)
		}
		return cfg, nil
	}
}
