package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ronaldyuwandika/go-http-exporter"
	"github.com/ronaldyuwandika/go-http-exporter/httpserver"
	"github.com/ronaldyuwandika/go-http-exporter/promexp"
)

func main() {
	// 1. Create a server-side exporter with sub-ms buckets
	serverExporter := promexp.NewServer(nil)
	serverExporter.Register()

	// 2. Wrap the handler with instrumentation
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello %s", r.URL.Query().Get("name"))
	})
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		// Dynamic path is normalized to /users/:id by SlugNormalizer
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "user %s", r.URL.Path)
	})

	handler := httpserver.New(mux,
		httpexporter.WithExporter(serverExporter),
		// PathNormalizer is on by default (SlugNormalizer)
	)

	// 3. Expose metrics
	mux.Handle("/metrics", promhttp.Handler())

	log.Println("server listening on :8080, metrics at :8080/metrics")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
