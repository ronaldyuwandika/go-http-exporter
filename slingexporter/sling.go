package slingexporter

import (
	"net/http"

	"github.com/ronaldyuwandika/go-http-exporter"
	"github.com/ronaldyuwandika/go-http-exporter/httptransport"
)

// NewHTTPClient returns an *http.Client with export transport wrapping base.
// If base is nil, http.DefaultClient is used. Accepts optional httptransport
// options for path normalization and other configuration.
func NewHTTPClient(e httpexporter.Exporter, base *http.Client, opts ...httpexporter.Option) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	rt := base.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}

	allOpts := make([]httpexporter.Option, 0, len(opts)+2)
	allOpts = append(allOpts, httpexporter.WithExporter(e), httpexporter.WithTransport(rt))
	allOpts = append(allOpts, opts...)

	return &http.Client{
		Transport:     httptransport.New(allOpts...),
		CheckRedirect: base.CheckRedirect,
		Jar:           base.Jar,
		Timeout:       base.Timeout,
	}
}
