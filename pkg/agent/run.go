package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/spf13/cobra"
)

// ConfigFilePath is where the agent will try to load the configuration from
var ConfigFilePath string

// Run starts the agent process
func Run(cmd *cobra.Command, args []string) {
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

	serverURL, err := url.Parse(fmt.Sprintf("%s://%s%s", config.Endpoint.Protocol, config.Endpoint.Host, config.Endpoint.Path))
	if err != nil {
		log.Fatalf("Failed to build URL: %s", err)
	}

	dump, err := config.Dump()
	if err != nil {
		log.Fatalf("Failed to dump config: %s", err)
	}

	log.Printf("Loaded config: \n%s", dump)

	for {
		log.Println("Running Agent...")
		log.Println("Posting data to ", serverURL)
		err = postData(serverURL, []*api.DataReading{
			&api.DataReading{
				DataGatherer: "dummy",
				Timestamp:    api.Time{Time: time.Now()},
				Data: map[string]string{
					"field1": "data1",
					"field2": "data2",
					"field3": "data3",
				},
			},
		})
		// TODO: handle errors gracefully: e.g. handle retries when it is possible
		if err != nil {
			log.Fatalf("Post to server failed: %+v", err)
		}
		time.Sleep(10 * time.Second)
	}
}

func postData(serverURL *url.URL, readings []*api.DataReading) error {
	data, err := json.Marshal(readings)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("ASDF", serverURL.String(), bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	ss := serverURL.String()
	fmt.Println(ss)
	resp, err := http.Post(ss, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	// client := &http.Client{}
	// resp, err := client.Do(req)
	// if err != nil {
	// 	return err
	// }

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// resp.Body.Close()

	if code := resp.StatusCode; code < 200 || code >= 300 {
		return fmt.Errorf("Received response with status code %d. Body: %s", code, string(body))
	}

	log.Println("Data sent successfully. Server says: ", string(body))

	return nil
}
