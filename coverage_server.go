package main

import (
	"log"
	"net/http"
	"runtime/coverage"
)

func startCoverageServer() {
	//log.Println("Coverage build detected. Starting private admin server on localhost:8081...")
	adminMux := http.NewServeMux()

	adminMux.HandleFunc("/_debug/coverage/download", func(w http.ResponseWriter, r *http.Request) {
		log.Println("{\n  \"message\": \"Received request to download coverage counter data...\"\n}")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="coverage.out"`)

		if err := coverage.WriteCounters(w); err != nil {
			log.Printf("Error writing coverage counters to response: %v", err)
		}
	})

	adminMux.HandleFunc("/_debug/coverage/meta/download", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Received request to download coverage metadata...")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="coverage.meta"`)

		if err := coverage.WriteMeta(w); err != nil {
			log.Printf("Error writing coverage metadata to response: %v", err)
		}
	})

	go func() {
		if err := http.ListenAndServe("localhost:8089", adminMux); err != nil {
			log.Printf("Admin server failed: %v", err)
		}
	}()
}
