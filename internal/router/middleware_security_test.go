package router

import (
	"context"
	"net/http"
	"testing"
)

func TestHandlerSecurityHeaders(t *testing.T) {
	_, ts := testServer(t)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/v1/health", http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/health: %v", err)
	}
	defer resp.Body.Close()

	want := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Content-Security-Policy":   "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'",
		"X-XSS-Protection":          "0",
	}

	for header, wantVal := range want {
		gotVal := resp.Header.Get(header)
		if gotVal != wantVal {
			t.Errorf("header %q: got %q, want %q", header, gotVal, wantVal)
		}
	}
}
