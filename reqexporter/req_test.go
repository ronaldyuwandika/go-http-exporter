package reqexporter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ronaldyuwandika/go-http-exporter"
)

func TestInstall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var mu sync.Mutex
	var captured *httpexporter.ResponseInfo

	client := NewClient(
		httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
			mu.Lock()
			captured = resp
			mu.Unlock()
		}),
	)

	resp, err := client.R().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if captured == nil {
		t.Fatal("exporter was not called")
	}
	if captured.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", captured.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("response should still be valid after export")
	}
}

func TestInstallRequestInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	var mu sync.Mutex
	var capturedReq *httpexporter.RequestInfo

	client := NewClient(
		httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
			mu.Lock()
			capturedReq = req
			mu.Unlock()
		}),
	)

	_, err := client.R().
		SetHeader("X-Custom", "val").
		Get(server.URL + "/api/v1/users")
	if err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if capturedReq == nil {
		t.Fatal("request info was not captured")
	}
	if capturedReq.Method != "GET" {
		t.Fatalf("expected GET, got %s", capturedReq.Method)
	}
	if capturedReq.Path != "/api/v1/users" {
		t.Fatalf("expected /api/v1/users, got %s", capturedReq.Path)
	}
	if capturedReq.Host == "" {
		t.Fatal("expected non-empty host")
	}
}

func TestInstallDuration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var capturedDur time.Duration
	client := NewClient(
		httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
			capturedDur = resp.Duration
		}),
	)

	_, err := client.R().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	if capturedDur < 10*time.Millisecond {
		t.Fatalf("duration too short: %v", capturedDur)
	}
}

func TestInstallNilExporter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(nil)
	resp, err := client.R().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("expected 200 with nil exporter")
	}
}
