package openbao

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHealthUnhealthyStatus tests the Health method when the server
// returns a 500 status.
func TestHealthUnhealthyStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("not healthy"))
	}))
	defer server.Close()

	client := NewClient(Config{Address: server.URL})
	err := client.Health()
	if err == nil {
		t.Error("expected error for unhealthy server")
	}
}

// TestHealthStandbyStatus tests that standby (429) is considered healthy.
func TestHealthStandbyStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer server.Close()

	client := NewClient(Config{Address: server.URL})
	err := client.Health()
	if err != nil {
		t.Errorf("standby should be healthy, got error: %v", err)
	}
}

// TestDoRequestEmptyBody tests that empty body from a 200 response is handled.
func TestDoRequestEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write nothing.
	}))
	defer server.Close()

	client := NewClient(Config{Address: server.URL, Token: "t"})
	result, err := client.KVRead("secret", "empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for empty response, got %v", result)
	}
}

// TestDoRequestInvalidJSON tests that invalid JSON from a 200 response returns error.
func TestDoRequestInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient(Config{Address: server.URL, Token: "t"})
	_, err := client.KVRead("secret", "bad-json")
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

// TestDoRequestNetworkError tests network errors.
func TestDoRequestNetworkError(t *testing.T) {
	client := NewClient(Config{Address: "http://127.0.0.1:1", Token: "t"})
	_, err := client.KVRead("secret", "net-err")
	if err == nil {
		t.Error("expected error for network failure")
	}
}

// TestAppRoleLoginEmpty tests AppRoleLogin with empty response.
func TestAppRoleLoginEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Empty body.
	}))
	defer server.Close()

	client := NewClient(Config{Address: server.URL, Token: "t"})
	_, err := client.AppRoleLogin("approle", "r", "s")
	if err == nil {
		t.Error("expected error for empty AppRole login response")
	}
}

// TestInitStatusEmpty tests InitStatus with empty response.
func TestInitStatusEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Empty body.
	}))
	defer server.Close()

	client := NewClient(Config{Address: server.URL, Token: "t"})
	_, err := client.InitStatus()
	if err == nil {
		t.Error("expected error for empty init status response")
	}
}

// TestPolicyReadMissingDataField tests PolicyRead when response has no data field.
func TestPolicyReadMissingDataField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"other": "field",
		})
	}))
	defer server.Close()

	client := NewClient(Config{Address: server.URL, Token: "t"})
	result, err := client.PolicyRead("test-policy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

// TestDoRequestWithNoToken tests that requests work without a token.
func TestDoRequestWithNoToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "" {
			t.Error("expected no token header")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(Config{Address: server.URL})
	err := client.KVDelete("secret", "no-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestPolicyReadNetworkError tests PolicyRead when the server is unreachable.
func TestPolicyReadNetworkError(t *testing.T) {
	client := NewClient(Config{Address: "http://127.0.0.1:1", Token: "t"})
	_, err := client.PolicyRead("test-policy")
	if err == nil {
		t.Error("expected error for network failure")
	}
}

// TestAppRoleLoginNetworkError tests AppRoleLogin when the server is unreachable.
func TestAppRoleLoginNetworkError(t *testing.T) {
	client := NewClient(Config{Address: "http://127.0.0.1:1", Token: "t"})
	_, err := client.AppRoleLogin("approle", "r", "s")
	if err == nil {
		t.Error("expected error for network failure")
	}
}

// TestHealthNetworkError tests Health when the server is unreachable.
func TestHealthNetworkError(t *testing.T) {
	client := NewClient(Config{Address: "http://127.0.0.1:1", Token: "t"})
	err := client.Health()
	if err == nil {
		t.Error("expected error for network failure")
	}
}

// TestInitStatusNetworkError tests InitStatus when the server is unreachable.
func TestInitStatusNetworkError(t *testing.T) {
	client := NewClient(Config{Address: "http://127.0.0.1:1", Token: "t"})
	_, err := client.InitStatus()
	if err == nil {
		t.Error("expected error for network failure")
	}
}

// TestDoRequestInvalidURL tests doRequest with an invalid URL.
func TestDoRequestInvalidURL(t *testing.T) {
	client := NewClient(Config{Address: "://invalid", Token: "t"})
	err := client.KVWrite("secret", "path", map[string]interface{}{"k": "v"})
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// TestHealthInvalidURL tests Health with an invalid URL.
func TestHealthInvalidURL(t *testing.T) {
	client := NewClient(Config{Address: "://invalid", Token: "t"})
	err := client.Health()
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// TestInitStatusMissingInitialized tests InitStatus when the response has no initialized field.
func TestInitStatusMissingInitialized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"something_else": true,
		})
	}))
	defer server.Close()

	client := NewClient(Config{Address: server.URL, Token: "t"})
	_, err := client.InitStatus()
	if err == nil {
		t.Error("expected error for missing initialized field")
	}
}

// TestDoRequest400WithErrors tests that 400 errors are properly parsed.
func TestDoRequest400WithErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"invalid path", "missing field"},
		})
	}))
	defer server.Close()

	client := NewClient(Config{Address: server.URL, Token: "t"})
	err := client.KVWrite("secret", "bad", map[string]interface{}{"k": "v"})
	if err == nil {
		t.Error("expected error for 400 response")
	}
}
