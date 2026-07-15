// Package httpserver provides an HTTP server middleware that exports request
// metrics via the httpexporter.Exporter interface.
//
// Usage:
//
//	exporter := promexp.NewServer(nil)
//	handler := httpserver.New(myHandler, httpexporter.WithExporter(exporter))
//	http.ListenAndServe(":8080", handler)
//
// Path normalization is enabled by default (SlugNormalizer).
// Duration buckets start at 0.1ms — use promexp.NewServer for sub-ms precision.
package httpserver

import (
	"net/http"
	"time"

	"github.com/ronaldyuwandika/go-http-exporter"
)

// responseWriter wraps http.ResponseWriter to capture the status code and
// response body size.
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += int64(n)
	return n, err
}

// Middleware wraps an http.Handler and exports metrics for every request.
type Middleware struct {
	handler http.Handler
	opts    *httpexporter.Options
}

// New wraps handler with request instrumentation.
// Options are the same httpexporter.Option values used by the client transport:
//
//	httpexporter.WithExporter(e)
//	httpexporter.WithPathNormalizer(httpexporter.SlugNormalizer)
//	httpexporter.WithShouldExport(fn)
func New(handler http.Handler, opts ...httpexporter.Option) *Middleware {
	options := httpexporter.DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}
	return &Middleware{
		handler: handler,
		opts:    options,
	}
}

func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.opts.ShouldExport != nil && !m.opts.ShouldExport(r) {
		m.handler.ServeHTTP(w, r)
		return
	}

	start := time.Now()

	ri := httpexporter.NewRequestInfo(r)
	if m.opts.PathNormalizer != nil {
		ri.NormalizedPath = m.opts.PathNormalizer(ri.Path)
	}

	sw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
	m.handler.ServeHTTP(sw, r)

	dur := time.Since(start)
	rsi := &httpexporter.ResponseInfo{
		StatusCode: sw.statusCode,
		Status:     http.StatusText(sw.statusCode),
		Duration:   dur,
		BodySize:   sw.bytesWritten,
	}

	if m.opts.Exporter != nil {
		m.opts.Exporter.Export(r.Context(), ri, rsi)
	}
}
