package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-resty/resty/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ronaldyuwandika/go-http-exporter/promexp"
	"github.com/ronaldyuwandika/go-http-exporter/restyexporter"
)

func main() {
	// 1. Create Prometheus exporter
	promExporter := promexp.New(nil)
	promExporter.Register()

	// 2. Create resty client and install the exporter
	client := resty.New()
	restyexporter.Install(client, promExporter)

	// 3. Make requests
	urls := []string{
		"https://httpbin.org/get",
		"https://httpbin.org/json",
	}
	for _, url := range urls {
		resp, err := client.R().Get(url)
		if err != nil {
			log.Printf("request failed: %v", err)
			continue
		}
		fmt.Printf("GET %s -> %s\n", url, resp.Status())
	}

	// 4. Serve Prometheus metrics
	http.Handle("/metrics", promhttp.Handler())
	log.Println("metrics at http://localhost:2113/metrics")
	log.Fatal(http.ListenAndServe(":2113", nil))
}
