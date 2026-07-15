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

func TestStatusFamily(t *testing.T) {
	tests := []struct {
		code   int
		family string
	}{
		{100, "1xx"}, {199, "1xx"},
		{200, "2xx"}, {299, "2xx"},
		{301, "3xx"}, {399, "3xx"},
		{400, "4xx"}, {499, "4xx"},
		{500, "5xx"}, {502, "5xx"}, {503, "5xx"},
	}
	for _, tt := range tests {
		got := statusFamily(tt.code)
		if got != tt.family {
			t.Errorf("statusFamily(%d) = %s, want %s", tt.code, got, tt.family)
		}
	}
}

func TestStatusCodeCounter(t *testing.T) {
	e := New(nil)

	req := &httpexporter.RequestInfo{
		Method: "GET",
		Host:   "api.example.com",
		Path:   "/ok",
	}
	resp := &httpexporter.ResponseInfo{
		StatusCode: 200,
		Duration:   10 * time.Millisecond,
	}

	e.Export(context.Background(), req, resp)

	// Verify the status_code counter was created with status_family label.
	found := false
	ch := make(chan prometheus.Metric, 30)
	e.Collect(ch)
	close(ch)

	for m := range ch {
		desc := m.Desc().String()
		if descMatches(desc, "status_code_total") {
			found = true
			if !descMatches(desc, "status_family") {
				t.Fatal("status_code_total missing status_family label")
			}
		}
	}
	if !found {
		t.Fatal("status_code_total metric not found in collection")
	}
}

func TestStatusCodeCounterMultiple(t *testing.T) {
	e := New(nil)

	for _, code := range []int{200, 200, 404, 500, 200, 503} {
		e.Export(context.Background(),
			&httpexporter.RequestInfo{Method: "GET", Host: "api", Path: "/x"},
			&httpexporter.ResponseInfo{StatusCode: code, Duration: time.Millisecond},
		)
	}

	ch := make(chan prometheus.Metric, 40)
	e.Collect(ch)
	close(ch)

	statusCodeTotalCount := 0
	for m := range ch {
		if descMatches(m.Desc().String(), "status_code_total") {
			statusCodeTotalCount++
		}
	}
	if statusCodeTotalCount < 3 {
		t.Fatalf("expected at least 3 status_code_total series, got %d", statusCodeTotalCount)
	}
}

func TestStatusCodeCounterNoStatusCode(t *testing.T) {
	e := New(nil)

	req := &httpexporter.RequestInfo{Method: "GET", Host: "api", Path: "/"}
	resp := &httpexporter.ResponseInfo{
		Error:    http.ErrHandlerTimeout,
		Duration: time.Second,
	}

	// Should not panic when StatusCode is 0 (error case).
	e.Export(context.Background(), req, resp)
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
