package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/hashicorp/go-multierror"
	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/datagatherer"
	dgerror "github.com/jetstack/preflight/pkg/datagatherer/error"
	"github.com/jetstack/preflight/pkg/version"
	"github.com/spf13/cobra"
)

// ConfigFilePath is where the agent will try to load the configuration from
var ConfigFilePath string

// Period is the time waited between scans
var Period time.Duration

// OneShot flag causes agent to run once
var OneShot bool

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

// schema version of the data sent by the agent.
// The new default version is v2.
// In v2 the agent posts data readings using api.gathereredResources
// Any requests without a schema version set will be interpreted
// as using v1 by the backend. In v1 the agent sends
// raw resource data of unstructuredList
const schemaVersion string = "v2.0.0"

// Run starts the agent process
func Run(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config, preflightClient := getConfiguration(ctx)

	dataGatherers := map[string]datagatherer.DataGatherer{}
	var wg sync.WaitGroup

	// load datagatherer config and boot each one
	for _, dgConfig := range config.DataGatherers {
		kind := dgConfig.Kind
		if dgConfig.DataPath != "" {
			kind = "local"
			log.Fatalf("running data gatherer %s of type %s as Local, data-path override present: %s", dgConfig.Name, dgConfig.Kind, dgConfig.DataPath)
		}

		newDg, err := dgConfig.Config.NewDataGatherer(ctx)
		if err != nil {
			log.Fatalf("failed to instantiate %q data gatherer  %q: %v", kind, dgConfig.Name, err)
		}

		wg.Add(1)

		go func() {
			log.Printf("starting %q datagatherer", dgConfig.Name)

			// start the data gatherers and wait for the cache sync
			if err := newDg.Run(ctx.Done()); err != nil {
				log.Printf("failed to start %q data gatherer %q: %v", kind, dgConfig.Name, err)
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
				log.Printf("failed to complete initial sync of %q data gatherer %q: %v", kind, dgConfig.Name, err)
			}

			// regardless of success, this dataGatherers has been given a
			// chance to sync its cache and we will now continue as normal. We
			// assume at the informers will either recover or the log messages
			// above will help operators correct the issue.
			wg.Done()
		}()

		dataGatherers[dgConfig.Name] = newDg
	}

	// wait for initial sync period to complete. if unsuccessful, then crash
	// and restart.
	c := make(chan struct{})
	go func() {
		defer close(c)
		log.Printf("waiting for datagatherers to complete inital syncs")
		wg.Wait()
	}()
	select {
	case <-c:
		log.Printf("datagatherers inital sync completed")
	case <-time.After(60 * time.Second):
		log.Fatalf("datagatherers inital sync failed due to timeout of 60 seconds")
	}

	// begin the datagathering loop, periodically sending data to the
	// configured output using data in datagatherer caches or refreshing from
	// APIs each cycle depending on datagatherer implementation
	for {
		// if period is set in the config, then use that if not already set
		if Period == 0 && config.Period > 0 {
			log.Printf("Using period from config %s", config.Period)
			Period = config.Period
		}

		gatherAndOutputData(ctx, config, preflightClient, dataGatherers)

		if OneShot {
			break
		}

		time.Sleep(Period)
	}
}

func getConfiguration(ctx context.Context) (Config, *client.PreflightClient) {
	log.Printf("Preflight agent version: %s (%s)", version.PreflightVersion, version.Commit)
	file, err := os.Open(ConfigFilePath)
	if err != nil {
		log.Fatalf("Failed to load config file for agent from: %s", ConfigFilePath)
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)

	config, err := ParseConfig(b)
	if err != nil {
		log.Fatalf("Failed to parse config file: %s", err)
	}

	baseURL := config.Server
	if baseURL == "" {
		log.Printf("Using deprecated Endpoint configuration. User Server instead.")
		baseURL = fmt.Sprintf("%s://%s", config.Endpoint.Protocol, config.Endpoint.Host)
		_, err = url.Parse(baseURL)
		if err != nil {
			log.Fatalf("Failed to build URL: %s", err)
		}
	}

	if Period == 0 && config.Period == 0 && !OneShot {
		log.Fatalf("Failed to load period, must be set as flag or in config")
	}

	dump, err := config.Dump()
	if err != nil {
		log.Fatalf("Failed to dump config: %s", err)
	}

	log.Printf("Loaded config: \n%s", dump)

	var credentials *client.Credentials
	if CredentialsPath != "" {
		file, err = os.Open(CredentialsPath)
		if err != nil {
			log.Fatalf("Failed to load credentials from file %s", CredentialsPath)
		}
		defer file.Close()

		b, err = ioutil.ReadAll(file)

		credentials, err = client.ParseCredentials(b)
		if err != nil {
			log.Fatalf("Failed to parse credentials file: %s", err)
		}
	}

	agentMetadata := &api.AgentMetadata{
		Version:   version.PreflightVersion,
		ClusterID: config.ClusterID,
	}
	var preflightClient *client.PreflightClient
	if credentials != nil {
		log.Printf("A credentials file was specified. Using OAuth2 authentication...")
		preflightClient, err = client.New(agentMetadata, credentials, baseURL)
		if err != nil {
			log.Fatalf("Error creating preflight client: %+v", err)
		}
	} else {
		log.Printf("No credentials file was specified. Starting client with no authentication...")
		preflightClient, err = client.NewWithNoAuth(agentMetadata, baseURL)
		if err != nil {
			log.Fatalf("Error creating preflight client: %+v", err)
		}
	}

	return config, preflightClient
}

