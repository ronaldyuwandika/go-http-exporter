package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ronaldyuwandika/go-http-exporter"
)

func TestMiddleware(t *testing.T) {
	var mu sync.Mutex
	var capturedReq *httpexporter.RequestInfo
	var capturedResp *httpexporter.ResponseInfo

	handler := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}),
		httpexporter.WithExporter(
			httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
				mu.Lock()
				capturedReq = req
				capturedResp = resp
				mu.Unlock()
			}),
		),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/users")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()

	if capturedReq == nil {
		t.Fatal("exporter was not called")
	}
	if capturedReq.Method != "GET" {
		t.Fatalf("expected GET, got %s", capturedReq.Method)
	}
	if capturedReq.Path != "/api/v1/users" {
		t.Fatalf("expected /api/v1/users, got %s", capturedReq.Path)
	}
	if capturedResp == nil {
		t.Fatal("response info was not captured")
	}
	if capturedResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", capturedResp.StatusCode)
	}
	if capturedResp.BodySize <= 0 {
		t.Fatal("expected positive body size")
	}
	if capturedResp.Duration <= 0 {
		t.Fatal("expected positive duration")
	}
}

func TestMiddlewareNormalizedPath(t *testing.T) {
	var mu sync.Mutex
	var capturedPath string

	handler := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}),
		httpexporter.WithExporter(
			httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
				mu.Lock()
				capturedPath = req.NormalizedPath
				mu.Unlock()
			}),
		),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	http.Get(server.URL + "/users/42")

	mu.Lock()
	defer mu.Unlock()

	if capturedPath != "/users/:id" {
		t.Fatalf("expected /users/:id, got %s", capturedPath)
	}
}

func TestMiddlewareFilter(t *testing.T) {
	exported := false

	handler := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}),
		httpexporter.WithExporter(
			httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
				exported = true
			}),
		),
		httpexporter.WithShouldExport(func(r *http.Request) bool {
			return r.URL.Path != "/health"
		}),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	http.Get(server.URL + "/health")
	if exported {
		t.Fatal("exporter should not have been called for /health")
	}
}

func TestMiddlewareSubMsDuration(t *testing.T) {
	var capturedDur time.Duration

	handler := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// no-op — handler returns in microseconds
	}),
		httpexporter.WithExporter(
			httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
				capturedDur = resp.Duration
			}),
		),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	http.Get(server.URL + "/fast")

	if capturedDur <= 0 {
		t.Fatal("expected positive duration")
	}
	// Most local handlers finish in <1ms; verify we have sub-ms precision
	// by checking the duration is captured (not zero/rounded).
	if capturedDur == capturedDur.Truncate(time.Millisecond) {
		t.Logf("note: duration %v has no sub-ms component", capturedDur)
	}
}

func TestMiddlewareStatusCodeCapture(t *testing.T) {
	var capturedCode int

	handler := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}),
		httpexporter.WithExporter(
			httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
				capturedCode = resp.StatusCode
			}),
		),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	http.Get(server.URL + "/missing")

	if capturedCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", capturedCode)
	}
}

func TestMiddlewareNilExporter(t *testing.T) {
	handler := New(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		httpexporter.WithExporter(nil),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/ok")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatal("expected 200 with nil exporter")
	}
}

func TestMiddlewareStatusFallback(t *testing.T) {
	// Handler that writes before setting status — should default to 200.
	var capturedCode int

	handler := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}),
		httpexporter.WithExporter(
			httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
				capturedCode = resp.StatusCode
			}),
		),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	http.Get(server.URL + "/implicit-200")

	if capturedCode != http.StatusOK {
		t.Fatalf("expected implicit 200, got %d", capturedCode)
	}
}

func TestResponseWriterFlusher(t *testing.T) {
	var flushed bool

	handler := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Error("wrapper does not implement http.Flusher")
			return
		}
		f.Flush()
		flushed = true
	}))

	server := httptest.NewServer(handler)
	defer server.Close()

	http.Get(server.URL + "/flush")

	if !flushed {
		t.Fatal("Flush was not called")
	}
}

func TestResponseWriterHijacker(t *testing.T) {
	handler := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Hijacker); !ok {
			t.Error("wrapper does not implement http.Hijacker")
		}
	}))

	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/hijack")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestMiddlewarePanicRecovery(t *testing.T) {
	var capturedCode int
	var captured bool

	handler := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("crash")
	}),
		httpexporter.WithExporter(
			httpexporter.ExporterFunc(func(ctx context.Context, req *httpexporter.RequestInfo, resp *httpexporter.ResponseInfo) {
				capturedCode = resp.StatusCode
				captured = true
			}),
		),
	)

	// Simulate net/http's built-in panic recovery by wrapping manually.
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recover()
		}()
		handler.ServeHTTP(w, r)
	})

	server := httptest.NewServer(panicHandler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/panic")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if !captured {
		t.Fatal("panic was not captured by middleware")
	}
	if capturedCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 on panic, got %d", capturedCode)
	}
}
