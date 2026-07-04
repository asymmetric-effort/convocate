package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
)

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

func TestOpenbaoLogin_Success(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/auth/userpass/login/testuser" {
			t.Errorf("path = %s, want /v1/auth/userpass/login/testuser", r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["password"] != "secret" {
			t.Errorf("password = %q, want %q", body["password"], "secret")
		}

		json.NewEncoder(w).Encode(map[string]any{
			"auth": map[string]any{
				"client_token":   "tok-abc",
				"entity_id":      "ent-001",
				"policies":       []string{"default", "node-read"},
				"metadata":       map[string]string{"name": "Test User"},
				"lease_duration": 3600,
			},
		})
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	resp, err := openbaoLogin("testuser", "secret")
	if err != nil {
		t.Fatalf("openbaoLogin failed: %v", err)
	}
	if resp.Auth.ClientToken != "tok-abc" {
		t.Errorf("ClientToken = %q, want %q", resp.Auth.ClientToken, "tok-abc")
	}
	if resp.Auth.EntityID != "ent-001" {
		t.Errorf("EntityID = %q, want %q", resp.Auth.EntityID, "ent-001")
	}
	if resp.Auth.LeaseDuration != 3600 {
		t.Errorf("LeaseDuration = %d, want 3600", resp.Auth.LeaseDuration)
	}
}

func TestOpenbaoLogin_BadCredentials(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoLogin("baduser", "badpass")
	if err == nil {
		t.Fatal("expected error for bad credentials")
	}
}

func TestOpenbaoLogin_Forbidden(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoLogin("user", "pass")
	if err == nil {
		t.Fatal("expected error for forbidden")
	}
}

func TestOpenbaoLogin_BadRequest(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoLogin("user", "pass")
	if err == nil {
		t.Fatal("expected error for bad request")
	}
}

func TestOpenbaoLogin_ServerError(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoLogin("user", "pass")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestOpenbaoLookupEntity_Success(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "my-token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.URL.Path != "/v1/identity/entity/id/ent-001" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":        "ent-001",
				"name":      "testuser",
				"metadata":  map[string]string{"name": "Test", "email": "test@x.com"},
				"policies":  []string{"default"},
				"group_ids": []string{"grp-1"},
			},
		})
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	resp, err := openbaoLookupEntity("my-token", "ent-001")
	if err != nil {
		t.Fatalf("openbaoLookupEntity failed: %v", err)
	}
	if resp.Data.ID != "ent-001" {
		t.Errorf("ID = %q, want %q", resp.Data.ID, "ent-001")
	}
	if resp.Data.Name != "testuser" {
		t.Errorf("Name = %q, want %q", resp.Data.Name, "testuser")
	}
	if resp.Data.Metadata["email"] != "test@x.com" {
		t.Errorf("email = %q, want %q", resp.Data.Metadata["email"], "test@x.com")
	}
}

func TestOpenbaoLookupEntity_Failure(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoLookupEntity("token", "nonexistent")
	if err == nil {
		t.Fatal("expected error for entity not found")
	}
}

