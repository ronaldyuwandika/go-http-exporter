package httpexporter

import (
	"testing"
	"time"
)

func TestSlugNormalizerUUID(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"", ""},
		{"/", "/"},
		{"/api/v1", "/api/v1"},
		{"/users/550e8400-e29b-41d4-a716-446655440000", "/users/:uuid"},
		{"/users/550e8400-e29b-41d4-a716-446655440000/profile", "/users/:uuid/profile"},
		{"/orders/12345", "/orders/:id"},
		{"/items/0", "/items/:id"},
		{"/files/a1b2c3d4e5f67890abcdef1234567890", "/files/:hex"},
		{"/stats/2024-01-15", "/stats/:date"},
		{"/products/my-product-v2", "/products/:slug"},
		{"/api/v1/users/42/items/550e8400-e29b-41d4-a716-446655440000", "/api/v1/users/:id/items/:uuid"},
		{"/health", "/health"},
		{"/static/js/app.js", "/static/js/app.js"},
	}

	for _, tt := range tests {
		got := SlugNormalizer(tt.in)
		if got != tt.out {
			t.Errorf("SlugNormalizer(%q) = %q, want %q", tt.in, got, tt.out)
		}
	}
}

func TestPathNormalizerType(t *testing.T) {
	var _ PathNormalizer = SlugNormalizer

	identity := func(p string) string { return p }
	var _ PathNormalizer = identity
}

func TestCopyRequestInfoNormalizedPath(t *testing.T) {
	dnsDur := 5 * time.Millisecond
	original := &RequestInfo{
		Method:         "GET",
		Path:           "/users/42",
		NormalizedPath: "/users/:id",
		DNSDuration:    dnsDur,
	}
	cpy := CopyRequestInfo(original)
	if cpy.NormalizedPath != "/users/:id" {
		t.Fatalf("expected /users/:id, got %s", cpy.NormalizedPath)
	}
	if cpy.DNSDuration != dnsDur {
		t.Fatalf("expected %v, got %v", dnsDur, cpy.DNSDuration)
	}
}
