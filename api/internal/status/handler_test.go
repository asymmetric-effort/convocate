package status

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleStatus(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body platformStatus
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != "healthy" {
		t.Errorf("expected status healthy, got %q", body.Status)
	}
	if body.Version != "2.0.0-dev" {
		t.Errorf("expected version 2.0.0-dev, got %q", body.Version)
	}
	if len(body.Services) != 4 {
		t.Errorf("expected 4 services, got %d", len(body.Services))
	}
	if body.Uptime == "" {
		t.Error("uptime should not be empty")
	}
	if body.Timestamp == "" {
		t.Error("timestamp should not be empty")
	}
}
