package main

import (
	"bytes"
	"log"
	"net/http"
	"runtime/coverage"
)

func startCoverageServer() {
	adminMux := http.NewServeMux()

	adminMux.HandleFunc("/_debug/coverage/download", func(w http.ResponseWriter, r *http.Request) {
		log.Println("{\n  \"message\": \"Received request to download coverage counter data...\"\n}")

		var buffer bytes.Buffer

		// Attempt to write the coverage counters to the buffer.
		if err := coverage.WriteCounters(&buffer); err != nil {
			log.Printf("Error writing coverage counters to buffer: %v", err)
			// Inform the client that an internal error occurred.
			http.Error(w, "Failed to generate coverage report", http.StatusInternalServerError)
			return
		}

		// Check if any data was written to the buffer.
		if buffer.Len() == 0 {
			log.Println("Coverage data is empty. No counters were written.")
		} else {
			log.Printf("Successfully wrote %d bytes of coverage data to the buffer.", buffer.Len())
		}

		// If successful, proceed to write the buffer's content to the actual HTTP response.
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="coverage.out"`)

		// Write the captured coverage data from the buffer to the response writer.
		if _, err := w.Write(buffer.Bytes()); err != nil {
			log.Printf("Error writing coverage data from buffer to response: %v", err)
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
