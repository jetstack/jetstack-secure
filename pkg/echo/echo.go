package echo

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/jetstack/preflight/api"
)

var EchoListen string

var Compact bool

func Echo(cmd *cobra.Command, args []string) error {
	http.HandleFunc("/", echoHandler)
	fmt.Println("Listening to requests at ", EchoListen)
	return http.ListenAndServe(EchoListen, nil)
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, fmt.Sprintf("invalid method. Expected POST, received %s", r.Method), http.StatusBadRequest)
		return
	}

	// decode all data, however only datareadings are printed below
	var payload api.DataReadingsPost
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		writeError(w, fmt.Sprintf("decoding body: %+v", err), http.StatusBadRequest)
		return
	}

	// print the data sent to the echo server to the console

	if Compact {
		fmt.Printf("-- %s %s -> created %d\n", r.Method, r.URL.Path, http.StatusCreated)
		fmt.Printf("received %d readings:\n", len(payload.DataReadings))
		for _, r := range payload.DataReadings {
			fmt.Printf("%+v\n", r)
		}
	} else {
		color.Green("-- %s %s -> created %d\n", r.Method, r.URL.Path, http.StatusCreated)
		fmt.Printf("received %d readings:\n", len(payload.DataReadings))

		for i, r := range payload.DataReadings {
			c := color.New(color.FgYellow)
			if i%2 == 0 {
				c = color.New(color.FgCyan)
			}

			c.Printf("%v:\n%s\n", i, prettyPrint(r))
		}

		color.Green("-----")
	}

	// return successful response to the agent
	fmt.Fprintf(w, `{ "status": "ok" }`)
	w.Header().Set("Content-Type", "application/json")
}

func writeError(w http.ResponseWriter, err string, code int) {
	fmt.Printf("-- error %d -> %s\n", code, err)
	w.Header().Set("Content-Type", "application/json")
	http.Error(w, fmt.Sprintf(`{ "error": "%s", "code": %d }`, err, code), code)
}

func prettyPrint(reading *api.DataReading) string {
	return fmt.Sprintf(`ClusterID: %s
Data gatherer: %s
Timestamp: %s
SchemaVersion: %s
Data: %+v`,
		reading.ClusterID, reading.DataGatherer, reading.Timestamp, reading.SchemaVersion, reading.Data)
}
