# Go HTTP Exporter

Prometheus-ready HTTP client & server metrics SDK for Go. One interface, both sides.

[![Go Reference](https://pkg.go.dev/badge/github.com/ronaldyuwandika/go-http-exporter.svg)](https://pkg.go.dev/github.com/ronaldyuwandika/go-http-exporter)
![Go Version](https://img.shields.io/badge/Go-1.24%2B-blue)

## Features

- **Client & server** — instrument both outgoing requests and incoming handlers
- **Plug any exporter** — Prometheus (built-in), OpenTelemetry, custom logging
- **Four clients** — `net/http`, `go-resty/resty`, `imroc/req`, `dghubble/sling`
- **Sub-ms precision** — server histograms detect down to 0.1 ms (1-digit ms)
- **Path normalization** — auto-detect UUIDs, IDs, dates, slugs to prevent cardinality explosion
- **DNS latency** — DNS resolution timing via `net/http/httptrace`
- **Per-path metrics** — latency histograms, error rates, request/response sizes per path
- **Status code counter** — dedicated counter with `status_code` + `status_family` labels (2xx/4xx/5xx)
- **Go version** — root module supports 1.24+; optional `reqexporter` sub-module at 1.25+

## Install

```bash
go get github.com/ronaldyuwandika/go-http-exporter
```

For `imroc/req` support (Go 1.25+):

```bash
go get github.com/ronaldyuwandika/go-http-exporter/reqexporter
```

## Quick start

```go
package main

import (
    "net/http"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/ronaldyuwandika/go-http-exporter"
    "github.com/ronaldyuwandika/go-http-exporter/httptransport"
    "github.com/ronaldyuwandika/go-http-exporter/promexp"
)

func main() {
    // 1. Create a Prometheus metrics collector
    exporter := promexp.New(nil)
    exporter.Register()

    // 2. Wrap the HTTP transport with path normalization
    transport := httptransport.New(
        httpexporter.WithExporter(exporter),
        httpexporter.WithPathNormalizer(httpexporter.SlugNormalizer),
    )

    // 3. Use it
    client := &http.Client{Transport: transport}
    resp, _ := client.Get("https://api.example.com/users/42")
    resp.Body.Close()

    // 4. Serve metrics
    http.Handle("/metrics", promhttp.Handler())
    http.ListenAndServe(":2112", nil)
}
```

## Client integrations

### net/http — RoundTripper wrapper

```go
transport := httptransport.New(httpexporter.WithExporter(myExporter))
client := &http.Client{Transport: transport}
```

### go-resty/resty

```go
import "github.com/ronaldyuwandika/go-http-exporter/restyexporter"

client := resty.New()
restyexporter.Install(client, myExporter)

// For DNS latency + path normalization, wrap the transport:
client.SetTransport(httptransport.New(
    httpexporter.WithExporter(myExporter),
    httpexporter.WithPathNormalizer(httpexporter.SlugNormalizer),
))
```

### imroc/req (Go 1.25+)

```go
import "github.com/ronaldyuwandika/go-http-exporter/reqexporter"

client := reqexporter.NewClient(myExporter)
```

### dghubble/sling

```go
import "github.com/ronaldyuwandika/go-http-exporter/slingexporter"

client := slingexporter.NewHTTPClient(myExporter, nil)
// With path normalization:
client := slingexporter.NewHTTPClient(myExporter, nil,
    httpexporter.WithPathNormalizer(httpexporter.SlugNormalizer),
)
```

## Server instrumentation

Wrap any `http.Handler` to capture request duration, status code, and response size. Path normalization is on by default.

```go
import (
    "github.com/ronaldyuwandika/go-http-exporter/httpserver"
    "github.com/ronaldyuwandika/go-http-exporter/promexp"
)

exporter := promexp.NewServer(nil) // http_server_ prefix, 0.1ms buckets
exporter.Register()

handler := httpserver.New(mux, httpexporter.WithExporter(exporter))
http.ListenAndServe(":8080", handler)
```

### Precision

| Side | Bucket start | Metric prefix |
|------|-------------|---------------|
| Client (`promexp.New`) | 5 ms | `http_client_` |
| Server (`promexp.NewServer`) | 0.1 ms | `http_server_` |

Server histograms start at 0.1 ms (100 µs) for single-digit millisecond detection — suitable for local handler durations that complete in microseconds.

## Prometheus client metrics

| Metric | Type | Labels |
|--------|------|--------|
| `http_client_requests_total` | Counter | method, host, path, status_code |
| `http_client_duration_seconds` | Histogram | method, host, path, status_code |
| `http_client_dns_lookup_seconds` | Histogram | method, host, path, status_code |
| `http_client_size_request_bytes` | Histogram | method, host, path, status_code |
| `http_client_size_response_bytes` | Histogram | method, host, path, status_code |
| `http_client_errors_total` | Counter | method, host, path, status_code |
| `http_client_response_status_code_total` | Counter | status_code, status_family |

## Prometheus server metrics

| Metric | Type | Labels |
|--------|------|--------|
| `http_server_requests_total` | Counter | method, host, path, status_code |
| `http_server_duration_seconds` | Histogram | method, host, path, status_code |
| `http_server_dns_lookup_seconds` | Histogram | method, host, path, status_code |
| `http_server_size_request_bytes` | Histogram | method, host, path, status_code |
| `http_server_size_response_bytes` | Histogram | method, host, path, status_code |
| `http_server_errors_total` | Counter | method, host, path, status_code |
| `http_server_response_status_code_total` | Counter | status_code, status_family |

Alerting examples:

```promql
# Error rate per status family
sum(rate(http_client_response_status_code_total{status_family=~"4xx|5xx"}[5m]))
/ sum(rate(http_client_response_status_code_total[5m]))

# 99th percentile latency per normalized path
histogram_quantile(0.99, rate(http_client_duration_seconds_bucket[5m]))
```

## Instrument all routes

The transport wrapper captures **every HTTP request** automatically — you don't need to annotate routes individually. Combined with path normalization, dynamic segments like `/users/42` are grouped into `/users/:id` for clean metrics.

### Standard net/http

Wrap the transport once and all requests through that client are instrumented:

```go
transport := httptransport.New(
    httpexporter.WithExporter(myExporter),
    // PathNormalizer is enabled by default (SlugNormalizer)
)
client := &http.Client{Transport: transport}

// All of these are automatically normalized:
client.Get("https://api.example.com/users/42")       // path: /users/:id
client.Get("https://api.example.com/users/abc-123")   // path: /users/:slug
client.Post("https://api.example.com/orders", ...)    // path: /orders
```

### go-resty/resty

```go
client := resty.New()
restyexporter.Install(client, myExporter)

// For path normalization + DNS timing, also wrap the transport:
client.SetTransport(httptransport.New(
    httpexporter.WithExporter(myExporter),
))
```

### imroc/req

```go
client := reqexporter.NewClient(myExporter)

// For path normalization + DNS timing:
client.SetTransport(httptransport.New(
    httpexporter.WithExporter(myExporter),
))
```

### dghubble/sling

```go
client := slingexporter.NewHTTPClient(myExporter, nil)
// Path normalization is on by default via the wrapped transport.
```

### Filtering specific routes

Use `WithShouldExport` to skip health-check or internal endpoints:

```go
transport := httptransport.New(
    httpexporter.WithExporter(myExporter),
    httpexporter.WithShouldExport(func(r *http.Request) bool {
        return r.URL.Path != "/health"
    }),
)
```

## Path normalization

Prevent cardinality explosion by grouping dynamic path segments:

```go
normalizer := httpexporter.SlugNormalizer

normalizer("/users/550e8400-e29b-41d4-a716-446655440000")  // "/users/:uuid"
normalizer("/orders/12345")                                  // "/orders/:id"
normalizer("/stats/2024-01-15")                              // "/stats/:date"
normalizer("/products/my-product-v2")                        // "/products/:slug"
```

Custom normalizer:

```go
routePatterns := map[string]string{"/users/:id": "/users/:id"}
normalizer := func(path string) string {
    if p, ok := routePatterns[path]; ok { return p }
    return path
}
```

## Custom exporter

```go
type myExporter struct{}

func (e *myExporter) Export(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
    log.Printf("[%s] %s %s -> %d (%v)",
        resp.Duration, req.Method, req.URL, resp.StatusCode, resp.Error)
}

transport := httptransport.New(httpexporter.WithExporter(&myExporter{}))
```

Or inline:

```go
transport := httptransport.New(httpexporter.WithExporter(
    httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
        // record metrics here
    }),
))
```

## Multiple exporters

```go
transport := httptransport.New(httpexporter.WithExporter(
    httpexporter.MultiExporter{
        promexp.New(nil),
        otelExporter,
        logExporter,
    },
))
```

## Go version compatibility

| Component | Minimum Go | Dependencies |
|-----------|-----------|--------------|
| Root module (core, httptransport, resty, sling, promexp) | 1.24 | resty/v2, prometheus/client_golang |
| reqexporter sub-module | 1.25 | imroc/req/v3, core module |

## License

MIT
