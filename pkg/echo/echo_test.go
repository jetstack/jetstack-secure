package echo

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jetstack/preflight/api"
)

func TestEchoServerRequestResponse(t *testing.T) {
	// create sample data in same format that would be generated by the agent
	sampleUpload := api.DataReadingsPost{
		AgentMetadata: &api.AgentMetadata{
			Version:   "test suite",
			ClusterID: "test_suite_cluster",
		},
		DataGatherTime: time.Now(),
		DataReadings: []*api.DataReading{
			&api.DataReading{
				ClusterID:    "test_suite_cluster",
				DataGatherer: "dummy",
				Timestamp:    api.Time{time.Now()},
				Data: map[string]string{
					"test": "test",
				},
				SchemaVersion: "2.0.0",
			},
		},
	}

	// generate the JSON representation of the data to be sent to the echo server
	requestBodyJSON, err := json.Marshal(sampleUpload)
	if err != nil {
		t.Fatalf("failed to generate JSON request body to post: %s", err)
	}

	// generate a request to test the handler containing the JSON data as a body
	req, err := http.NewRequest("POST", "http://example.com/api/v1/datareadings", bytes.NewBuffer(requestBodyJSON))
	if err != nil {
		t.Fatalf("failed to generate request to test echo server: %s", err)
	}

	// create recorder to save the response
	rr := httptest.NewRecorder()

	// perform the request with the handler
	echoHandler(rr, req)

	// Check the response from the echo handler is 200 OK
	response := rr.Result()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("echo server responded with an unexpected code: %d", response.StatusCode)
	}
}
