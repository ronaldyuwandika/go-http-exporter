package restyexporter

import (
	"context"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/ronaldyuwandika/go-http-exporter"
)

// Install attaches HTTP export hooks to a resty client.
//
// For DNS latency metrics or path normalization, combine with httptransport
// by wrapping the underlying transport:
//
//	client := resty.New()
//	client.SetTransport(httptransport.New(
//	    httpexporter.WithExporter(exporter),
//	    httpexporter.WithPathNormalizer(httpexporter.SlugNormalizer),
//	))
func Install(client *resty.Client, e httpexporter.Exporter) {
	client.OnBeforeRequest(func(c *resty.Client, r *resty.Request) error {
		if r.Context() == nil {
			r.SetContext(context.Background())
		}
		r.SetContext(context.WithValue(r.Context(), ctxKeyStartTime, time.Now()))
		return nil
	})

	client.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		start, ok := resp.Request.Context().Value(ctxKeyStartTime).(time.Time)
		if !ok {
			start = time.Now()
		}
		dur := time.Since(start)

		ri := httpexporter.NewRequestInfo(resp.Request.RawRequest)
		rsi := httpexporter.NewResponseInfo(resp.RawResponse, dur, nil)
		e.Export(resp.Request.Context(), ri, rsi)
		return nil
	})

	client.OnError(func(r *resty.Request, err error) {
		start, ok := r.Context().Value(ctxKeyStartTime).(time.Time)
		if !ok {
			start = time.Now()
		}
		dur := time.Since(start)

		ri := httpexporter.NewRequestInfo(r.RawRequest)
		e.Export(r.Context(), ri, httpexporter.NewResponseInfo(nil, dur, err))
	})
}

type ctxKey string

const ctxKeyStartTime ctxKey = "httpexporter_resty_start"
