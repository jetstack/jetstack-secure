package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/spf13/cobra"
)

// ConfigFilePath is where the agent will try to load the configuration from
var ConfigFilePath string

// AuthToken is the authorization token that will be used for API calls
var AuthToken string

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

	// AuthToken flag takes preference over token in configuration file.
	if AuthToken == "" {
		AuthToken = config.Token
	} else {
		log.Printf("Using authorization token from flag.")
	}

	if config.Token != "" {
		config.Token = "(redacted)"
	}

	if AuthToken == "" {
		log.Fatalf("Missing authorization token. Cannot continue.")
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
		err = postData(serverURL, AuthToken, readings)
		// TODO: handle errors gracefully: e.g. handle retries when it is possible
		if err != nil {
			log.Fatalf("Post to server failed: %+v", err)
		}
		time.Sleep(10 * time.Second)
	}
}

func postData(serverURL *url.URL, token string, readings []*api.DataReading) error {
	data, err := json.Marshal(readings)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, serverURL.String(), bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	if len(token) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if code := resp.StatusCode; code < 200 || code >= 300 {
		return fmt.Errorf("Received response with status code %d. Body: %s", code, string(body))
	}

	log.Println("Data sent successfully. Server says: ", string(body))

	return nil
}
