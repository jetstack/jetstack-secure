package main

import (
	"encoding/json" // Import to safely escape error strings
	"fmt"           // Import to format strings
	"log"
	"net/http"
	"runtime/coverage"
)

func startCoverageServer() {
	adminMux := http.NewServeMux()

	adminMux.HandleFunc("/_debug/coverage/download", func(w http.ResponseWriter, r *http.Request) {
		// Simple info log as a JSON string
		log.Println(`{"level":"info","message":"Received request to download coverage counter data"}`)

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="coverage.out"`)

		if err := coverage.WriteCounters(w); err != nil {
			// Safely marshal the error to escape quotes and special characters
			escapedError, _ := json.Marshal(err.Error())

			// Construct the final JSON string and log it
			log.Println(
				fmt.Sprintf(`{"level":"error","message":"Error writing coverage counters to response","error":%s}`, string(escapedError)),
			)
		}
	})

	adminMux.HandleFunc("/_debug/coverage/meta/download", func(w http.ResponseWriter, r *http.Request) {
		log.Println(`{"level":"info","message":"Received request to download coverage metadata"}`)

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="coverage.meta"`)

		if err := coverage.WriteMeta(w); err != nil {
			escapedError, _ := json.Marshal(err.Error())
			log.Println(
				fmt.Sprintf(`{"level":"error","message":"Error writing coverage metadata to response","error":%s}`, string(escapedError)),
			)
		}
	})

	go func() {
		log.Println(`{"level":"info","message":"Starting private admin server","address":"localhost:8089"}`)
		if err := http.ListenAndServe("localhost:8089", adminMux); err != nil {
			escapedError, _ := json.Marshal(err.Error())
			log.Println(
				fmt.Sprintf(`{"level":"error","message":"Admin server failed","error":%s}`, string(escapedError)),
			)
		}
	}()
}
