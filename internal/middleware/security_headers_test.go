package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asymmetric-effort/convocate/internal/middleware"
)

func TestSecurityHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.SecurityHeaders(inner)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	want := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Content-Security-Policy":   "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'",
		"X-XSS-Protection":          "0",
	}

	for header, wantVal := range want {
		gotVal := rr.Header().Get(header)
		if gotVal != wantVal {
			t.Errorf("header %q: got %q, want %q", header, gotVal, wantVal)
		}
	}
}

func TestSecurityHeadersPassthrough(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	})

	handler := middleware.SecurityHeaders(inner)

	req := httptest.NewRequest(http.MethodGet, "/ping", http.NoBody)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("inner handler was not called")
	}
	if rr.Code != http.StatusTeapot {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusTeapot)
	}
}
