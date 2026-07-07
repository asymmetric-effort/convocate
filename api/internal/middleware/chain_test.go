package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChain(t *testing.T) {
	var order []string

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw1-after")
		})
	}

	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw2-after")
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	})

	chained := Chain(handler, mw1, mw2)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	chained.ServeHTTP(w, r)

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("order = %v, want %v", order, expected)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d] = %q, want %q", i, order[i], v)
		}
	}
}

func TestCORS(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := CORS(handler)

	// Test preflight
	w := httptest.NewRecorder()
	r := httptest.NewRequest("OPTIONS", "/", nil)
	corsHandler.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "http://localhost:8080" {
		t.Errorf("CORS origin = %q", origin)
	}

	// Test normal request
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/", nil)
	corsHandler.ServeHTTP(w2, r2)
	if w2.Code != http.StatusOK {
		t.Errorf("GET status = %d, want %d", w2.Code, http.StatusOK)
	}
}
