package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockProcess creates a server with no real process (for handler tests)
func setupTestServer() (*Server, *http.ServeMux) {
	m := NewMetrics()
	m.StdinBytes.Add(42)
	m.StdoutBytes.Add(100)

	// Process with nil cmd — handlers check IsRunning
	proc := &Process{
		metrics: m,
		done:    make(chan struct{}),
	}

	auth := NewAuth("", "") // dev mode
	srv := NewServer(proc, m, auth, "test-v1", "claude-v2", "test-pod", "test-node")

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	return srv, mux
}

func TestHandleHealthz(t *testing.T) {
	_, mux := setupTestServer()

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status = %q, want %q", body["status"], "ok")
	}
}

func TestHandleReadyz_NotRunning(t *testing.T) {
	_, mux := setupTestServer()

	req := httptest.NewRequest("GET", "/readyz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Process has nil cmd → not running → 503
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleMetrics(t *testing.T) {
	_, mux := setupTestServer()

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var snap MetricsSnapshot
	if err := json.NewDecoder(rec.Body).Decode(&snap); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}

	if snap.WrapperVersion != "test-v1" {
		t.Errorf("WrapperVersion = %q, want %q", snap.WrapperVersion, "test-v1")
	}
	if snap.ClaudeCodeVersion != "claude-v2" {
		t.Errorf("ClaudeCodeVersion = %q, want %q", snap.ClaudeCodeVersion, "claude-v2")
	}
	if snap.StdinBytes != 42 {
		t.Errorf("StdinBytes = %d, want 42", snap.StdinBytes)
	}
	if snap.PodName != "test-pod" {
		t.Errorf("PodName = %q, want %q", snap.PodName, "test-pod")
	}
}

func TestHandleMetrics_NoAuth(t *testing.T) {
	_, mux := setupTestServer()

	req := httptest.NewRequest("GET", "/metrics", nil)
	// No Authorization header
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleStdin_NoAuth(t *testing.T) {
	_, mux := setupTestServer()

	req := httptest.NewRequest("POST", "/stdin", strings.NewReader("hello"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleStdin_NoPipe(t *testing.T) {
	_, mux := setupTestServer()

	req := httptest.NewRequest("POST", "/stdin", strings.NewReader("hello"))
	req.Header.Set("Authorization", "Bearer mock-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Process has nil stdin pipe → 500
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleRestart_NoAuth(t *testing.T) {
	_, mux := setupTestServer()

	req := httptest.NewRequest("POST", "/control/restart", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleSignal_InvalidBody(t *testing.T) {
	_, mux := setupTestServer()

	req := httptest.NewRequest("POST", "/control/signal", strings.NewReader("not json"))
	req.Header.Set("Authorization", "Bearer mock-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleSignal_UnknownSignal(t *testing.T) {
	_, mux := setupTestServer()

	req := httptest.NewRequest("POST", "/control/signal", strings.NewReader(`{"signal":"SIGFOO"}`))
	req.Header.Set("Authorization", "Bearer mock-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestParseSignal(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"SIGTERM", true},
		{"TERM", true},
		{"SIGINT", true},
		{"INT", true},
		{"SIGKILL", true},
		{"SIGUSR1", true},
		{"SIGUSR2", true},
		{"SIGHUP", true},
		{"SIGFOO", false},
		{"", false},
	}
	for _, tt := range tests {
		_, ok := parseSignal(tt.input)
		if ok != tt.ok {
			t.Errorf("parseSignal(%q) ok = %v, want %v", tt.input, ok, tt.ok)
		}
	}
}

func TestHandleStdout_NotWebSocket(t *testing.T) {
	_, mux := setupTestServer()

	req := httptest.NewRequest("GET", "/stdout", nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Should return 426 Upgrade Required since it's not a WebSocket request
	if rec.Code != http.StatusUpgradeRequired {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUpgradeRequired)
	}
}

func TestHandleStderr_NotWebSocket(t *testing.T) {
	_, mux := setupTestServer()

	req := httptest.NewRequest("GET", "/stderr", nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUpgradeRequired {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUpgradeRequired)
	}
}

func TestRouteRegistration(t *testing.T) {
	_, mux := setupTestServer()

	// Test that all expected routes are registered by sending requests
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/healthz"},
		{"GET", "/readyz"},
		{"GET", "/metrics"},
		{"POST", "/stdin"},
		{"GET", "/stdout"},
		{"GET", "/stderr"},
		{"POST", "/control/restart"},
		{"POST", "/control/signal"},
	}

	for _, r := range routes {
		req := httptest.NewRequest(r.method, r.path, nil)
		if r.method == "POST" {
			req.Body = io.NopCloser(strings.NewReader("{}"))
		}
		req.Header.Set("Authorization", "Bearer mock-token")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		// Should not be 404 (route not found)
		if rec.Code == http.StatusNotFound {
			t.Errorf("%s %s returned 404 — route not registered", r.method, r.path)
		}
	}
}

func TestEnvOr(t *testing.T) {
	if got := envOr("NONEXISTENT_VAR_12345", "fallback"); got != "fallback" {
		t.Errorf("envOr = %q, want %q", got, "fallback")
	}
}

func TestFileExists_Missing(t *testing.T) {
	if fileExists("/nonexistent/path/123") {
		t.Error("fileExists should return false for missing file")
	}
}

// Ensure Metrics tracks time correctly
func TestMetrics_Uptime(t *testing.T) {
	m := NewMetrics()
	time.Sleep(10 * time.Millisecond)
	snap := m.Snapshot("v", "v", "p", "n", 0)
	if snap.UptimeSeconds < 0 {
		t.Errorf("UptimeSeconds = %d, should be >= 0", snap.UptimeSeconds)
	}
}
