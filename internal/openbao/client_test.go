package openbao

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// mockBao is a minimal in-memory OpenBao API mock for unit testing.
type mockBao struct {
	mu       sync.Mutex
	kvStore  map[string]map[string]interface{} // path -> data
	policies map[string]string                 // name -> rules
	initialized bool
}

func newMockBao() *mockBao {
	return &mockBao{
		kvStore:     make(map[string]map[string]interface{}),
		policies:    make(map[string]string),
		initialized: true,
	}
}

func (m *mockBao) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := r.URL.Path

	// Health check.
	if path == "/v1/sys/health" {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"initialized": true, "sealed": false})
		return
	}

	// Init status.
	if path == "/v1/sys/init" {
		json.NewEncoder(w).Encode(map[string]interface{}{"initialized": m.initialized})
		return
	}

	// AppRole login.
	if strings.HasSuffix(path, "/login") && r.Method == "POST" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"auth": map[string]interface{}{
				"client_token": "mock-token-abc123",
			},
		})
		return
	}

	// KV v2 data operations.
	if strings.HasPrefix(path, "/v1/secret/data/") {
		kvPath := strings.TrimPrefix(path, "/v1/secret/data/")
		switch r.Method {
		case "GET":
			data, exists := m.kvStore[kvPath]
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"data": data,
				},
			})
			return
		case "POST":
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Data map[string]interface{} `json:"data"`
			}
			json.Unmarshal(body, &req)
			m.kvStore[kvPath] = req.Data
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{})
			return
		}
	}

	// KV v2 metadata operations (delete).
	if strings.HasPrefix(path, "/v1/secret/metadata/") {
		kvPath := strings.TrimPrefix(path, "/v1/secret/metadata/")
		if r.Method == "DELETE" {
			delete(m.kvStore, kvPath)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	// Policy operations.
	if strings.HasPrefix(path, "/v1/sys/policies/acl/") {
		policyName := strings.TrimPrefix(path, "/v1/sys/policies/acl/")
		switch r.Method {
		case "PUT":
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Policy string `json:"policy"`
			}
			json.Unmarshal(body, &req)
			m.policies[policyName] = req.Policy
			w.WriteHeader(http.StatusNoContent)
			return
		case "GET":
			rules, exists := m.policies[policyName]
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"rules": rules,
				},
			})
			return
		case "DELETE":
			delete(m.policies, policyName)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

func testClient(t *testing.T) (*Client, *mockBao) {
	t.Helper()
	mock := newMockBao()
	server := httptest.NewServer(mock)
	t.Cleanup(server.Close)
	client := NewClient(Config{
		Address: server.URL,
		Token:   "test-token",
	})
	return client, mock
}

func TestHealth(t *testing.T) {
	client, _ := testClient(t)
	err := client.Health()
	if err != nil {
		t.Fatalf("Health() error: %v", err)
	}
}

