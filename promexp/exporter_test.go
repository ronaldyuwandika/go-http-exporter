package promexp

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/ronaldyuwandika/go-http-exporter"
)

func TestNewWithDefaultBuckets(t *testing.T) {
	e := New(nil)
	if e == nil {
		t.Fatal("expected non-nil exporter")
	}
}

func TestExport(t *testing.T) {
	e := New(nil)

	req := &httpexporter.RequestInfo{
		Method:         "POST",
		URL:            "https://api.example.com/users",
		Host:           "api.example.com",
		Path:           "/users/42",
		NormalizedPath: "/users/:id",
		BodySize:       256,
	}
	resp := &httpexporter.ResponseInfo{
		StatusCode: 201,
		Status:     "201 Created",
		Duration:   150 * time.Millisecond,
		BodySize:   1024,
	}

	e.Export(context.Background(), req, resp)

	ch := make(chan prometheus.Metric, 10)
	e.Collect(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}
	if count == 0 {
		t.Fatal("expected metrics to be collected")
	}
}

func TestExportWithDNS(t *testing.T) {
	e := New(nil)

	req := &httpexporter.RequestInfo{
		Method:         "GET",
		URL:            "https://api.example.com/data",
		Host:           "api.example.com",
		Path:           "/data",
		NormalizedPath: "/data",
		DNSDuration:    5 * time.Millisecond,
	}
	resp := &httpexporter.ResponseInfo{
		StatusCode: 200,
		Duration:   100 * time.Millisecond,
	}

	e.Export(context.Background(), req, resp)

	ch := make(chan prometheus.Metric, 20)
	e.Collect(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}
	if count == 0 {
		t.Fatal("expected metrics to be collected")
	}
}

func TestExportNormalizedPath(t *testing.T) {
	e := New(nil)

	req := &httpexporter.RequestInfo{
		Method:         "GET",
		Host:           "api.example.com",
		Path:           "/users/550e8400-e29b-41d4-a716-446655440000",
		NormalizedPath: "/users/:uuid",
	}
	resp := &httpexporter.ResponseInfo{
		StatusCode: 404,
		Duration:   50 * time.Millisecond,
	}

	// Should use NormalizedPath, not the raw UUID path.
	e.Export(context.Background(), req, resp)

	ch := make(chan prometheus.Metric, 10)
	e.Collect(ch)
	close(ch)

	for m := range ch {
		desc := m.Desc().String()
		// Verify we don't see the raw UUID in the metric description.
		if descMatches(desc, "550e8400") {
			t.Fatal("raw UUID leaked into metric labels")
		}
		_ = desc
	}
}

func TestExportError(t *testing.T) {
	e := New(nil)

	req := &httpexporter.RequestInfo{
		Method: "GET",
		URL:    "https://down.example.com",
		Host:   "down.example.com",
		Path:   "/",
	}
	resp := &httpexporter.ResponseInfo{
		Error:    http.ErrHandlerTimeout,
		Duration: 10 * time.Second,
	}

	e.Export(context.Background(), req, resp)
}

func TestRegister(t *testing.T) {
	e := New(nil)
	reg := prometheus.NewRegistry()
	e.MustRegister(reg)
}

func TestImplementsExporter(t *testing.T) {
	var _ httpexporter.Exporter = (*Exporter)(nil)
}

func TestImplementsCollector(t *testing.T) {
	var _ prometheus.Collector = (*Exporter)(nil)
}

func descMatches(desc, substr string) bool {
	return len(desc) > 0 && contains(desc, substr)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
