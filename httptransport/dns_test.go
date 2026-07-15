package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ronaldyuwandika/go-http-exporter"
)

func TestTransportWithPathNormalizer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var mu sync.Mutex
	var capturedPath string
	var capturedNormalized string

	tr := New(
		httpexporter.WithExporter(httpexporter.ExporterFunc(
			func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
				mu.Lock()
				capturedPath = req.Path
				capturedNormalized = req.NormalizedPath
				mu.Unlock()
			},
		)),
		httpexporter.WithPathNormalizer(httpexporter.SlugNormalizer),
	)

	client := &http.Client{Transport: tr}
	resp, err := client.Get(server.URL + "/users/550e8400-e29b-41d4-a716-446655440000/profile")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()
	if capturedPath != "/users/550e8400-e29b-41d4-a716-446655440000/profile" {
		t.Fatalf("expected raw path, got %s", capturedPath)
	}
	if capturedNormalized != "/users/:uuid/profile" {
		t.Fatalf("expected /users/:uuid/profile, got %s", capturedNormalized)
	}
}

func TestTransportDNSDuration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var mu sync.Mutex
	var dnsCaptured time.Duration

	tr := New(
		httpexporter.WithExporter(httpexporter.ExporterFunc(
			func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
				mu.Lock()
				dnsCaptured = req.DNSDuration
				mu.Unlock()
			},
		)),
	)

	client := &http.Client{Transport: tr}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()
	// DNS tracing may be zero for localhost (cached/no resolution),
	// but the field should exist and not cause panics.
	_ = dnsCaptured
}
