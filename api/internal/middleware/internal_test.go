package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInternalAuth_NoKeyConfigured(t *testing.T) {
	// metricsAPIKey is loaded at init from env; set it to empty for dev mode
	origKey := metricsAPIKey
	metricsAPIKey = ""
	defer func() { metricsAPIKey = origKey }()

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := InternalAuth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !called {
		t.Error("handler was not called in dev mode")
	}
}

func TestInternalAuth_ValidKey(t *testing.T) {
	origKey := metricsAPIKey
	metricsAPIKey = "secret-metrics-key"
	defer func() { metricsAPIKey = origKey }()

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := InternalAuth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "Bearer secret-metrics-key")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !called {
		t.Error("handler was not called with valid key")
	}
}

func TestInternalAuth_InvalidKey(t *testing.T) {
	origKey := metricsAPIKey
	metricsAPIKey = "secret-metrics-key"
	defer func() { metricsAPIKey = origKey }()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := InternalAuth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "Bearer wrong-key")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestInternalAuth_MissingAuth(t *testing.T) {
	origKey := metricsAPIKey
	metricsAPIKey = "secret-metrics-key"
	defer func() { metricsAPIKey = origKey }()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := InternalAuth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestInternalAuth_BasicAuth(t *testing.T) {
	origKey := metricsAPIKey
	metricsAPIKey = "secret-metrics-key"
	defer func() { metricsAPIKey = origKey }()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := InternalAuth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
