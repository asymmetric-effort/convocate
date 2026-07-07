package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewAuth_NoKey(t *testing.T) {
	a := NewAuth("", "")
	if a.publicKey != nil {
		t.Error("publicKey should be nil when no key path provided")
	}
	if a.expectedSAToken != "" {
		t.Error("expectedSAToken should be empty when no path provided")
	}
}

func TestNewAuth_InvalidPath(t *testing.T) {
	a := NewAuth("/nonexistent/path", "")
	if a.publicKey != nil {
		t.Error("publicKey should be nil for missing file")
	}
}

func TestAuth_VerifyToken_DevMode(t *testing.T) {
	a := NewAuth("", "") // dev mode — no key
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
	a := NewAuth("", "")
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
	a := NewAuth("", "") // dev mode
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

// ---------------------------------------------------------------------------
// K8s SA token tests
// ---------------------------------------------------------------------------

func TestAuth_VerifySAToken_Disabled(t *testing.T) {
	a := NewAuth("", "") // no SA token configured
	req := httptest.NewRequest("GET", "/", nil)
	if !a.VerifySAToken(req) {
		t.Error("VerifySAToken should pass when SA auth is disabled")
	}
}

func TestAuth_VerifySAToken_Valid(t *testing.T) {
	// Write a test SA token file
	dir := t.TempDir()
	tokenPath := dir + "/token"
	os.WriteFile(tokenPath, []byte("test-sa-token-xyz"), 0644)

	a := NewAuth("", tokenPath)
	if a.expectedSAToken != "test-sa-token-xyz" {
		t.Errorf("expectedSAToken = %q, want %q", a.expectedSAToken, "test-sa-token-xyz")
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-K8s-SA-Token", "test-sa-token-xyz")
	if !a.VerifySAToken(req) {
		t.Error("VerifySAToken should pass with correct token")
	}
}

func TestAuth_VerifySAToken_Invalid(t *testing.T) {
	dir := t.TempDir()
	tokenPath := dir + "/token"
	os.WriteFile(tokenPath, []byte("correct-token"), 0644)

	a := NewAuth("", tokenPath)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-K8s-SA-Token", "wrong-token")
	if a.VerifySAToken(req) {
		t.Error("VerifySAToken should fail with wrong token")
	}
}

func TestAuth_VerifySAToken_Missing(t *testing.T) {
	dir := t.TempDir()
	tokenPath := dir + "/token"
	os.WriteFile(tokenPath, []byte("correct-token"), 0644)

	a := NewAuth("", tokenPath)

	req := httptest.NewRequest("GET", "/", nil)
	// No X-K8s-SA-Token header
	if a.VerifySAToken(req) {
		t.Error("VerifySAToken should fail with missing token")
	}
}

func TestAuth_RequireRole_SATokenRejection(t *testing.T) {
	dir := t.TempDir()
	tokenPath := dir + "/token"
	os.WriteFile(tokenPath, []byte("correct-token"), 0644)

	a := NewAuth("", tokenPath)
	handler := a.RequireRole("agent-view", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Valid JWT but missing SA token
	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	// No X-K8s-SA-Token
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (SA token missing)", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuth_RequireRole_BothTokensValid(t *testing.T) {
	dir := t.TempDir()
	tokenPath := dir + "/token"
	os.WriteFile(tokenPath, []byte("correct-sa-token"), 0644)

	a := NewAuth("", tokenPath)
	called := false
	handler := a.RequireRole("agent-view", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	req.Header.Set("X-K8s-SA-Token", "correct-sa-token")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !called {
		t.Error("handler should have been called with both tokens valid")
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
