package httpexporter

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestExporterFunc(t *testing.T) {
	called := false
	fn := ExporterFunc(func(ctx context.Context, req *RequestInfo, resp *ResponseInfo) {
		called = true
	})
	fn.Export(context.Background(), &RequestInfo{}, &ResponseInfo{})
	if !called {
		t.Fatal("ExporterFunc was not called")
	}
}

func TestMultiExporter(t *testing.T) {
	count := 0
	inc := ExporterFunc(func(ctx context.Context, req *RequestInfo, resp *ResponseInfo) {
		count++
	})

	m := MultiExporter{inc, inc, inc}
	m.Export(context.Background(), &RequestInfo{}, &ResponseInfo{})

	if count != 3 {
		t.Fatalf("expected 3 exports, got %d", count)
	}
}

func TestMultiExporterImplements(t *testing.T) {
	var _ Exporter = MultiExporter(nil)
}

func TestCopyRequestInfo(t *testing.T) {
	original := &RequestInfo{
		Method:  "GET",
		URL:     "http://example.com",
		Host:    "example.com",
		Path:    "/",
		Headers: make(http.Header),
	}
	original.Headers.Set("X-Test", "value")

	cpy := CopyRequestInfo(original)
	if cpy.Headers.Get("X-Test") != "value" {
		t.Fatal("headers not copied")
	}

	original.Headers.Set("X-Test", "modified")
	if cpy.Headers.Get("X-Test") != "value" {
		t.Fatal("header copy is not independent")
	}
}

func TestCopyResponseInfo(t *testing.T) {
	dur := 100 * time.Millisecond
	original := &ResponseInfo{
		StatusCode: 200,
		Status:     "200 OK",
		Duration:   dur,
	}
	cpy := CopyResponseInfo(original)
	if cpy.Duration != dur {
		t.Fatalf("expected %v, got %v", dur, cpy.Duration)
	}
}