func TestOpenbaoTokenLookupSelf_Success(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "good-token" {
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

	resp, err := openbaoTokenLookupSelf("good-token")
	if err != nil {
		t.Fatalf("openbaoTokenLookupSelf failed: %v", err)
	}
	if resp.Data.EntityID != "ent-100" {
		t.Errorf("EntityID = %q, want %q", resp.Data.EntityID, "ent-100")
	}
	if len(resp.Data.Policies) != 2 {
		t.Errorf("Policies len = %d, want 2", len(resp.Data.Policies))
	}
}

func TestOpenbaoTokenLookupSelf_Failure(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoTokenLookupSelf("bad-token")
	if err == nil {
		t.Fatal("expected error for bad token lookup")
	}
}

func TestOpenbaoRevokeSelf_Success(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/auth/token/revoke-self" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	err := openbaoRevokeSelf("token-to-revoke")
	if err != nil {
		t.Fatalf("openbaoRevokeSelf failed: %v", err)
	}
}

func TestOpenbaoRevokeSelf_Failure(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	err := openbaoRevokeSelf("token")
	if err == nil {
		t.Fatal("expected error for failed revoke")
	}
}

func TestRolesToApplets_Admin(t *testing.T) {
	result := rolesToApplets([]string{"admin-policy"})
	if len(result) != 7 {
		t.Errorf("len = %d, want 7", len(result))
	}
}

func TestRolesToApplets_AdminLowerMixed(t *testing.T) {
	result := rolesToApplets([]string{"Admin"})
	if len(result) != 7 {
		t.Errorf("len = %d, want 7 for Admin (case-insensitive)", len(result))
	}
}

func TestRolesToApplets_NodePolicy(t *testing.T) {
	result := rolesToApplets([]string{"node-read"})
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1", len(result))
	}
	if result[0] != "nmgr" {
		t.Errorf("result[0] = %q, want nmgr", result[0])
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
	sort.Strings(result)
	expected := []string{"ac", "amgr", "ide", "nmgr", "pb", "repo", "sup"}
	if len(result) != len(expected) {
		t.Fatalf("len = %d, want %d", len(result), len(expected))
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("result[%d] = %q, want %q", i, result[i], v)
		}
	}
}

