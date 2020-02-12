package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/jetstack/preflight/api"
	"github.com/spf13/cobra"
)

var echoCmd = &cobra.Command{
	Use:   "echo",
	Short: "starts an echo server to test the agent",
	Long: `The agent sends data to a server. This echo server
can be used to act as the server part and echo the data received by the agent.`,
	Run: echo,
}

var echoListen string

func init() {
	rootCmd.AddCommand(echoCmd)
	echoCmd.PersistentFlags().StringVarP(
		&echoListen,
		"listen",
		"l",
		":8080",
		"Address where to listen.",
	)
}

func echo(cmd *cobra.Command, args []string) {
	http.HandleFunc("/", echoHandler)
	fmt.Println("Listening to requests at ", echoListen)
	err := http.ListenAndServe(echoListen, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, fmt.Sprintf("invalid method. Expected POST, received %s", r.Method), http.StatusBadRequest)
		return
	}

	var readings []*api.DataReading

	err := json.NewDecoder(r.Body).Decode(&readings)
	if err != nil {
		writeError(w, fmt.Sprintf("decoding body: %+v", err), http.StatusBadRequest)
		return
	}
	fmt.Printf("-- %s %s -> created %d\n", r.Method, r.URL.Path, http.StatusCreated)
	fmt.Printf("received %d readings:\n", len(readings))
	for _, r := range readings {
		fmt.Printf("%+v\n", r)
	}
	fmt.Println("-----")

	fmt.Fprintf(w, `{ "status": "ok" }`)
	w.Header().Set("Content-Type", "application/json")
}

func writeError(w http.ResponseWriter, err string, code int) {
	fmt.Printf("-- error %d -> %s\n", code, err)
	w.Header().Set("Content-Type", "application/json")
	http.Error(w, fmt.Sprintf(`"{ "error": "%s", "code": %d }"`, err, code), code)
}
