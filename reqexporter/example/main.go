package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/ronaldyuwandika/go-http-exporter"
	"github.com/ronaldyuwandika/go-http-exporter/reqexporter"
)

func main() {
	logExporter := httpexporter.ExporterFunc(
		func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
			fmt.Fprintf(os.Stderr, "[%s] %s %s -> %d (%s)\n",
				resp.Duration, req.Method, req.URL,
				resp.StatusCode, resp.Status)
		},
	)

	client := reqexporter.NewClient(logExporter)

	resp, err := client.R().Get("https://httpbin.org/get")
	if err != nil {
		log.Fatalf("request failed: %v", err)
	}
	fmt.Printf("GET https://httpbin.org/get -> %s\n", resp.Status)
}