func TestRolesToApplets_Empty(t *testing.T) {
	result := rolesToApplets([]string{})
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

func TestRolesToApplets_UnknownPolicy(t *testing.T) {
	result := rolesToApplets([]string{"default", "unknown-policy"})
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

func TestRolesToApplets_DuplicatePolicies(t *testing.T) {
	result := rolesToApplets([]string{"node-read", "node-write"})
	// Both match "node-" so only one "nmgr" entry
	if len(result) != 1 {
		t.Errorf("len = %d, want 1 (deduped)", len(result))
	}
}

func TestBuildPrincipalFromEntity(t *testing.T) {
	entity := &openbaoEntityResponse{}
	entity.Data.ID = "ent-001"
	entity.Data.Name = "bob"
	entity.Data.Metadata = map[string]string{
		"name":  "Bob Builder",
		"email": "bob@example.com",
	}
	entity.Data.Policies = []string{"default"}
	entity.Data.GroupIDs = []string{"grp-1"}

	p := buildPrincipalFromEntity(entity, []string{"node-read", "default"})

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
	if len(p.Groups) != 1 {
		t.Errorf("Groups len = %d, want 1", len(p.Groups))
	}
	// default + node-read (deduplicated)
	if len(p.Roles) != 2 {
		t.Errorf("Roles = %v, want 2 entries", p.Roles)
	}
}

func TestBuildPrincipalFromEntity_FallbackName(t *testing.T) {
	entity := &openbaoEntityResponse{}
	entity.Data.ID = "ent-002"
	entity.Data.Name = "fallback"
	entity.Data.Metadata = map[string]string{} // no "name"

	p := buildPrincipalFromEntity(entity, []string{})

	if p.Name != "fallback" {
		t.Errorf("Name = %q, want %q (entity name fallback)", p.Name, "fallback")
	}
}

func TestBuildPrincipalFromEntity_DeduplicatesPolicies(t *testing.T) {
	entity := &openbaoEntityResponse{}
	entity.Data.ID = "ent-003"
	entity.Data.Name = "user"
	entity.Data.Metadata = map[string]string{}
	entity.Data.Policies = []string{"default", "admin"}

	p := buildPrincipalFromEntity(entity, []string{"default", "admin", "node-read"})

	// default, admin, node-read (deduplicated)
	if len(p.Roles) != 3 {
		t.Errorf("Roles len = %d, want 3", len(p.Roles))
	}
}

func TestOpenbaoLogin_InvalidJSON(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoLogin("user", "pass")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestOpenbaoLookupEntity_InvalidJSON(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoLookupEntity("token", "ent-001")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestOpenbaoTokenLookupSelf_InvalidJSON(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoTokenLookupSelf("token")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestOpenbaoRevokeSelf_OK200(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	err := openbaoRevokeSelf("token")
	if err != nil {
		t.Fatalf("expected no error for 200 OK: %v", err)
	}
}

func TestOpenbaoLogin_ConnectionRefused(t *testing.T) {
	os.Setenv("OPENBAO_ADDR", "http://127.0.0.1:1") // port 1 should refuse
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoLogin("user", "pass")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestOpenbaoLookupEntity_ConnectionRefused(t *testing.T) {
	os.Setenv("OPENBAO_ADDR", "http://127.0.0.1:1")
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoLookupEntity("token", "ent-001")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestOpenbaoTokenLookupSelf_ConnectionRefused(t *testing.T) {
	os.Setenv("OPENBAO_ADDR", "http://127.0.0.1:1")
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoTokenLookupSelf("token")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestOpenbaoRevokeSelf_ConnectionRefused(t *testing.T) {
	os.Setenv("OPENBAO_ADDR", "http://127.0.0.1:1")
	defer os.Unsetenv("OPENBAO_ADDR")

	err := openbaoRevokeSelf("token")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestOpenbaoMFAValidate_Success(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/sys/mfa/validate" {
			t.Errorf("path = %s, want /v1/sys/mfa/validate", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["mfa_request_id"] != "req-123" {
			t.Errorf("mfa_request_id = %v, want req-123", body["mfa_request_id"])
		}
		payload, _ := body["mfa_payload"].(map[string]any)
		codes, _ := payload["method-abc"].([]any)
		if len(codes) != 1 || codes[0] != "654321" {
			t.Errorf("mfa_payload unexpected: %v", payload)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"auth": map[string]any{
				"client_token":   "tok-mfa-ok",
				"entity_id":      "ent-mfa",
				"policies":       []string{"default", "admin"},
				"metadata":       map[string]string{"name": "MFA User"},
				"lease_duration": 7200,
			},
		})
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	resp, err := openbaoMFAValidate("req-123", "method-abc", "654321")
	if err != nil {
		t.Fatalf("openbaoMFAValidate failed: %v", err)
	}
	if resp.Auth.ClientToken != "tok-mfa-ok" {
		t.Errorf("ClientToken = %q, want %q", resp.Auth.ClientToken, "tok-mfa-ok")
	}
	if resp.Auth.EntityID != "ent-mfa" {
		t.Errorf("EntityID = %q, want %q", resp.Auth.EntityID, "ent-mfa")
	}
}

func TestOpenbaoMFAValidate_InvalidCode(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoMFAValidate("req-123", "method-abc", "000000")
	if err == nil {
		t.Fatal("expected error for invalid MFA code")
	}
}

func TestOpenbaoMFAValidate_ServerError(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoMFAValidate("req-123", "method-abc", "123456")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestOpenbaoMFAValidate_InvalidJSON(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoMFAValidate("req-123", "method-abc", "123456")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestOpenbaoMFAValidate_ConnectionRefused(t *testing.T) {
	os.Setenv("OPENBAO_ADDR", "http://127.0.0.1:1")
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoMFAValidate("req-123", "method-abc", "123456")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestOpenbaoMFAMethodID(t *testing.T) {
	os.Setenv("OPENBAO_MFA_METHOD_ID", "test-id-123")
	defer os.Unsetenv("OPENBAO_MFA_METHOD_ID")

	id := openbaoMFAMethodID()
	if id != "test-id-123" {
		t.Errorf("openbaoMFAMethodID() = %q, want %q", id, "test-id-123")
	}
}

func TestOpenbaoMFAMethodID_Empty(t *testing.T) {
	os.Unsetenv("OPENBAO_MFA_METHOD_ID")

	id := openbaoMFAMethodID()
	if id != "" {
		t.Errorf("openbaoMFAMethodID() = %q, want empty", id)
	}
}

func TestBuildPrincipalFromEntity_SkipsEmptyPolicies(t *testing.T) {
	entity := &openbaoEntityResponse{}
	entity.Data.ID = "ent-004"
	entity.Data.Name = "user"
	entity.Data.Metadata = map[string]string{}
	entity.Data.Policies = []string{"", "default"}

	p := buildPrincipalFromEntity(entity, []string{"", "node-read"})

	if len(p.Roles) != 2 {
		t.Errorf("Roles len = %d, want 2 (empty skipped)", len(p.Roles))
	}
}
