package reqexporter

import (
	"context"
	"time"

	"github.com/imroc/req/v3"
	"github.com/ronaldyuwandika/go-http-exporter"
)

// NewClient creates an instrumented req.Client with the exporter pre-installed.
func NewClient(e httpexporter.Exporter) *req.Client {
	c := req.C()
	Install(c, e)
	return c
}

// Install attaches HTTP export hooks to a req client.
//
// For DNS latency or path normalization, combine with httptransport:
//
//	c := req.C()
//	c.SetTransport(httptransport.New(httpexporter.WithExporter(e), ...))
func Install(client *req.Client, e httpexporter.Exporter) {
	if e == nil {
		return
	}

	client.OnBeforeRequest(func(c *req.Client, r *req.Request) error {
		if r.Context() == nil {
			r.SetContext(context.Background())
		}
		r.SetContext(context.WithValue(r.Context(), ctxKeyStartTime, time.Now()))
		return nil
	})

	client.OnAfterResponse(func(c *req.Client, resp *req.Response) error {
		start, ok := resp.Request.Context().Value(ctxKeyStartTime).(time.Time)
		if !ok {
			start = time.Now()
		}
		dur := time.Since(start)

		var ri *httpexporter.RequestInfo
		if resp.Request.RawRequest != nil {
			ri = httpexporter.NewRequestInfo(resp.Request.RawRequest)
		} else {
			ri = &httpexporter.RequestInfo{}
		}
		rsi := httpexporter.NewResponseInfo(resp.Response, dur, nil)
		// req buffers the body; ContentLength may be -1 for chunked.
		rsi.BodySize = int64(len(resp.Bytes()))
		e.Export(resp.Request.Context(), ri, rsi)
		return nil
	})
}

type ctxKey string

const ctxKeyStartTime ctxKey = "httpexporter_req_start"
