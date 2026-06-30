package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewAuth_NoKey(t *testing.T) {
	a := NewAuth("")
	if a.publicKey != nil {
		t.Error("publicKey should be nil when no key path provided")
	}
}

func TestNewAuth_InvalidPath(t *testing.T) {
	a := NewAuth("/nonexistent/path")
	if a.publicKey != nil {
		t.Error("publicKey should be nil for missing file")
	}
}

func TestAuth_VerifyToken_DevMode(t *testing.T) {
	a := NewAuth("") // dev mode — no key
	claims, err := a.VerifyToken("mock-token")
	if err != nil {
		t.Fatalf("VerifyToken in dev mode: %v", err)
	}
	if claims.Sub != "mock" {
		t.Errorf("Sub = %q, want %q", claims.Sub, "mock")
	}
	if !claims.HasRole("admin") {
		t.Error("dev mode should grant admin role")
	}
}

func TestJWTClaims_HasRole(t *testing.T) {
	tests := []struct {
		roles []string
		check string
		want  bool
	}{
		{[]string{"agent-view"}, "agent-view", true},
		{[]string{"agent-view"}, "agent-update", false},
		{[]string{"admin"}, "agent-view", true},
		{[]string{"admin"}, "anything", true},
		{[]string{}, "agent-view", false},
		{[]string{"node-view", "agent-update"}, "agent-update", true},
	}
	for _, tt := range tests {
		c := &JWTClaims{Roles: tt.roles}
		got := c.HasRole(tt.check)
		if got != tt.want {
			t.Errorf("HasRole(%q) with roles %v = %v, want %v", tt.check, tt.roles, got, tt.want)
		}
	}
}

func TestAuth_RequireRole_MissingToken(t *testing.T) {
	a := NewAuth("")
	// Override to require real tokens by setting a non-nil key
	// In dev mode, all tokens pass. Test the missing-token path.
	handler := a.RequireRole("agent-view", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/metrics", nil)
	// No Authorization header
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuth_RequireRole_ValidToken_DevMode(t *testing.T) {
	a := NewAuth("") // dev mode
	called := false
	handler := a.RequireRole("agent-view", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !called {
		t.Error("handler should have been called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestExtractBearerToken_Header(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer my-token-123")
	got := extractBearerToken(req)
	if got != "my-token-123" {
		t.Errorf("got %q, want %q", got, "my-token-123")
	}
}

func TestExtractBearerToken_QueryParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/?token=query-token", nil)
	got := extractBearerToken(req)
	if got != "query-token" {
		t.Errorf("got %q, want %q", got, "query-token")
	}
}

func TestExtractBearerToken_Empty(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	got := extractBearerToken(req)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
