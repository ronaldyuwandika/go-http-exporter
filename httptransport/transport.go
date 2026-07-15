package httptransport

import (
	"context"
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
		resp.Body = &trackedBody{
			ReadCloser: resp.Body,
			rsi:        rsi,
			ttfb:       ttfb,
			ri:         ri,
			ctx:        ctx,
			exp:        t.opts.Exporter,
		}
	} else {
		if t.opts.Exporter != nil {
			t.opts.Exporter.Export(ctx, ri, rsi)
		}
	}

	return resp, nil
}

// trackedBody wraps an io.ReadCloser to track actual bytes read, read duration,
// and read errors. Export state is stored in struct fields (no closure allocation).
// Export fires once even if Close is called multiple times.
type trackedBody struct {
	io.ReadCloser
	closeDone int32
	bytesRead int64
	readStart time.Time
	readOnce  sync.Once
	readErr   error

	rsi  *httpexporter.ResponseInfo
	ttfb time.Duration
	ri   *httpexporter.RequestInfo
	ctx  context.Context
	exp  httpexporter.Exporter
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
	if atomic.CompareAndSwapInt32(&b.closeDone, 0, 1) {
		var bodyDur time.Duration
		if !b.readStart.IsZero() {
			bodyDur = time.Since(b.readStart)
		}
		b.rsi.Duration = b.ttfb + bodyDur
		b.rsi.BodySize = b.bytesRead
		if b.readErr != nil && b.rsi.Error == nil {
			b.rsi.Error = b.readErr
		}
		if b.exp != nil {
			b.exp.Export(b.ctx, b.ri, b.rsi)
		}
	}
	return err
}