func gatherAndOutputData(ctx context.Context, config Config, preflightClient *client.PreflightClient, dataGatherers map[string]datagatherer.DataGatherer) {
	var readings []*api.DataReading

	// Input/OutputPath flag overwrites agent.yaml configuration
	if InputPath == "" {
		InputPath = config.InputPath
	}
	if OutputPath == "" {
		OutputPath = config.OutputPath
	}

	if InputPath != "" {
		log.Printf("Reading data from local file: %s", InputPath)
		data, err := ioutil.ReadFile(InputPath)
		if err != nil {
			log.Fatalf("failed to read local data file: %s", err)
		}
		err = json.Unmarshal(data, &readings)
		if err != nil {
			log.Fatalf("failed to unmarshal local data file: %s", err)
		}
	} else {
		readings = gatherData(ctx, config, dataGatherers)
	}

	if OutputPath != "" {
		data, err := json.MarshalIndent(readings, "", "  ")
		err = ioutil.WriteFile(OutputPath, data, 0644)
		if err != nil {
			log.Fatalf("failed to output to local file: %s", err)
		}
		log.Printf("Data saved to local file: %s", OutputPath)
	} else {
		backOff := backoff.NewExponentialBackOff()
		backOff.InitialInterval = 30 * time.Second
		backOff.MaxInterval = 3 * time.Minute
		backOff.MaxElapsedTime = BackoffMaxTime
		post := func() error {
			return postData(config, preflightClient, readings)
		}
		err := backoff.RetryNotify(post, backOff, func(err error, t time.Duration) {
			log.Printf("retrying in %v after error: %s", t, err)
		})
		if err != nil {
			log.Fatalf("%v", err)
		}

	}
}

func startAndSyncDataGather(ctx context.Context, dg datagatherer.DataGatherer) error {
	if err := dg.Run(nil); err != nil {
		return err
	}
	return dg.WaitForCacheSync(nil)
}

func gatherData(ctx context.Context, config Config, dataGatherers map[string]datagatherer.DataGatherer) []*api.DataReading {
	readings := []*api.DataReading{}

	var dgError *multierror.Error
	for k, dg := range dataGatherers {
		dgData, err := dg.Fetch()
		if err != nil {
			if _, ok := err.(*dgerror.ConfigError); ok {
				if StrictMode {
					dgError = multierror.Append(dgError, fmt.Errorf("%s: %v", k, err))
				} else {
					log.Printf("config error in %q datagatherer: %v", k, err)
				}
			} else {
				dgError = multierror.Append(dgError, fmt.Errorf("error in datagatherer %q: %v", k, err))
			}
			continue
		} else {
			log.Printf("successfully gathered data from %q datagatherer", k)

			readings = append(readings, &api.DataReading{
				ClusterID:     config.ClusterID,
				DataGatherer:  k,
				Timestamp:     api.Time{Time: time.Now()},
				Data:          dgData,
				SchemaVersion: schemaVersion,
			})
		}
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
		log.Fatalf("halting datagathering in strict mode due to error: %s", dgError.ErrorOrNil())
	}

	return readings
}

func postData(config Config, preflightClient *client.PreflightClient, readings []*api.DataReading) error {
	baseURL := config.Server
	var err error

	log.Println("Running Agent...")
	log.Println("Posting data to:", baseURL)
	if config.OrganizationID == "" {
		data, err := json.Marshal(readings)
		if err != nil {
			log.Fatalf("Cannot marshal readings: %+v", err)
		}
		path := config.Endpoint.Path
		if path == "" {
			path = "/api/v1/datareadings"
		}
		res, err := preflightClient.Post(path, bytes.NewBuffer(data))

		if err != nil {
			return fmt.Errorf("Failed to post data: %+v", err)
		}
		if code := res.StatusCode; code < 200 || code >= 300 {
			errorContent := ""
			body, _ := ioutil.ReadAll(res.Body)
			if err == nil {
				errorContent = string(body)
			}
			defer res.Body.Close()

			return fmt.Errorf("Received response with status code %d. Body: %s", code, errorContent)
		}
		log.Println("Data sent successfully.")
	} else {
		err := preflightClient.PostDataReadings(config.OrganizationID, readings)
		if err != nil {
			return fmt.Errorf("Post to server failed: %+v", err)
		}
		log.Println("Data sent successfully.")
	}
	return err
}
