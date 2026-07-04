package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asymmetric-effort/convocate/internal/httputil"
)

func TestRBAC_AdminRolePasses(t *testing.T) {
	principal := &httputil.Principal{
		ID:       "usr-001",
		Username: "admin",
		Roles:    []string{"admin"},
	}

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := RBAC("node-manager")
	handler := mw(inner)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !called {
		t.Error("handler was not called for admin role")
	}
}

func TestRBAC_CorrectRolePasses(t *testing.T) {
	principal := &httputil.Principal{
		ID:       "usr-002",
		Username: "operator",
		Roles:    []string{"node-manager", "viewer"},
	}

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := RBAC("node-manager")
	handler := mw(inner)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !called {
		t.Error("handler was not called for correct role")
	}
}

func TestRBAC_WrongRoleReturns403(t *testing.T) {
	principal := &httputil.Principal{
		ID:       "usr-003",
		Username: "viewer",
		Roles:    []string{"viewer"},
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	mw := RBAC("node-manager")
	handler := mw(inner)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}

	var errResp httputil.Error
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Code != "forbidden" {
		t.Errorf("error code = %q, want %q", errResp.Code, "forbidden")
	}
}

func TestRBAC_NoPrincipalReturns403(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	mw := RBAC("node-manager")
	handler := mw(inner)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	// No principal in context
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}

	var errResp httputil.Error
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Message != "no principal in context" {
		t.Errorf("message = %q, want %q", errResp.Message, "no principal in context")
	}
}

func TestRBAC_NilPrincipalReturns403(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	mw := RBAC("node-manager")
	handler := mw(inner)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), nil)
	r = r.WithContext(ctx)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRBAC_EmptyRolesReturns403(t *testing.T) {
	principal := &httputil.Principal{
		ID:       "usr-004",
		Username: "noroles",
		Roles:    []string{},
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	mw := RBAC("node-manager")
	handler := mw(inner)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRBAC_MultipleRolesWithMatch(t *testing.T) {
	principal := &httputil.Principal{
		ID:       "usr-005",
		Username: "multi",
		Roles:    []string{"viewer", "editor", "node-manager"},
	}

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := RBAC("node-manager")
	handler := mw(inner)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !called {
		t.Error("handler was not called")
	}
}
