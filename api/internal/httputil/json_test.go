package httputil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}
	WriteJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var result map[string]string
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["key"] != "value" {
		t.Errorf("body key = %q, want %q", result["key"], "value")
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusBadRequest, "validation_failed", "bad input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var result Error
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.Code != "validation_failed" {
		t.Errorf("code = %q, want %q", result.Code, "validation_failed")
	}
	if result.Message != "bad input" {
		t.Errorf("message = %q, want %q", result.Message, "bad input")
	}
}

func TestReadJSON(t *testing.T) {
	body := bytes.NewBufferString(`{"name":"test"}`)
	r := httptest.NewRequest("POST", "/", body)

	var target struct {
		Name string `json:"name"`
	}
	err := ReadJSON(r, &target)
	if err != nil {
		t.Fatalf("ReadJSON failed: %v", err)
	}
	if target.Name != "test" {
		t.Errorf("Name = %q, want %q", target.Name, "test")
	}
}
