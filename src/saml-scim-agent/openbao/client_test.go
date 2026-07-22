package openbao

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("https://vault.example.com:8200/", "s.token123", false)
	if client.Addr != "https://vault.example.com:8200" {
		t.Errorf("expected trailing slash stripped, got %s", client.Addr)
	}
	if client.Token != "s.token123" {
		t.Errorf("expected token s.token123, got %s", client.Token)
	}
	if client.HTTP == nil {
		t.Fatal("HTTP client is nil")
	}
}

func TestNewClientSkipTLS(t *testing.T) {
	client := NewClient("https://vault.example.com:8200", "token", true)
	transport := client.HTTP.Transport.(*http.Transport)
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify to be true")
	}

	client2 := NewClient("https://vault.example.com:8200", "token", false)
	transport2 := client2.HTTP.Transport.(*http.Transport)
	if transport2.TLSClientConfig.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify to be false")
	}
}

func TestCheckHealth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sys/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Vault-Token") != "test-token" {
			t.Errorf("missing or wrong token header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"initialized":true,"sealed":false}`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.CheckHealth()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckHealthUnhealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.CheckHealth()
	if err == nil {
		t.Fatal("expected error for unhealthy vault")
	}
}

func TestCheckHealthConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	err := client.CheckHealth()
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
}

func TestAuthenticate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/userpass/login/testuser" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		// Verify body
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]string
		json.Unmarshal(body, &parsed)
		if parsed["password"] != "password123" {
			t.Errorf("expected password password123, got %s", parsed["password"])
		}

		resp := map[string]interface{}{
			"auth": map[string]interface{}{
				"client_token": "s.abc123",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	token, err := client.Authenticate("testuser", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "s.abc123" {
		t.Errorf("expected token s.abc123, got %s", token)
	}
}

func TestAuthenticateFail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.Authenticate("baduser", "badpass")
	if err == nil {
		t.Fatal("expected error for bad credentials")
	}
}

func TestAuthenticateInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not json`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.Authenticate("user", "pass")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestAuthenticateConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	_, err := client.Authenticate("user", "pass")
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
}

func TestKVReadWrite(t *testing.T) {
	store := make(map[string]interface{})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body struct {
				Data map[string]interface{} `json:"data"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			store = body.Data
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			if len(store) == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"data": store,
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)

	// Read non-existent
	data, err := client.KVRead("secret/data/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Fatal("expected nil for non-existent key")
	}

	// Write
	err = client.KVWrite("secret/data/test", map[string]interface{}{
		"key": "value",
	})
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	// Read back
	data, err = client.KVRead("secret/data/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if data["key"] != "value" {
		t.Errorf("expected 'value', got %v", data["key"])
	}
}

func TestKVReadInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not json`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.KVRead("secret/data/test")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestKVReadServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.KVRead("secret/data/test")
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestKVReadConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	_, err := client.KVRead("secret/data/test")
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
}

func TestKVWriteServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.KVWrite("secret/data/test", map[string]interface{}{"k": "v"})
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestKVWriteConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	err := client.KVWrite("secret/data/test", map[string]interface{}{"k": "v"})
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
}

func TestKVWrite204(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.KVWrite("secret/data/test", map[string]interface{}{"k": "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetEntityByName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/identity/entity/name/testuser" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "entity-123",
				"name": "testuser",
				"metadata": map[string]string{
					"email": "test@example.com",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	entity, err := client.GetEntityByName("testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity == nil {
		t.Fatal("expected entity, got nil")
	}
	if entity.ID != "entity-123" {
		t.Errorf("expected id entity-123, got %s", entity.ID)
	}
	if entity.Metadata["email"] != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", entity.Metadata["email"])
	}
}

func TestGetEntityByNameNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	entity, err := client.GetEntityByName("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity != nil {
		t.Fatal("expected nil for non-existent entity")
	}
}

func TestGetEntityByNameServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.GetEntityByName("user")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestGetEntityByNameInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`bad json`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.GetEntityByName("user")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGetEntityByNameConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	_, err := client.GetEntityByName("user")
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
}

func TestGetEntityByID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/identity/entity/id/entity-123" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "entity-123",
				"name": "testuser",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	entity, err := client.GetEntityByID("entity-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity == nil {
		t.Fatal("expected entity")
	}
	if entity.ID != "entity-123" {
		t.Errorf("expected id entity-123, got %s", entity.ID)
	}
}

func TestGetEntityByIDNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	entity, err := client.GetEntityByID("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity != nil {
		t.Fatal("expected nil")
	}
}

func TestGetEntityByIDServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.GetEntityByID("id")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestGetEntityByIDInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`bad`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.GetEntityByID("id")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGetEntityByIDConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	_, err := client.GetEntityByID("id")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateEntity(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/identity/entity" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Verify body
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "newuser" {
			t.Errorf("expected name newuser, got %v", body["name"])
		}

		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "new-entity-id",
				"name": "newuser",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	entity, err := client.CreateEntity("newuser", map[string]string{"email": "new@example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity.ID != "new-entity-id" {
		t.Errorf("expected id new-entity-id, got %s", entity.ID)
	}
}

func TestCreateEntityServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.CreateEntity("user", nil)
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestCreateEntityInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`bad json`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.CreateEntity("user", nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCreateEntityConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	_, err := client.CreateEntity("user", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateEntity(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/v1/identity/entity/name/testuser" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.UpdateEntity("testuser", map[string]string{"email": "new@example.com"}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateEntityServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.UpdateEntity("user", nil, false)
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestUpdateEntityConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	err := client.UpdateEntity("user", nil, false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteEntity(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/v1/identity/entity/name/testuser" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.DeleteEntity("testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteEntityServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.DeleteEntity("user")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestDeleteEntityConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	err := client.DeleteEntity("user")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteEntity200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.DeleteEntity("user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateUserpass(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/auth/userpass/users/testuser" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.CreateUserpass("testuser", "secret123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateUserpassServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.CreateUserpass("user", "pass")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestCreateUserpassConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	err := client.CreateUserpass("user", "pass")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteUserpass(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/v1/auth/userpass/users/testuser" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.DeleteUserpass("testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteUserpassServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.DeleteUserpass("user")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestDeleteUserpassConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	err := client.DeleteUserpass("user")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListEntities(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "LIST" {
			t.Errorf("expected LIST method, got %s", r.Method)
		}
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"keys": []string{"user1", "user2"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	names, err := client.ListEntities()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(names))
	}
	if names[0] != "user1" || names[1] != "user2" {
		t.Errorf("unexpected entity names: %v", names)
	}
}

func TestListEntitiesNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	names, err := client.ListEntities()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Errorf("expected nil, got %v", names)
	}
}

func TestListEntitiesServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.ListEntities()
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestListEntitiesInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`bad json`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.ListEntities()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestListEntitiesConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	_, err := client.ListEntities()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetGroupByName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/identity/group/name/admins" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"id":                "group-123",
				"name":             "admins",
				"member_entity_ids": []string{"entity-1", "entity-2"},
				"type":             "internal",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	group, err := client.GetGroupByName("admins")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group == nil {
		t.Fatal("expected group")
	}
	if group.ID != "group-123" {
		t.Errorf("expected id group-123, got %s", group.ID)
	}
	if group.Name != "admins" {
		t.Errorf("expected name admins, got %s", group.Name)
	}
}

func TestGetGroupByNameNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	group, err := client.GetGroupByName("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group != nil {
		t.Fatal("expected nil")
	}
}

func TestGetGroupByNameServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.GetGroupByName("group")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestGetGroupByNameInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`bad`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.GetGroupByName("group")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGetGroupByNameConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	_, err := client.GetGroupByName("group")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetGroupByID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/identity/group/id/group-123" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "group-123",
				"name": "admins",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	group, err := client.GetGroupByID("group-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group == nil {
		t.Fatal("expected group")
	}
	if group.ID != "group-123" {
		t.Errorf("expected id group-123, got %s", group.ID)
	}
}

func TestGetGroupByIDNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	group, err := client.GetGroupByID("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group != nil {
		t.Fatal("expected nil")
	}
}

func TestGetGroupByIDServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.GetGroupByID("id")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestGetGroupByIDInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`bad`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.GetGroupByID("id")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGetGroupByIDConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	_, err := client.GetGroupByID("id")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateGroup(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "newgroup" {
			t.Errorf("expected name newgroup, got %v", body["name"])
		}
		if body["type"] != "internal" {
			t.Errorf("expected type internal, got %v", body["type"])
		}

		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "new-group-id",
				"name": "newgroup",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	group, err := client.CreateGroup("newgroup", []string{"entity-1"}, map[string]string{"desc": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group.ID != "new-group-id" {
		t.Errorf("expected id new-group-id, got %s", group.ID)
	}
}

func TestCreateGroupServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.CreateGroup("group", nil, nil)
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestCreateGroupInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`bad`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.CreateGroup("group", nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCreateGroupConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	_, err := client.CreateGroup("group", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateGroup(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.UpdateGroup("admins", []string{"entity-1", "entity-2"}, map[string]string{"desc": "updated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateGroupServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.UpdateGroup("group", nil, nil)
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestUpdateGroupConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	err := client.UpdateGroup("group", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateGroup204(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.UpdateGroup("group", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteGroup(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/v1/identity/group/id/group-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.DeleteGroup("group-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteGroupServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.DeleteGroup("id")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestDeleteGroupConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	err := client.DeleteGroup("id")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteGroup200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	err := client.DeleteGroup("id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListGroups(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "LIST" {
			t.Errorf("expected LIST method, got %s", r.Method)
		}
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"keys": []string{"admins", "users"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	names, err := client.ListGroups()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(names))
	}
}

func TestListGroupsNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	names, err := client.ListGroups()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if names != nil {
		t.Errorf("expected nil, got %v", names)
	}
}

func TestListGroupsServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.ListGroups()
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestListGroupsInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`bad`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, err := client.ListGroups()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestListGroupsConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token", true)
	_, err := client.ListGroups()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetEntityByNameWithToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "user-token-123" {
			t.Errorf("expected user-token-123, got %s", r.Header.Get("X-Vault-Token"))
		}
		if r.URL.Path != "/v1/identity/entity/name/testuser" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "entity-456",
				"name": "testuser",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "admin-token", true)
	entity, err := client.GetEntityByNameWithToken("testuser", "user-token-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity == nil {
		t.Fatal("expected entity")
	}
	if entity.ID != "entity-456" {
		t.Errorf("expected entity-456, got %s", entity.ID)
	}
}

func TestGetEntityByNameWithTokenNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "admin-token", true)
	entity, err := client.GetEntityByNameWithToken("nonexistent", "user-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity != nil {
		t.Fatal("expected nil")
	}
}

func TestGetEntityByNameWithTokenServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "admin-token", true)
	_, err := client.GetEntityByNameWithToken("user", "token")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestGetEntityByNameWithTokenInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`bad`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "admin-token", true)
	_, err := client.GetEntityByNameWithToken("user", "token")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGetEntityByNameWithTokenConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "admin-token", true)
	_, err := client.GetEntityByNameWithToken("user", "token")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRequestWithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]string
		json.Unmarshal(body, &parsed)
		if parsed["key"] != "value" {
			t.Errorf("expected key=value, got %v", parsed)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, status, err := client.request(http.MethodPost, "/v1/test", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("expected status 200, got %d", status)
	}
}

func TestRequestWithoutBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "" {
			t.Errorf("expected no Content-Type for nil body, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-token", true)
	_, status, err := client.request(http.MethodGet, "/v1/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("expected status 200, got %d", status)
	}
}

func TestRequestWithTokenBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "custom-token" {
			t.Errorf("expected custom-token, got %s", r.Header.Get("X-Vault-Token"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "default-token", true)
	_, status, err := client.requestWithToken(http.MethodPost, "/v1/test", "custom-token", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("expected 200, got %d", status)
	}
}

func TestRequestMarshalError(t *testing.T) {
	client := NewClient("http://localhost:1", "test-token", true)
	// Channels can't be marshaled to JSON
	badBody := map[string]interface{}{"ch": make(chan int)}
	_, _, err := client.request(http.MethodPost, "/v1/test", badBody)
	if err == nil {
		t.Fatal("expected error for unmarshalable body")
	}
}

func TestRequestBadURL(t *testing.T) {
	client := NewClient("http://[invalid", "test-token", true)
	_, _, err := client.request(http.MethodGet, "/v1/test", nil)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestRequestWithTokenBadURL(t *testing.T) {
	client := NewClient("http://[invalid", "test-token", true)
	_, _, err := client.requestWithToken(http.MethodGet, "/v1/test", "token", nil)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestRequestWithTokenMarshalError(t *testing.T) {
	client := NewClient("http://localhost:1", "test-token", true)
	badBody := map[string]interface{}{"ch": make(chan int)}
	_, _, err := client.requestWithToken(http.MethodPost, "/v1/test", "token", badBody)
	if err == nil {
		t.Fatal("expected error for unmarshalable body")
	}
}

func TestRequestWithTokenNoBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "" {
			t.Errorf("expected no Content-Type, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "default-token", true)
	_, status, err := client.requestWithToken(http.MethodGet, "/v1/test", "custom-token", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("expected 200, got %d", status)
	}
}
