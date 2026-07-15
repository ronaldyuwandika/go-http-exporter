package httptransport

import (
	"io"
	"net/http"
	"net/http/httptrace"
	"sync"
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
	ttfb := time.Since(start)

	ctx := req.Context()
	ri := httpexporter.NewRequestInfo(req)
	ri.DNSDuration = time.Duration(atomic.LoadInt64(&dnsDurNano))

	if t.opts.PathNormalizer != nil {
		ri.NormalizedPath = t.opts.PathNormalizer(ri.Path)
	}

	rsi := httpexporter.NewResponseInfo(resp, ttfb, err)

	if err != nil {
		if t.opts.Exporter != nil {
			t.opts.Exporter.Export(ctx, ri, rsi)
		}
		return resp, err
	}

	if resp.Body != nil {
		exporter := t.opts.Exporter
		resp.Body = &trackedBody{
			ReadCloser: resp.Body,
			onClose: func(bytesRead int64, bodyReadDur time.Duration, readErr error) {
				rsi.Duration = ttfb + bodyReadDur
				rsi.BodySize = bytesRead
				if readErr != nil && rsi.Error == nil {
					rsi.Error = readErr
				}
				if exporter != nil {
					exporter.Export(ctx, ri, rsi)
				}
			},
		}
	} else {
		if t.opts.Exporter != nil {
			t.opts.Exporter.Export(ctx, ri, rsi)
		}
	}

	return resp, nil
}

// trackedBody wraps an io.ReadCloser to track actual bytes read, read duration,
// and read errors. The onClose callback fires exactly once even if Close is
// called multiple times (a common Go pattern).
type trackedBody struct {
	io.ReadCloser
	onClose   func(bytesRead int64, bodyReadDuration time.Duration, readErr error)
	closeOnce sync.Once
	bytesRead int64
	readStart time.Time
	readOnce  sync.Once
	readErr   error
}

func (b *trackedBody) Read(p []byte) (int, error) {
	b.readOnce.Do(func() {
		b.readStart = time.Now()
	})
	n, err := b.ReadCloser.Read(p)
	b.bytesRead += int64(n)
	if err != nil && err != io.EOF {
		b.readErr = err
	}
	return n, err
}

func (b *trackedBody) Close() error {
	err := b.ReadCloser.Close()
	b.closeOnce.Do(func() {
		var bodyDur time.Duration
		if !b.readStart.IsZero() {
			bodyDur = time.Since(b.readStart)
		}
		b.onClose(b.bytesRead, bodyDur, b.readErr)
	})
	return err
}
