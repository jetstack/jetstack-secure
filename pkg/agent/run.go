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

// Run starts the agent process
func Run(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	for {
		config, preflightClient := getConfiguration(ctx)

		// if period is set in the config, then use that if not already set
		if Period == 0 && config.Period > 0 {
			log.Printf("Using period from config %s", config.Period)
			Period = config.Period
		}

		gatherAndOutputData(ctx, config, preflightClient)
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

	if Period == 0 && config.Period == 0 {
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

func gatherAndOutputData(ctx context.Context, config Config, preflightClient *client.PreflightClient) {
	var readings []*api.DataReading

	// Input/OutputPath flag overwrites agent.yaml configuration
	if InputPath == "" {
		InputPath = config.InputPath
	}
	if OutputPath == "" {
		OutputPath = config.OutputPath
	}

	if InputPath != "" {
		log.Println("Reading data from", InputPath)
		data, err := ioutil.ReadFile(InputPath)
		if err != nil {
			log.Fatal(err)
		}
		err = json.Unmarshal(data, &readings)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Data read successfully.")
	} else {
		readings = gatherData(ctx, config)
	}

	if OutputPath != "" {
		data, err := json.MarshalIndent(readings, "", "  ")
		err = ioutil.WriteFile(OutputPath, data, 0644)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Data saved locally to", OutputPath)
	} else {
		backOff := backoff.NewExponentialBackOff()
		backOff.InitialInterval = 30 * time.Second
		backOff.MaxInterval = 3 * time.Minute
		backOff.MaxElapsedTime = BackoffMaxTime
		post := func() error {
			return postData(config, preflightClient, readings)
		}
		err := backoff.RetryNotify(post, backOff, notify)
		if err != nil {
			log.Fatalf("%v", err)
		}

	}
}

func gatherData(ctx context.Context, config Config) []*api.DataReading {

	dataGatherers := make(map[string]datagatherer.DataGatherer)

	for _, dgConfig := range config.DataGatherers {
		kind := dgConfig.Kind
		if dgConfig.DataPath != "" {
			kind = "local"
			log.Printf("Running data gatherer %s of type %s as Local, data-path override present", dgConfig.Name, dgConfig.Kind)
		}

		dg, err := dgConfig.Config.NewDataGatherer(ctx)
		if err != nil {
			log.Fatalf("failed to instantiate %s DataGatherer: %v", kind, err)
		}

		dataGatherers[dgConfig.Name] = dg
	}

	//TODO Change backoff parameters to those desired
	backOff := backoff.NewExponentialBackOff()
	backOff.MaxElapsedTime = BackoffMaxTime
	readings := []*api.DataReading{}
	completedDataGatherers := make(map[string]bool, len(dataGatherers))

	// Fetch from all datagatherers
	getReadings := func() error {
		var dgError *multierror.Error
		for k, dg := range dataGatherers {
			if completedDataGatherers[k] {
				continue
			}
			dgData, err := dg.Fetch()
			if err != nil {
				if _, ok := err.(*dgerror.ConfigError); ok {
					if StrictMode {
						err = fmt.Errorf("%s: %v", k, err)
						dgError = multierror.Append(dgError, err)
					} else {
						log.Printf("%s: %v", k, err)
					}
				} else {
					err = fmt.Errorf("%s: %v", k, err)
					dgError = multierror.Append(dgError, err)
				}
				continue
			} else {
				completedDataGatherers[k] = true

				log.Printf("Successfully gathered data for %q", k)
				now := time.Now()

				readings = append(readings, &api.DataReading{
					ClusterID:    config.ClusterID,
					DataGatherer: k,
					Timestamp:    api.Time{Time: now},
					Data:         dgData,
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
		return dgError.ErrorOrNil()
	}

	if StrictMode {
		err := getReadings()
		if err != nil {
			log.Fatalf("%v", err)
		}
	} else {
		err := backoff.RetryNotify(func() error { return getReadings() }, backOff, notify)
		if err != nil {
			log.Println(err)
			log.Printf("This will not be retried")
		} else {
			log.Printf("Finished gathering data")
		}
	}

	return readings
}

func notify(err error, t time.Duration) {
	log.Println(err, "\nRetrying...")
}

func postData(config Config, preflightClient *client.PreflightClient, readings []*api.DataReading) error {
	baseURL := config.Server
	var err error

	log.Println("Running Agent...")
	log.Println("Posting data to ", baseURL)
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
	} else {
		err := preflightClient.PostDataReadings(config.OrganizationID, readings)
		if err != nil {
			return fmt.Errorf("Post to server failed: %+v", err)
		}
	}
	return err
}
