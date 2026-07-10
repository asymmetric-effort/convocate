package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/asymmetric-effort/convocate/internal/httputil"
)

func TestAuth_MockTokenRejected(t *testing.T) {
	// Mock auth is no longer supported — any token must validate via OpenBao.
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := Auth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "Bearer mock-token")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuth_AnyInvalidTokenRejected(t *testing.T) {
	// All tokens must validate via OpenBao — no special-case tokens.
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := Auth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "Bearer any-invalid-token")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuth_MissingToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := Auth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var errResp httputil.Error
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Code != "unauthorized" {
		t.Errorf("error code = %q, want %q", errResp.Code, "unauthorized")
	}
}

func TestAuth_EmptyBearerToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := Auth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "Bearer ")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuth_InvalidAuthScheme(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := Auth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuth_QueryParamToken(t *testing.T) {
	// Mock OpenBao server that accepts valid-query-token
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/token/lookup-self":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"entity_id": "ent-qp",
					"policies":  []string{"admin-policy"},
				},
			})
		case "/v1/identity/entity/id/ent-qp":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":        "ent-qp",
					"name":      "queryuser",
					"metadata":  map[string]string{"name": "Query User", "email": "qp@example.com"},
					"policies":  []string{"default"},
					"group_ids": []string{},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, ok := httputil.PrincipalFromContext(r.Context())
		if !ok {
			t.Error("expected principal in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := Auth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test?token=valid-query-token", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !called {
		t.Error("handler was not called")
	}
}

func TestAuth_ValidOpenBaoToken(t *testing.T) {
	// Mock OpenBao server
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/token/lookup-self":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"entity_id": "ent-123",
					"policies":  []string{"admin-policy"},
				},
			})
		case "/v1/identity/entity/id/ent-123":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":       "ent-123",
					"name":     "testuser",
					"metadata": map[string]string{"name": "Test User", "email": "test@example.com"},
					"policies": []string{"default"},
					"group_ids": []string{"grp-1"},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	var got *httputil.Principal
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := httputil.PrincipalFromContext(r.Context())
		if !ok {
			t.Error("expected principal in context")
			return
		}
		got = p
		w.WriteHeader(http.StatusOK)
	})

	handler := Auth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "Bearer valid-token-123")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got == nil {
		t.Fatal("principal was nil")
	}
	if got.ID != "ent-123" {
		t.Errorf("ID = %q, want %q", got.ID, "ent-123")
	}
	if got.Username != "testuser" {
		t.Errorf("Username = %q, want %q", got.Username, "testuser")
	}
	if got.Name != "Test User" {
		t.Errorf("Name = %q, want %q", got.Name, "Test User")
	}
	if got.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "test@example.com")
	}
	if got.IDP != "openbao" {
		t.Errorf("IDP = %q, want %q", got.IDP, "openbao")
	}
}

func TestAuth_InvalidToken_OpenBaoRejects(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := Auth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "Bearer bad-token")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuth_TokenWithNoEntityID(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"entity_id": "",
				"policies":  []string{"default"},
			},
		})
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := Auth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "Bearer some-token")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var errResp httputil.Error
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Message != "token has no associated entity" {
		t.Errorf("message = %q, want %q", errResp.Message, "token has no associated entity")
	}
}

