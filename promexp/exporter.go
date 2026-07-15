// Package promexp provides a Prometheus Exporter implementation that collects
// HTTP client metrics using the prometheus client_golang library.
//
//	exporter := promexp.New(promexp.DefaultBuckets)
//	client := &http.Client{
//	    Transport: httptransport.New(
//	        httpexporter.WithExporter(exporter),
//	    ),
//	}
package promexp

import (
	"context"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/ronaldyuwandika/go-http-exporter"
)

const (
	namespace = "http_client"

	subsystemRequests = "requests"
	subsystemDuration = "duration"
	subsystemDns      = "dns"
	subsystemSize     = "size"
	subsystemErrors   = "errors"
)

// DefaultBuckets provides sensible latency buckets (seconds) for HTTP clients.
var DefaultBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
}

// DefaultDNSBuckets provides sensible latency buckets for DNS resolution.
var DefaultDNSBuckets = []float64{
	0.001, 0.002, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0,
}

// Exporter records HTTP client metrics via Prometheus.
type Exporter struct {
	requestsTotal   *prometheus.CounterVec
	statusCodeTotal *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	dnsDuration     *prometheus.HistogramVec
	requestSize     *prometheus.HistogramVec
	responseSize    *prometheus.HistogramVec
	errorsTotal     *prometheus.CounterVec
}

// Verify Exporter implements httpexporter.Exporter.
var _ httpexporter.Exporter = (*Exporter)(nil)

// New creates a Prometheus Exporter with custom duration buckets.
func New(buckets []float64) *Exporter {
	if buckets == nil {
		buckets = DefaultBuckets
	}

	labels := []string{"method", "host", "path", "status_code"}
	statusCodeLabels := []string{"status_code", "status_family"}

	e := &Exporter{
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemRequests,
			Name:      "total",
			Help:      "Total number of HTTP requests made.",
		}, labels),

		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemDuration,
			Name:      "seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   buckets,
		}, labels),

		dnsDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemDns,
			Name:      "lookup_seconds",
			Help:      "DNS lookup duration in seconds.",
			Buckets:   DefaultDNSBuckets,
		}, labels),

		requestSize: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemSize,
			Name:      "request_bytes",
			Help:      "HTTP request body size in bytes.",
			Buckets:   prometheus.ExponentialBuckets(256, 2, 12),
		}, labels),

		responseSize: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemSize,
			Name:      "response_bytes",
			Help:      "HTTP response body size in bytes.",
			Buckets:   prometheus.ExponentialBuckets(256, 2, 12),
		}, labels),

		errorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemErrors,
			Name:      "total",
			Help:      "Total number of HTTP request errors.",
		}, labels),

		statusCodeTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "response",
			Name:      "status_code_total",
			Help:      "Total count of HTTP response status codes.",
		}, statusCodeLabels),
	}

	return e
}

// Register registers all metrics with the default Prometheus registry.
func (e *Exporter) Register() {
	prometheus.MustRegister(e)
}

// MustRegister registers all metrics with the given registerer, panicking on error.
func (e *Exporter) MustRegister(r prometheus.Registerer) {
	r.MustRegister(e)
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	e.requestsTotal.Describe(ch)
	e.statusCodeTotal.Describe(ch)
	e.requestDuration.Describe(ch)
	e.dnsDuration.Describe(ch)
	e.requestSize.Describe(ch)
	e.responseSize.Describe(ch)
	e.errorsTotal.Describe(ch)
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.requestsTotal.Collect(ch)
	e.statusCodeTotal.Collect(ch)
	e.requestDuration.Collect(ch)
	e.dnsDuration.Collect(ch)
	e.requestSize.Collect(ch)
	e.responseSize.Collect(ch)
	e.errorsTotal.Collect(ch)
}

// Export records HTTP metrics from the request/response cycle.
func (e *Exporter) Export(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
	statusCode := ""
	if resp != nil {
		statusCode = strconv.Itoa(resp.StatusCode)
	}

	// Prefer normalized path to prevent cardinality explosion.
	labelPath := req.Path
	if req.NormalizedPath != "" {
		labelPath = req.NormalizedPath
	}

	labels := prometheus.Labels{
		"method":      req.Method,
		"host":        req.Host,
		"path":        labelPath,
		"status_code": statusCode,
	}

	e.requestsTotal.With(labels).Inc()
	e.requestDuration.With(labels).Observe(resp.Duration.Seconds())

	if resp != nil && resp.StatusCode > 0 {
		e.statusCodeTotal.With(prometheus.Labels{
			"status_code":   statusCode,
			"status_family": statusFamily(resp.StatusCode),
		}).Inc()
	}

	if req.DNSDuration > 0 {
		e.dnsDuration.With(labels).Observe(req.DNSDuration.Seconds())
	}

	if req.BodySize > 0 {
		e.requestSize.With(labels).Observe(float64(req.BodySize))
	}
	if resp != nil && resp.BodySize > 0 {
		e.responseSize.With(labels).Observe(float64(resp.BodySize))
	}
	if resp != nil && resp.Error != nil {
		e.errorsTotal.With(labels).Inc()
	}
}

func statusFamily(code int) string {
	switch {
	case code < 200:
		return "1xx"
	case code < 300:
		return "2xx"
	case code < 400:
		return "3xx"
	case code < 500:
		return "4xx"
	default:
		return "5xx"
	}
}
