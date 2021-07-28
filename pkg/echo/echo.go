package echo

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/fatih/color"
	"github.com/jetstack/preflight/api"
	"github.com/spf13/cobra"
)

var EchoListen, AllowedToken string

func Echo(cmd *cobra.Command, args []string) {
	http.HandleFunc("/", echoHandler)
	fmt.Println("Listening to requests at ", EchoListen)
	err := http.ListenAndServe(EchoListen, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	code, err := checkAuthorization(w, r)
	if err != nil {
		writeError(w, err.Error(), code)
		return
	}

	if r.Method != http.MethodPost {
		writeError(w, fmt.Sprintf("invalid method. Expected POST, received %s", r.Method), http.StatusBadRequest)
		return
	}

	// decode all data, however only datareadings are printed below
	var payload api.DataReadingsPost
	err = json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		writeError(w, fmt.Sprintf("decoding body: %+v", err), http.StatusBadRequest)
		return
	}

	// print the data sent to the echo server to the console

	color.Green("-- %s %s -> created %d\n", r.Method, r.URL.Path, http.StatusCreated)
	fmt.Printf("received %d readings:\n", len(payload.DataReadings))
	for i, r := range payload.DataReadings {
		if i%2 == 0 {
			color.Yellow("Reading:\n%s\n", prettyPrint(r))
		} else {
			color.Cyan("Reading:\n%s\n", prettyPrint(r))
		}
	}
	color.Green("-----")

	// return successful response to the agent
	fmt.Fprintf(w, `{ "status": "ok" }`)
	w.Header().Set("Content-Type", "application/json")
}

func checkAuthorization(w http.ResponseWriter, r *http.Request) (int, error) {
	if AllowedToken != "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="Echo"`)

		s := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		if len(s) != 2 {
			return 400, fmt.Errorf("bad request: malformed Authorization header")
		}

		if s[0] != "Bearer" {
			return 401, fmt.Errorf("not authorized")
		}

		if s[1] != AllowedToken {
			return 401, fmt.Errorf("not authorized")
		}
	}

	return 0, nil
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
