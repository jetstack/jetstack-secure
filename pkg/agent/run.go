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

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/version"
	"github.com/spf13/cobra"
)

// ConfigFilePath is where the agent will try to load the configuration from
var ConfigFilePath string

// AuthToken is the authorization token that will be used for API calls
var AuthToken string

// Period is the number of seconds between scans
var Period uint

// Number of times the agent will gather and post data
var NumberPeriods int

// CredentialsPath is where the agent will try to loads the credentials. (Experimental)
var CredentialsPath string


// Run starts the agent process
func Run(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	for i := 0; i != NumberPeriods; {
		gatherAndPostData(ctx)
		
		if i == NumberPeriods - 1 {
			break
		}

		time.Sleep(time.Duration(Period) * time.Second)

		// Progress loop if a positive number of periods is given
		if NumberPeriods > 0 {
			i++
		}
	}
}

func gatherAndPostData(ctx context.Context) {
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

	// AuthToken flag takes preference over token in configuration file.
	if AuthToken == "" {
		AuthToken = config.Token
	} else {
		log.Printf("Using authorization token from flag.")
	}

	if config.Token != "" {
		config.Token = "(redacted)"
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
		Version: version.PreflightVersion,
	}
	var preflightClient *client.PreflightClient
	if credentials != nil {
		log.Printf("A credentials file was specified. Using OAuth2 authentication...")
		preflightClient, err = client.New(agentMetadata, credentials, baseURL)
		if err != nil {
			log.Fatalf("Error creating preflight client: %+v", err)
		}
	} else {
		if AuthToken == "" {
			log.Fatalf("Missing authorization token. Cannot continue.")
		}

		preflightClient, err = client.NewWithBasicAuth(agentMetadata, AuthToken, baseURL)
		if err != nil {
			log.Fatalf("Error creating preflight client: %+v", err)
		}
	}

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

	// Fetch from all datagatherers
	now := time.Now()
	readings := []*api.DataReading{}
	failedDataGatherers := []string{}
	for k, dg := range dataGatherers {
		i, err := dg.Fetch()
		if err != nil {
			log.Printf("Error fetching with DataGatherer %q: %s", k, err)
			failedDataGatherers = append(failedDataGatherers, k)
			continue
		}

		log.Printf("Gathered data for %q:\n", k)

		readings = append(readings, &api.DataReading{
			ClusterID:    config.ClusterID,
			DataGatherer: k,
			Timestamp:    api.Time{Time: now},
			Data:         i,
		})
	}

	if len(failedDataGatherers) > 0 {
		log.Printf(
			"Warning, the following DataGatherers failed, %s. Their data is not being sent.",
			strings.Join(failedDataGatherers, ", "),
		)
	}

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
			log.Fatalf("Failed to post data: %+v", err)
		}
		if code := res.StatusCode; code < 200 || code >= 300 {
			errorContent := ""
			body, _ := ioutil.ReadAll(res.Body)
			if err == nil {
				errorContent = string(body)
			}
			defer res.Body.Close()

			log.Fatalf("Received response with status code %d. Body: %s", code, errorContent)
		}
	} else {
		err = preflightClient.PostDataReadings(config.OrganizationID, readings)
		// TODO: handle errors gracefully: e.g. handle retries when it is possible
		if err != nil {
			log.Fatalf("Post to server failed: %+v", err)
		}
	}

	log.Println("Data sent successfully.")
}
