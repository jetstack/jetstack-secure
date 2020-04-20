package agent

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/spf13/cobra"
)

// ConfigFilePath is where the agent will try to load the configuration from
var ConfigFilePath string

// AuthToken is the authorization token that will be used for API calls
var AuthToken string

// Period is the number of seconds between scans
var Period uint

// CredentialsPath is where the agent will try to loads the credentials. (Experimental)
var CredentialsPath string

// Run starts the agent process
func Run(cmd *cobra.Command, args []string) {
	ctx := context.Background()

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

	if config.Token != "" {
		config.Token = "(redacted)"
	}

	serverURL, err := url.Parse(fmt.Sprintf("%s://%s%s", config.Endpoint.Protocol, config.Endpoint.Host, config.Endpoint.Path))
	if err != nil {
		log.Fatalf("Failed to build URL: %s", err)
	}

	dump, err := config.Dump()
	if err != nil {
		log.Fatalf("Failed to dump config: %s", err)
	}

	log.Printf("Loaded config: \n%s", dump)

	var credentials *Credentials
	if CredentialsPath != "" {
		file, err = os.Open(CredentialsPath)
		if err != nil {
			log.Fatalf("Failed to load credentials from file %s", CredentialsPath)
		}
		defer file.Close()

		b, err = ioutil.ReadAll(file)

		credentials, err = ParseCredentials(b)
		if err != nil {
			log.Fatalf("Failed to parse credentials file: %s", err)
		}
	}

	var preflightClient *client.PreflightClient
	if credentials != nil {
		log.Printf("A credentials file was specified. Using OAuth2 authentication...")
		preflightClient, err = client.New(credentials.UserKey, credentials.UserKeySecret, credentials.Server, serverURL.String())
		if err != nil {
			log.Fatalf("Error creating preflight client: %+v", err)
		}
	} else {
		// AuthToken flag takes preference over token in configuration file.
		if AuthToken == "" {
			AuthToken = config.Token
		} else {
			log.Printf("Using authorization token from flag.")
		}

		if AuthToken == "" {
			log.Fatalf("Missing authorization token. Cannot continue.")
		}

		preflightClient, err = client.NewWithBasicAuth(AuthToken, serverURL.String())
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
	for k, dg := range dataGatherers {
		i, err := dg.Fetch()
		if err != nil {
			log.Fatalf("Error fetching with DataGatherer %q: %s", k, err)
		}

		log.Printf("Gathered data for %q:\n", k)

		readings = append(readings, &api.DataReading{
			DataGatherer: k,
			Timestamp:    api.Time{Time: now},
			Data:         i,
		})
	}

	for {
		log.Println("Running Agent...")
		log.Println("Posting data to ", serverURL)
		err = preflightClient.PostDataReadings(readings)
		// TODO: handle errors gracefully: e.g. handle retries when it is possible
		if err != nil {
			log.Fatalf("Post to server failed: %+v", err)
		} else {
			log.Println("Data sent successfully.")
		}
		time.Sleep(time.Duration(Period) * time.Second)
	}
}
