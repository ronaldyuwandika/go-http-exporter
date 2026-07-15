package slingexporter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/ronaldyuwandika/go-http-exporter"
)

func TestNewHTTPClient(t *testing.T) {
	client := NewHTTPClient(
		httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {}),
		nil,
	)

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Transport == nil {
		t.Fatal("expected transport to be set")
	}
}

func TestNewHTTPClientIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var mu sync.Mutex
	var captured *httpexporter.ResponseInfo

	client := NewHTTPClient(
		httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
			mu.Lock()
			captured = resp
			mu.Unlock()
		}),
		nil,
	)

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()
	if captured == nil {
		t.Fatal("exporter was not called")
	}
	if captured.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", captured.StatusCode)
	}
}
