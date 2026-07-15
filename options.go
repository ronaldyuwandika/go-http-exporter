package httpexporter

import "net/http"

// Options configures the exporter behavior.
type Options struct {
	Exporter       Exporter
	Transport      http.RoundTripper
	ShouldExport   func(req *http.Request) bool
	PathNormalizer PathNormalizer
}

// Option modifies Options.
type Option func(*Options)

// WithExporter sets the exporter to use.
func WithExporter(e Exporter) Option {
	return func(o *Options) { o.Exporter = e }
}

// WithTransport sets the base RoundTripper to wrap.
func WithTransport(t http.RoundTripper) Option {
	return func(o *Options) { o.Transport = t }
}

// WithShouldExport sets a filter function to decide which requests to export.
func WithShouldExport(fn func(req *http.Request) bool) Option {
	return func(o *Options) { o.ShouldExport = fn }
}

// WithPathNormalizer sets the path normalizer for grouping dynamic segments.
func WithPathNormalizer(fn PathNormalizer) Option {
	return func(o *Options) { o.PathNormalizer = fn }
}

// DefaultOptions returns the default Options.
func DefaultOptions() *Options {
	return &Options{
		Exporter:  NoopExporter,
		Transport: http.DefaultTransport,
		ShouldExport: func(req *http.Request) bool {
			return true
		},
	}
}