func TestHealthUnreachable(t *testing.T) {
	client := NewClient(Config{Address: "https://127.0.0.1:1"})
	err := client.Health()
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestInitStatus(t *testing.T) {
	client, _ := testClient(t)
	initialized, err := client.InitStatus()
	if err != nil {
		t.Fatalf("InitStatus() error: %v", err)
	}
	if !initialized {
		t.Error("expected initialized=true")
	}
}

func TestKVWriteRead(t *testing.T) {
	client, _ := testClient(t)

	data := map[string]interface{}{
		"ssh_key": "private-key-data",
		"pat":     "ghp_abc123",
	}
	err := client.KVWrite("secret", "test/path", data)
	if err != nil {
		t.Fatalf("KVWrite error: %v", err)
	}

	result, err := client.KVRead("secret", "test/path")
	if err != nil {
		t.Fatalf("KVRead error: %v", err)
	}
	if result == nil {
		t.Fatal("KVRead returned nil")
	}
	if result["ssh_key"] != "private-key-data" {
		t.Errorf("ssh_key: got %q, want %q", result["ssh_key"], "private-key-data")
	}
	if result["pat"] != "ghp_abc123" {
		t.Errorf("pat: got %q, want %q", result["pat"], "ghp_abc123")
	}
}

func TestKVReadNotFound(t *testing.T) {
	client, _ := testClient(t)
	result, err := client.KVRead("secret", "nonexistent")
	if err != nil {
		t.Fatalf("KVRead error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestKVDelete(t *testing.T) {
	client, _ := testClient(t)

	err := client.KVWrite("secret", "to-delete", map[string]interface{}{"key": "val"})
	if err != nil {
		t.Fatalf("KVWrite error: %v", err)
	}

	err = client.KVDelete("secret", "to-delete")
	if err != nil {
		t.Fatalf("KVDelete error: %v", err)
	}

	result, err := client.KVRead("secret", "to-delete")
	if err != nil {
		t.Fatalf("KVRead error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil after delete, got %v", result)
	}
}

func TestPolicyWriteRead(t *testing.T) {
	client, _ := testClient(t)

	rules := `path "secret/data/test/*" { capabilities = ["read"] }`
	err := client.PolicyWrite("test-policy", rules)
	if err != nil {
		t.Fatalf("PolicyWrite error: %v", err)
	}

	got, err := client.PolicyRead("test-policy")
	if err != nil {
		t.Fatalf("PolicyRead error: %v", err)
	}
	if got != rules {
		t.Errorf("PolicyRead: got %q, want %q", got, rules)
	}
}

func TestPolicyReadNotFound(t *testing.T) {
	client, _ := testClient(t)
	result, err := client.PolicyRead("nonexistent")
	if err != nil {
		t.Fatalf("PolicyRead error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestPolicyDelete(t *testing.T) {
	client, _ := testClient(t)

	err := client.PolicyWrite("to-delete", `path "x" { capabilities = ["read"] }`)
	if err != nil {
		t.Fatalf("PolicyWrite error: %v", err)
	}

	err = client.PolicyDelete("to-delete")
	if err != nil {
		t.Fatalf("PolicyDelete error: %v", err)
	}

	result, err := client.PolicyRead("to-delete")
	if err != nil {
		t.Fatalf("PolicyRead error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty after delete, got %q", result)
	}
}

func TestAppRoleLogin(t *testing.T) {
	client, _ := testClient(t)

	token, err := client.AppRoleLogin("approle", "role-id-123", "secret-id-456")
	if err != nil {
		t.Fatalf("AppRoleLogin error: %v", err)
	}
	if token != "mock-token-abc123" {
		t.Errorf("token: got %q, want %q", token, "mock-token-abc123")
	}
}

func TestSetToken(t *testing.T) {
	client, _ := testClient(t)
	client.SetToken("new-token")
	// Verify it doesn't panic and operations still work.
	err := client.Health()
	if err != nil {
		t.Fatalf("Health after SetToken error: %v", err)
	}
}

func TestOpenBaoError(t *testing.T) {
	err := &OpenBaoError{
		StatusCode: 403,
		Errors:     []string{"permission denied"},
	}
	got := err.Error()
	if !strings.Contains(got, "403") {
		t.Errorf("error should contain status code: %q", got)
	}
	if !strings.Contains(got, "permission denied") {
		t.Errorf("error should contain message: %q", got)
	}
}

func TestOpenBaoErrorType(t *testing.T) {
	// Test that server returning 403 produces an *OpenBaoError.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"permission denied"},
		})
	}))
	defer server.Close()

	client := NewClient(Config{Address: server.URL, Token: "bad-token"})
	_, err := client.KVRead("secret", "test")
	if err == nil {
		t.Fatal("expected error for 403")
	}

	var baoErr *OpenBaoError
	if !errors.As(err, &baoErr) {
		t.Fatalf("expected *OpenBaoError, got %T: %v", err, err)
	}
	if baoErr.StatusCode != 403 {
		t.Errorf("StatusCode: got %d, want 403", baoErr.StatusCode)
	}
}
