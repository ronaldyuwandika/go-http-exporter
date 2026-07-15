// Package httpexporter provides a pluggable HTTP client metrics SDK with
// built-in support for net/http, go-resty/resty, imroc/req, and dghubble/sling.
//
// Core types:
//
//	Exporter      — metrics collector interface
//	RequestInfo   — per-request metadata (method, path, host, DNS timing)
//	ResponseInfo  — per-response metadata (status, duration, error)
//	PathNormalizer — maps dynamic paths to patterns (/users/42 → /users/:id)
//
// Quick start:
//
//	transport := httptransport.New(httpexporter.WithExporter(myExporter))
//	client := &http.Client{Transport: transport}
package httpexporter

import (
	"context"
	"net/http"
	"time"
)

// Exporter collects metrics about HTTP request/response cycles.
type Exporter interface {
	Export(ctx context.Context, req *RequestInfo, resp *ResponseInfo)
}

// ExporterFunc is a function adapter implementing Exporter.
type ExporterFunc func(ctx context.Context, req *RequestInfo, resp *ResponseInfo)

func (f ExporterFunc) Export(ctx context.Context, req *RequestInfo, resp *ResponseInfo) {
	f(ctx, req, resp)
}

// PathNormalizer converts a raw URL path into a normalized form for metric
// grouping. Returns the normalized path or the original if no normalization
// is needed. Use SlugNormalizer for built-in slug/param detection.
type PathNormalizer func(path string) string

// RequestInfo holds metadata about an HTTP request.
type RequestInfo struct {
	Method         string
	URL            string
	Host           string
	Path           string
	NormalizedPath string
	Headers        http.Header
	BodySize       int64
	DNSDuration    time.Duration
}

// ResponseInfo holds metadata about an HTTP response.
type ResponseInfo struct {
	StatusCode int
	Status     string
	Headers    http.Header
	BodySize   int64
	Duration   time.Duration
	Error      error
}

// MultiExporter fans out Export calls to multiple exporters.
type MultiExporter []Exporter

var _ Exporter = MultiExporter(nil)

func (m MultiExporter) Export(ctx context.Context, req *RequestInfo, resp *ResponseInfo) {
	for _, e := range m {
		e.Export(ctx, req, resp)
	}
}

// NoopExporter is an exporter that does nothing.
var NoopExporter Exporter = ExporterFunc(func(ctx context.Context, req *RequestInfo, resp *ResponseInfo) {})

// CopyRequestInfo creates a shallow copy of RequestInfo.
func CopyRequestInfo(r *RequestInfo) *RequestInfo {
	if r == nil {
		return nil
	}
	cpy := *r
	cpy.Headers = r.Headers.Clone()
	return &cpy
}

// CopyResponseInfo creates a shallow copy of ResponseInfo.
func CopyResponseInfo(r *ResponseInfo) *ResponseInfo {
	if r == nil {
		return nil
	}
	cpy := *r
	cpy.Headers = r.Headers.Clone()
	return &cpy
}

// NewRequestInfo extracts RequestInfo from an http.Request.
func NewRequestInfo(r *http.Request) *RequestInfo {
	info := &RequestInfo{
		Method:  r.Method,
		Host:    r.Host,
		Headers: r.Header.Clone(),
	}
	if r.URL != nil {
		info.URL = r.URL.String()
		info.Path = r.URL.Path
	}
	if r.ContentLength > 0 {
		info.BodySize = r.ContentLength
	}
	return info
}

// NewResponseInfo creates a ResponseInfo from an http.Response and timing.
func NewResponseInfo(resp *http.Response, dur time.Duration, err error) *ResponseInfo {
	info := &ResponseInfo{
		Duration: dur,
		Error:    err,
	}
	if resp != nil {
		info.StatusCode = resp.StatusCode
		info.Status = resp.Status
		info.Headers = resp.Header.Clone()
		if resp.ContentLength >= 0 {
			info.BodySize = resp.ContentLength
		}
	}
	return info
}
