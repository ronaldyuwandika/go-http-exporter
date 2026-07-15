package httptransport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/ronaldyuwandika/go-http-exporter"
)

func TestTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))
	defer server.Close()

	var mu sync.Mutex
	var captured *httpexporter.ResponseInfo

	tr := New(httpexporter.WithExporter(
		httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
			mu.Lock()
			captured = resp
			mu.Unlock()
		}),
	))

	client := &http.Client{Transport: tr}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	// Read body fully before close to test body tracking
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()
	if captured == nil {
		t.Fatal("exporter was not called")
	}
	if captured.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", captured.StatusCode)
	}
	if captured.BodySize != int64(len(body)) {
		t.Fatalf("expected body size %d, got %d", len(body), captured.BodySize)
	}
	if captured.Duration <= 0 {
		t.Fatal("expected positive duration")
	}
}

func TestTransportError(t *testing.T) {
	var captured error

	tr := New(httpexporter.WithExporter(
		httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
			captured = resp.Error
		}),
	))

	client := &http.Client{Transport: tr}
	_, err := client.Get("http://invalid-url-that-fails.test")
	if err != nil {
		// expected — error is also exported
	}
	if captured == nil {
		t.Fatal("expected error to be captured")
	}
}

func TestTransportFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exported := false
	tr := New(
		httpexporter.WithExporter(
			httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
				exported = true
			}),
		),
		httpexporter.WithShouldExport(func(r *http.Request) bool {
			return false
		}),
	)

	client := &http.Client{Transport: tr}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if exported {
		t.Fatal("exporter should not have been called when filtered")
	}
}

func TestTransportDefaultOptions(t *testing.T) {
	tr := New()
	if tr.base != http.DefaultTransport {
		t.Fatal("expected default transport")
	}
}

func TestTransportNilTransport(t *testing.T) {
	// Just ensure no panic
	_ = New(httpexporter.WithTransport(nil))
}

func TestTransportNilExporter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	tr := New(httpexporter.WithExporter(nil))
	client := &http.Client{Transport: tr}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func TestTransportBodyReadError(t *testing.T) {
	var mu sync.Mutex
	var capturedErr error

	tr := New(httpexporter.WithExporter(
		httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
			mu.Lock()
			capturedErr = resp.Error
			mu.Unlock()
		}),
	))

	client := &http.Client{Transport: tr}
	resp, err := client.Get("http://invalid-url-that-fails.test")
	if err != nil {
		// expected
	}
	_ = resp

	mu.Lock()
	defer mu.Unlock()
	if capturedErr == nil {
		t.Fatal("expected error to be captured")
	}
}

// Ensure Transport implements RoundTripper
var _ http.RoundTripper = (*Transport)(nil)

func TestNewRequestInfo(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com/path", nil)
	req.Host = "example.com"
	info := httpexporter.NewRequestInfo(req)

	if info.Method != "GET" {
		t.Fatalf("expected GET, got %s", info.Method)
	}
	if info.Path != "/path" {
		t.Fatalf("expected /path, got %s", info.Path)
	}
	if info.Host != "example.com" {
		t.Fatalf("expected example.com, got %s", info.Host)
	}
}

func TestNewResponseInfo(t *testing.T) {
	resp := &http.Response{
		StatusCode:    404,
		Status:        "404 Not Found",
		ContentLength: 42,
	}

	info := httpexporter.NewResponseInfo(resp, 0, errors.New("boom"))
	if info.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", info.StatusCode)
	}
	if info.BodySize != 42 {
		t.Fatalf("expected 42, got %d", info.BodySize)
	}
	if info.Error == nil {
		t.Fatal("expected error to be captured")
	}
}

func TestNewResponseInfoNil(t *testing.T) {
	info := httpexporter.NewResponseInfo(nil, 0, errors.New("timeout"))
	if info.Error == nil {
		t.Fatal("expected error")
	}
}
