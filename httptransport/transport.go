package httptransport

import (
	"net/http"
	"net/http/httptrace"
	"sync/atomic"
	"time"

	"github.com/ronaldyuwandika/go-http-exporter"
)

// Transport wraps an http.RoundTripper and exports metrics.
// It satisfies http.RoundTripper and can be used as a drop-in replacement
// for any http.Client's Transport.
type Transport struct {
	base http.RoundTripper
	opts *httpexporter.Options
}

// New creates a Transport with the given options.
func New(opts ...httpexporter.Option) *Transport {
	options := httpexporter.DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}
	if options.Transport == nil {
		options.Transport = http.DefaultTransport
	}
	return &Transport{
		base: options.Transport,
		opts: options,
	}
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.opts.ShouldExport != nil && !t.opts.ShouldExport(req) {
		return t.base.RoundTrip(req)
	}

	var dnsStartNano int64
	var dnsDurNano int64

	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			atomic.StoreInt64(&dnsStartNano, time.Now().UnixNano())
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			start := atomic.LoadInt64(&dnsStartNano)
			if start > 0 {
				atomic.StoreInt64(&dnsDurNano, time.Now().UnixNano()-start)
			}
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	dur := time.Since(start)

	ctx := req.Context()
	ri := httpexporter.NewRequestInfo(req)
	ri.DNSDuration = time.Duration(atomic.LoadInt64(&dnsDurNano))

	if t.opts.PathNormalizer != nil {
		ri.NormalizedPath = t.opts.PathNormalizer(ri.Path)
	}

	rsi := httpexporter.NewResponseInfo(resp, dur, err)
	t.opts.Exporter.Export(ctx, ri, rsi)

	return resp, err
}