func TestAuth_EntityLookupFails(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/token/lookup-self":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"entity_id": "ent-fail",
					"policies":  []string{"default"},
				},
			})
		case "/v1/identity/entity/id/ent-fail":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	handler := Auth(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("Authorization", "Bearer some-token")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRolesToApplets_Admin(t *testing.T) {
	result := rolesToApplets([]string{"admin-policy"})
	if len(result) != 7 {
		t.Errorf("len = %d, want 7 (all applets)", len(result))
	}
}

func TestRolesToApplets_AdminLowercase(t *testing.T) {
	result := rolesToApplets([]string{"Admin"})
	if len(result) != 7 {
		t.Errorf("len = %d, want 7 (all applets)", len(result))
	}
}

func TestRolesToApplets_SpecificPolicies(t *testing.T) {
	result := rolesToApplets([]string{"node-read", "agent-write"})
	appletMap := make(map[string]bool)
	for _, a := range result {
		appletMap[a] = true
	}
	if !appletMap["nmgr"] {
		t.Error("expected nmgr for node-read policy")
	}
	if !appletMap["amgr"] {
		t.Error("expected amgr for agent-write policy")
	}
	if len(result) != 2 {
		t.Errorf("len = %d, want 2", len(result))
	}
}

func TestRolesToApplets_AllPolicyTypes(t *testing.T) {
	policies := []string{
		"node-read",
		"agent-write",
		"pb-execute",
		"ide-access",
		"access-manage",
		"repo-view",
		"support-ticket",
	}
	result := rolesToApplets(policies)
	if len(result) != 7 {
		t.Errorf("len = %d, want 7", len(result))
	}
}

func TestRolesToApplets_EmptyPolicies(t *testing.T) {
	result := rolesToApplets([]string{})
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

func TestRolesToApplets_UnknownPolicies(t *testing.T) {
	result := rolesToApplets([]string{"default", "something-else"})
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

func TestBuildPrincipal(t *testing.T) {
	entity := &entityResponse{}
	entity.Data.ID = "ent-001"
	entity.Data.Name = "bob"
	entity.Data.Metadata = map[string]string{
		"name":  "Bob Builder",
		"email": "bob@example.com",
	}
	entity.Data.Policies = []string{"default"}
	entity.Data.GroupIDs = []string{"grp-1", "grp-2"}

	p := buildPrincipal(entity, []string{"node-read", "default"})

	if p.ID != "ent-001" {
		t.Errorf("ID = %q, want %q", p.ID, "ent-001")
	}
	if p.Username != "bob" {
		t.Errorf("Username = %q, want %q", p.Username, "bob")
	}
	if p.Name != "Bob Builder" {
		t.Errorf("Name = %q, want %q", p.Name, "Bob Builder")
	}
	if p.Email != "bob@example.com" {
		t.Errorf("Email = %q, want %q", p.Email, "bob@example.com")
	}
	if p.IDP != "openbao" {
		t.Errorf("IDP = %q, want %q", p.IDP, "openbao")
	}
	if len(p.Groups) != 2 {
		t.Errorf("Groups len = %d, want 2", len(p.Groups))
	}
	// default + node-read (deduplicated)
	if len(p.Roles) != 2 {
		t.Errorf("Roles len = %d, want 2", len(p.Roles))
	}
}

func TestBuildPrincipal_FallbackName(t *testing.T) {
	entity := &entityResponse{}
	entity.Data.ID = "ent-002"
	entity.Data.Name = "fallback-name"
	entity.Data.Metadata = map[string]string{} // no "name" in metadata

	p := buildPrincipal(entity, []string{})

	if p.Name != "fallback-name" {
		t.Errorf("Name = %q, want %q (entity name fallback)", p.Name, "fallback-name")
	}
}

func TestBuildPrincipal_DeduplicatesPolicies(t *testing.T) {
	entity := &entityResponse{}
	entity.Data.ID = "ent-003"
	entity.Data.Name = "user"
	entity.Data.Metadata = map[string]string{}
	entity.Data.Policies = []string{"default", "admin"}

	p := buildPrincipal(entity, []string{"default", "admin", "node-read"})

	// default, admin, node-read (deduplicated)
	if len(p.Roles) != 3 {
		t.Errorf("Roles len = %d, want 3 (deduplicated)", len(p.Roles))
	}
}

func TestBuildPrincipal_EmptyPolicySkipped(t *testing.T) {
	entity := &entityResponse{}
	entity.Data.ID = "ent-004"
	entity.Data.Name = "user"
	entity.Data.Metadata = map[string]string{}
	entity.Data.Policies = []string{"", "default", ""}

	p := buildPrincipal(entity, []string{"", "node-read"})

	// only "default" and "node-read" - empties skipped
	if len(p.Roles) != 2 {
		t.Errorf("Roles len = %d, want 2 (empty strings skipped)", len(p.Roles))
	}
}

func TestOpenbaoAddr_Default(t *testing.T) {
	os.Unsetenv("OPENBAO_ADDR")
	addr := openbaoAddr()
	if addr != "http://openbao.security.svc:8200" {
		t.Errorf("addr = %q, want default", addr)
	}
}

func TestOpenbaoAddr_EnvOverride(t *testing.T) {
	os.Setenv("OPENBAO_ADDR", "http://localhost:8200/")
	defer os.Unsetenv("OPENBAO_ADDR")
	addr := openbaoAddr()
	if addr != "http://localhost:8200" {
		t.Errorf("addr = %q, want trailing slash stripped", addr)
	}
}

func TestLookupTokenSelf_Success(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "test-token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"entity_id": "ent-100",
				"policies":  []string{"default", "admin"},
			},
		})
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	resp, err := lookupTokenSelf("test-token")
	if err != nil {
		t.Fatalf("lookupTokenSelf failed: %v", err)
	}
	if resp.Data.EntityID != "ent-100" {
		t.Errorf("EntityID = %q, want %q", resp.Data.EntityID, "ent-100")
	}
	if len(resp.Data.Policies) != 2 {
		t.Errorf("Policies len = %d, want 2", len(resp.Data.Policies))
	}
}

func TestLookupTokenSelf_Failure(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := lookupTokenSelf("bad-token")
	if err == nil {
		t.Fatal("expected error for failed token lookup")
	}
}

func TestLookupEntity_Success(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":        "ent-200",
				"name":      "testentity",
				"metadata":  map[string]string{"name": "Test"},
				"policies":  []string{"default"},
				"group_ids": []string{"grp-a"},
			},
		})
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	resp, err := lookupEntity("token", "ent-200")
	if err != nil {
		t.Fatalf("lookupEntity failed: %v", err)
	}
	if resp.Data.ID != "ent-200" {
		t.Errorf("ID = %q, want %q", resp.Data.ID, "ent-200")
	}
}

func TestLookupEntity_Failure(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := lookupEntity("token", "nonexistent")
	if err == nil {
		t.Fatal("expected error for failed entity lookup")
	}
}

func TestLookupTokenSelf_InvalidJSON(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := lookupTokenSelf("token")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLookupEntity_InvalidJSON(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := lookupEntity("token", "ent-001")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLookupTokenSelf_ConnectionRefused(t *testing.T) {
	os.Setenv("OPENBAO_ADDR", "http://127.0.0.1:1")
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := lookupTokenSelf("token")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestLookupEntity_ConnectionRefused(t *testing.T) {
	os.Setenv("OPENBAO_ADDR", "http://127.0.0.1:1")
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := lookupEntity("token", "ent-001")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestAuth_QueryParamFallback_NoHeader(t *testing.T) {
	// Mock OpenBao server that accepts the token
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/token/lookup-self":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"entity_id": "ent-fb",
					"policies":  []string{"admin-policy"},
				},
			})
		case "/v1/identity/entity/id/ent-fb":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":        "ent-fb",
					"name":      "fallbackuser",
					"metadata":  map[string]string{"name": "Fallback User", "email": "fb@example.com"},
					"policies":  []string{"default"},
					"group_ids": []string{},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := Auth(inner)
	w := httptest.NewRecorder()
	// No Authorization header, only query param
	r := httptest.NewRequest("GET", "/test?token=valid-fallback-token", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !called {
		t.Error("handler was not called with query param token")
	}
}
