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
	"bufio"
	"net"
	"net/http"
	"time"

	"github.com/ronaldyuwandika/go-http-exporter"
)

// responseWriter wraps http.ResponseWriter to capture the status code and
// response body size while preserving optional interfaces (Flusher, Hijacker,
// Pusher) that downstream handlers or the HTTP server may rely on.
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

// Flush sends buffered data to the client. Delegates to the underlying writer
// if it implements http.Flusher, otherwise is a no-op.
func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack lets the caller take over the connection. Delegates to the underlying
// writer if it implements http.Hijacker, otherwise returns ErrNotSupported.
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Push initiates an HTTP/2 server push. Delegates to the underlying writer if
// it implements http.Pusher, otherwise returns ErrNotSupported.
func (w *responseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
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

	defer func() {
		if v := recover(); v != nil {
			dur := time.Since(start)
			rsi := &httpexporter.ResponseInfo{
				StatusCode: http.StatusInternalServerError,
				Status:     http.StatusText(http.StatusInternalServerError),
				Duration:   dur,
				BodySize:   sw.bytesWritten,
			}
			if m.opts.Exporter != nil {
				m.opts.Exporter.Export(r.Context(), ri, rsi)
			}
			panic(v)
		}

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
	}()

	m.handler.ServeHTTP(sw, r)
}
