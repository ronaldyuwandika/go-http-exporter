package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ronaldyuwandika/go-http-exporter"
	"github.com/ronaldyuwandika/go-http-exporter/httptransport"
	"github.com/ronaldyuwandika/go-http-exporter/promexp"
)

func main() {
	// 1. Create a Prometheus exporter
	promExporter := promexp.New(nil)
	promExporter.Register()

	// 2. Wrap the default transport with the exporter
	transport := httptransport.New(
		httpexporter.WithExporter(promExporter),
	)

	// 3. Create an HTTP client with the instrumented transport
	client := &http.Client{Transport: transport}

	// 4. Make some requests
	urls := []string{
		"https://httpbin.org/get",
		"https://httpbin.org/post",
	}
	for _, url := range urls {
		resp, err := client.Get(url)
		if err != nil {
			log.Printf("request failed: %v", err)
			continue
		}
		resp.Body.Close()
		fmt.Printf("GET %s -> %s\n", url, resp.Status)
	}

	// 5. Serve Prometheus metrics
	http.Handle("/metrics", promhttp.Handler())
	log.Println("metrics available at http://localhost:2112/metrics")
	log.Fatal(http.ListenAndServe(":2112", nil))
}
