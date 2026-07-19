package scim

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/asymmetric-effort/convocate/src/gatekeeper/openbao"
)

func mockBaoServer(t *testing.T) (*httptest.Server, *openbao.Client) {
	t.Helper()
	mux := http.NewServeMux()

	// Entities
	mux.HandleFunc("/v1/identity/entity/name", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "LIST" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"keys": []string{"alice", "bob"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/v1/identity/entity/name/alice", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"id":        "entity-1",
					"name":      "alice",
					"metadata":  map[string]string{"email": "alice@example.com", "display_name": "Alice Smith", "given_name": "Alice", "family_name": "Smith"},
					"group_ids": []string{"group-1"},
					"disabled":  false,
				},
			})
		case http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("/v1/identity/entity/name/bob", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":       "entity-2",
				"name":     "bob",
				"metadata": map[string]string{"email": "bob@example.com"},
				"disabled": false,
			},
		})
	})
	mux.HandleFunc("/v1/identity/entity/id/entity-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":        "entity-1",
				"name":      "alice",
				"metadata":  map[string]string{"email": "alice@example.com", "display_name": "Alice Smith", "given_name": "Alice", "family_name": "Smith"},
				"group_ids": []string{"group-1"},
				"disabled":  false,
			},
		})
	})
	mux.HandleFunc("/v1/identity/entity/id/entity-2", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":       "entity-2",
				"name":     "bob",
				"metadata": map[string]string{"email": "bob@example.com"},
				"disabled": false,
			},
		})
	})
	mux.HandleFunc("/v1/identity/entity/id/nonexistent", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/v1/identity/entity", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"id":   "new-entity",
					"name": "newuser",
				},
			})
		}
	})

	// Userpass
	mux.HandleFunc("/v1/auth/userpass/users/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Groups
	mux.HandleFunc("/v1/identity/group/name", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "LIST" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"keys": []string{"admins", "users"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/v1/identity/group/name/admins", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":                "group-1",
				"name":             "admins",
				"member_entity_ids": []string{"entity-1"},
				"type":             "internal",
			},
		})
	})
	mux.HandleFunc("/v1/identity/group/name/users", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":                "group-2",
				"name":             "users",
				"member_entity_ids": []string{"entity-1", "entity-2"},
				"type":             "internal",
			},
		})
	})
	mux.HandleFunc("/v1/identity/group/id/group-1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"id":                "group-1",
					"name":             "admins",
					"member_entity_ids": []string{"entity-1"},
					"type":             "internal",
				},
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("/v1/identity/group/id/group-2", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":                "group-2",
				"name":             "users",
				"member_entity_ids": []string{"entity-1", "entity-2"},
				"type":             "internal",
			},
		})
	})
	mux.HandleFunc("/v1/identity/group/id/nonexistent", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/v1/identity/group", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"id":                "new-group",
					"name":             "newgroup",
					"member_entity_ids": []string{},
					"type":             "internal",
				},
			})
		}
	})

	ts := httptest.NewServer(mux)
	client := openbao.NewClient(ts.URL, "test-token", true)
	return ts, client
}

func doRequest(h *Handler, method, path string, body string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	var req *http.Request
	if reader != nil {
		req = httptest.NewRequest(method, path, reader)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer test-token")
	if body != "" {
		req.Header.Set("Content-Type", "application/scim+json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// -- Auth Tests --

func TestHandlerNoAuth(t *testing.T) {
	h := &Handler{
		Client:  openbao.NewClient("http://localhost:1", "token", true),
		BaseURL: "https://example.com",
	}

	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandlerEmptyToken(t *testing.T) {
	h := &Handler{
		Client:  openbao.NewClient("http://localhost:1", "token", true),
		BaseURL: "https://example.com",
	}

	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandlerInvalidAuthScheme(t *testing.T) {
	h := &Handler{
		Client:  openbao.NewClient("http://localhost:1", "token", true),
		BaseURL: "https://example.com",
	}

	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// -- Service Provider Config --

func TestHandlerServiceProviderConfig(t *testing.T) {
	h := &Handler{
		Client:  openbao.NewClient("http://localhost:1", "token", true),
		BaseURL: "https://example.com",
	}

	w := doRequest(h, http.MethodGet, "/scim/v2/ServiceProviderConfig", "")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var cfg ServiceProviderConfig
	json.NewDecoder(w.Body).Decode(&cfg)
	if cfg.Schemas[0] != SchemaServiceProvider {
		t.Errorf("unexpected schema: %s", cfg.Schemas[0])
	}
}

func TestHandlerServiceProviderConfigMethodNotAllowed(t *testing.T) {
	h := &Handler{
		Client:  openbao.NewClient("http://localhost:1", "token", true),
		BaseURL: "https://example.com",
	}

	w := doRequest(h, http.MethodPost, "/scim/v2/ServiceProviderConfig", "")
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// -- Schemas --

func TestHandlerSchemas(t *testing.T) {
	h := &Handler{
		Client:  openbao.NewClient("http://localhost:1", "token", true),
		BaseURL: "https://example.com",
	}

	w := doRequest(h, http.MethodGet, "/scim/v2/Schemas", "")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlerSchemasMethodNotAllowed(t *testing.T) {
	h := &Handler{
		Client:  openbao.NewClient("http://localhost:1", "token", true),
		BaseURL: "https://example.com",
	}

	w := doRequest(h, http.MethodPost, "/scim/v2/Schemas", "")
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// -- Resource Types --

func TestHandlerResourceTypes(t *testing.T) {
	h := &Handler{
		Client:  openbao.NewClient("http://localhost:1", "token", true),
		BaseURL: "https://example.com",
	}

	w := doRequest(h, http.MethodGet, "/scim/v2/ResourceTypes", "")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlerResourceTypesMethodNotAllowed(t *testing.T) {
	h := &Handler{
		Client:  openbao.NewClient("http://localhost:1", "token", true),
		BaseURL: "https://example.com",
	}

	w := doRequest(h, http.MethodPut, "/scim/v2/ResourceTypes", "")
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// -- Users CRUD --

func TestListUsers(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodGet, "/scim/v2/Users", "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ListResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.TotalResults != 2 {
		t.Errorf("expected 2 results, got %d", resp.TotalResults)
	}
	if resp.Schemas[0] != SchemaListResponse {
		t.Errorf("unexpected schema: %s", resp.Schemas[0])
	}
}

func TestListUsersError(t *testing.T) {
	client := openbao.NewClient("http://127.0.0.1:1", "token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodGet, "/scim/v2/Users", "")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestListUsersMethodNotAllowed(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodDelete, "/scim/v2/Users", "")

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestGetUser(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodGet, "/scim/v2/Users/entity-1", "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var user User
	json.NewDecoder(w.Body).Decode(&user)
	if user.ID != "entity-1" {
		t.Errorf("expected id entity-1, got %s", user.ID)
	}
	if user.UserName != "alice" {
		t.Errorf("expected userName alice, got %s", user.UserName)
	}
	if user.DisplayName != "Alice Smith" {
		t.Errorf("expected displayName 'Alice Smith', got %s", user.DisplayName)
	}
	if user.Name == nil {
		t.Fatal("expected name")
	}
	if user.Name.GivenName != "Alice" {
		t.Errorf("expected givenName Alice, got %s", user.Name.GivenName)
	}
	if user.Name.FamilyName != "Smith" {
		t.Errorf("expected familyName Smith, got %s", user.Name.FamilyName)
	}
	if user.Name.Formatted != "Alice Smith" {
		t.Errorf("expected formatted 'Alice Smith', got %s", user.Name.Formatted)
	}
	if len(user.Emails) != 1 || user.Emails[0].Value != "alice@example.com" {
		t.Errorf("unexpected emails: %v", user.Emails)
	}
	if !user.Active {
		t.Error("expected active true")
	}
	if len(user.Groups) != 1 || user.Groups[0].Value != "group-1" {
		t.Errorf("unexpected groups: %v", user.Groups)
	}
	if user.Meta.ResourceType != "User" {
		t.Errorf("expected ResourceType User, got %s", user.Meta.ResourceType)
	}
}

func TestGetUserNotFound(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodGet, "/scim/v2/Users/nonexistent", "")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetUserError(t *testing.T) {
	client := openbao.NewClient("http://127.0.0.1:1", "token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodGet, "/scim/v2/Users/entity-1", "")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestGetUserMethodNotAllowed(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodPatch, "/scim/v2/Users/entity-1", "")

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestCreateUser(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"newuser","displayName":"New User","name":{"givenName":"New","familyName":"User"},"emails":[{"value":"new@example.com"}],"password":"secret123","active":true}`
	w := doRequest(h, http.MethodPost, "/scim/v2/Users", body)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var user User
	json.NewDecoder(w.Body).Decode(&user)
	if user.ID != "new-entity" {
		t.Errorf("expected id new-entity, got %s", user.ID)
	}
}

func TestCreateUserMissingUserName(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"displayName":"No Name"}`
	w := doRequest(h, http.MethodPost, "/scim/v2/Users", body)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateUserInvalidJSON(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodPost, "/scim/v2/Users", "not json")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateUserNoPassword(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"newuser","active":true}`
	w := doRequest(h, http.MethodPost, "/scim/v2/Users", body)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateUserConflict(t *testing.T) {
	// Server that returns error on create
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/identity/entity", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"newuser"}`
	w := doRequest(h, http.MethodPost, "/scim/v2/Users", body)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestUpdateUser(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"alice","displayName":"Alice Updated","name":{"givenName":"Alice","familyName":"Updated"},"emails":[{"value":"alice-new@example.com"}],"active":true}`
	w := doRequest(h, http.MethodPut, "/scim/v2/Users/entity-1", body)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var user User
	json.NewDecoder(w.Body).Decode(&user)
	if user.ID != "entity-1" {
		t.Errorf("expected id entity-1, got %s", user.ID)
	}
}

func TestUpdateUserNotFound(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"x","active":true}`
	w := doRequest(h, http.MethodPut, "/scim/v2/Users/nonexistent", body)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateUserInvalidJSON(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodPut, "/scim/v2/Users/entity-1", "not json")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateUserError(t *testing.T) {
	client := openbao.NewClient("http://127.0.0.1:1", "token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"x","active":true}`
	w := doRequest(h, http.MethodPut, "/scim/v2/Users/entity-1", body)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestUpdateUserUpdateFails(t *testing.T) {
	// Server where get succeeds but update fails
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/identity/entity/id/entity-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "entity-1",
				"name": "alice",
			},
		})
	})
	mux.HandleFunc("/v1/identity/entity/name/alice", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"alice","active":true}`
	w := doRequest(h, http.MethodPut, "/scim/v2/Users/entity-1", body)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteUser(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodDelete, "/scim/v2/Users/entity-1", "")

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteUserNotFound(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodDelete, "/scim/v2/Users/nonexistent", "")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteUserError(t *testing.T) {
	client := openbao.NewClient("http://127.0.0.1:1", "token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodDelete, "/scim/v2/Users/entity-1", "")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestDeleteUserEntityDeleteFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/identity/entity/id/entity-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "entity-1",
				"name": "alice",
			},
		})
	})
	mux.HandleFunc("/v1/auth/userpass/users/alice", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/v1/identity/entity/name/alice", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodDelete, "/scim/v2/Users/entity-1", "")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// -- Groups CRUD --

func TestListGroups(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodGet, "/scim/v2/Groups", "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ListResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.TotalResults != 2 {
		t.Errorf("expected 2 results, got %d", resp.TotalResults)
	}
}

func TestListGroupsError(t *testing.T) {
	client := openbao.NewClient("http://127.0.0.1:1", "token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodGet, "/scim/v2/Groups", "")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestListGroupsMethodNotAllowed(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodDelete, "/scim/v2/Groups", "")

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestGetGroup(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodGet, "/scim/v2/Groups/group-1", "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var group Group
	json.NewDecoder(w.Body).Decode(&group)
	if group.ID != "group-1" {
		t.Errorf("expected id group-1, got %s", group.ID)
	}
	if group.DisplayName != "admins" {
		t.Errorf("expected displayName admins, got %s", group.DisplayName)
	}
	if len(group.Members) != 1 || group.Members[0].Value != "entity-1" {
		t.Errorf("unexpected members: %v", group.Members)
	}
	if group.Meta.ResourceType != "Group" {
		t.Errorf("expected ResourceType Group, got %s", group.Meta.ResourceType)
	}
}

func TestGetGroupNotFound(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodGet, "/scim/v2/Groups/nonexistent", "")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetGroupError(t *testing.T) {
	client := openbao.NewClient("http://127.0.0.1:1", "token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodGet, "/scim/v2/Groups/group-1", "")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestGetGroupMethodNotAllowed(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodPatch, "/scim/v2/Groups/group-1", "")

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestCreateGroup(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"newgroup","members":[{"value":"entity-1"}]}`
	w := doRequest(h, http.MethodPost, "/scim/v2/Groups", body)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var group Group
	json.NewDecoder(w.Body).Decode(&group)
	if group.ID != "new-group" {
		t.Errorf("expected id new-group, got %s", group.ID)
	}
}

func TestCreateGroupMissingDisplayName(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"]}`
	w := doRequest(h, http.MethodPost, "/scim/v2/Groups", body)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateGroupInvalidJSON(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodPost, "/scim/v2/Groups", "not json")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateGroupConflict(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/identity/group", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"existing"}`
	w := doRequest(h, http.MethodPost, "/scim/v2/Groups", body)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestCreateGroupEmptyMembers(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"newgroup","members":[]}`
	w := doRequest(h, http.MethodPost, "/scim/v2/Groups", body)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateGroup(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"admins-updated","members":[{"value":"entity-1"},{"value":"entity-2"}]}`
	w := doRequest(h, http.MethodPut, "/scim/v2/Groups/group-1", body)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateGroupNotFound(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"x"}`
	w := doRequest(h, http.MethodPut, "/scim/v2/Groups/nonexistent", body)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateGroupInvalidJSON(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodPut, "/scim/v2/Groups/group-1", "not json")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateGroupError(t *testing.T) {
	client := openbao.NewClient("http://127.0.0.1:1", "token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"x"}`
	w := doRequest(h, http.MethodPut, "/scim/v2/Groups/group-1", body)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestUpdateGroupNoDisplayName(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	// No displayName - should use existing name
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"members":[{"value":"entity-1"}]}`
	w := doRequest(h, http.MethodPut, "/scim/v2/Groups/group-1", body)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateGroupUpdateFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/identity/group/id/group-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "group-1",
				"name": "admins",
			},
		})
	})
	mux.HandleFunc("/v1/identity/group", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"admins","members":[]}`
	w := doRequest(h, http.MethodPut, "/scim/v2/Groups/group-1", body)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestDeleteGroup(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodDelete, "/scim/v2/Groups/group-1", "")

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteGroupNotFound(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodDelete, "/scim/v2/Groups/nonexistent", "")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteGroupError(t *testing.T) {
	client := openbao.NewClient("http://127.0.0.1:1", "token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodDelete, "/scim/v2/Groups/group-1", "")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestDeleteGroupDeleteFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/identity/group/id/group-1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"id":   "group-1",
					"name": "admins",
				},
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodDelete, "/scim/v2/Groups/group-1", "")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// -- Not Found / Routing --

func TestNotFoundEndpoint(t *testing.T) {
	h := &Handler{
		Client:  openbao.NewClient("http://localhost:1", "token", true),
		BaseURL: "https://example.com",
	}

	w := doRequest(h, http.MethodGet, "/scim/v2/nonexistent", "")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// -- Schema/Serialization Tests --

func TestServiceProviderConfig(t *testing.T) {
	cfg := GetServiceProviderConfig()
	if len(cfg.Schemas) != 1 || cfg.Schemas[0] != SchemaServiceProvider {
		t.Errorf("unexpected schemas: %v", cfg.Schemas)
	}
	if cfg.Filter.Supported != true {
		t.Error("expected filter to be supported")
	}
	if cfg.Filter.MaxResults != 200 {
		t.Errorf("expected MaxResults 200, got %d", cfg.Filter.MaxResults)
	}
	if cfg.Bulk.Supported != false {
		t.Error("expected bulk to not be supported")
	}
	if cfg.Patch.Supported != false {
		t.Error("expected patch to not be supported")
	}
	if cfg.ChangePassword.Supported != false {
		t.Error("expected changePassword to not be supported")
	}
	if cfg.Sort.Supported != false {
		t.Error("expected sort to not be supported")
	}
	if cfg.Etag.Supported != false {
		t.Error("expected etag to not be supported")
	}
	if len(cfg.AuthenticationSchemes) != 1 {
		t.Fatalf("expected 1 auth scheme, got %d", len(cfg.AuthenticationSchemes))
	}
	if cfg.AuthenticationSchemes[0].Type != "oauthbearertoken" {
		t.Errorf("expected oauthbearertoken, got %s", cfg.AuthenticationSchemes[0].Type)
	}
}

func TestResourceTypes(t *testing.T) {
	rts := GetResourceTypes()
	if len(rts) != 2 {
		t.Fatalf("expected 2 resource types, got %d", len(rts))
	}
	if rts[0].Name != "User" {
		t.Errorf("expected first resource type User, got %s", rts[0].Name)
	}
	if rts[0].Endpoint != "/scim/v2/Users" {
		t.Errorf("expected endpoint /scim/v2/Users, got %s", rts[0].Endpoint)
	}
	if rts[1].Name != "Group" {
		t.Errorf("expected second resource type Group, got %s", rts[1].Name)
	}
	if rts[1].Endpoint != "/scim/v2/Groups" {
		t.Errorf("expected endpoint /scim/v2/Groups, got %s", rts[1].Endpoint)
	}
}

func TestGetSchemas(t *testing.T) {
	schemas := GetSchemas()
	if len(schemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(schemas))
	}
	if schemas[0]["id"] != SchemaUser {
		t.Errorf("expected User schema, got %v", schemas[0]["id"])
	}
	if schemas[1]["id"] != SchemaGroup {
		t.Errorf("expected Group schema, got %v", schemas[1]["id"])
	}
}

func TestUserSerialization(t *testing.T) {
	user := User{
		Schemas:     []string{SchemaUser},
		ID:          "user-123",
		UserName:    "jdoe",
		DisplayName: "John Doe",
		Active:      true,
		Emails:      []Email{{Value: "jdoe@example.com", Type: "work", Primary: true}},
		Name:        &Name{GivenName: "John", FamilyName: "Doe", Formatted: "John Doe"},
		Groups:      []GroupRef{{Value: "group-1", Display: "Admins"}},
		Meta: Meta{
			ResourceType: "User",
			Location:     "https://example.com/scim/v2/Users/user-123",
		},
	}

	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded User
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.UserName != "jdoe" {
		t.Errorf("expected userName jdoe, got %s", decoded.UserName)
	}
	if decoded.Name.GivenName != "John" {
		t.Errorf("expected givenName John, got %s", decoded.Name.GivenName)
	}
	if len(decoded.Emails) != 1 || decoded.Emails[0].Value != "jdoe@example.com" {
		t.Errorf("unexpected emails: %v", decoded.Emails)
	}
}

func TestGroupSerialization(t *testing.T) {
	group := Group{
		Schemas:     []string{SchemaGroup},
		ID:          "group-1",
		DisplayName: "Admins",
		Members: []MemberRef{
			{Value: "user-1", Display: "Alice"},
			{Value: "user-2", Display: "Bob"},
		},
		Meta: Meta{
			ResourceType: "Group",
			Location:     "https://example.com/scim/v2/Groups/group-1",
		},
	}

	data, err := json.Marshal(group)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded Group
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.DisplayName != "Admins" {
		t.Errorf("expected displayName Admins, got %s", decoded.DisplayName)
	}
	if len(decoded.Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(decoded.Members))
	}
}

func TestErrorResponseFormat(t *testing.T) {
	h := &Handler{
		Client:  openbao.NewClient("http://localhost:1", "token", true),
		BaseURL: "https://example.com",
	}

	w := doRequest(h, http.MethodGet, "/scim/v2/nonexistent", "")

	if ct := w.Header().Get("Content-Type"); ct != "application/scim+json" {
		t.Errorf("expected Content-Type application/scim+json, got %s", ct)
	}

	var errResp ErrorResponse
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Status != "404" {
		t.Errorf("expected status 404, got %s", errResp.Status)
	}
	if errResp.Schemas[0] != SchemaError {
		t.Errorf("unexpected error schema: %s", errResp.Schemas[0])
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/scim+json" {
		t.Errorf("expected application/scim+json, got %s", ct)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Detail != "test error" {
		t.Errorf("expected detail 'test error', got %s", resp.Detail)
	}
	if resp.Status != "400" {
		t.Errorf("expected status '400', got %s", resp.Status)
	}
}

func TestNowRFC3339(t *testing.T) {
	ts := nowRFC3339()
	if ts == "" {
		t.Error("expected non-empty timestamp")
	}
	if !strings.Contains(ts, "T") {
		t.Errorf("expected RFC3339 format with T, got %s", ts)
	}
}

func TestEntityToUserNoMetadata(t *testing.T) {
	entity := &openbao.Entity{
		ID:       "entity-x",
		Name:     "testuser",
		Metadata: nil,
		Disabled: false,
	}

	user := entityToUser(entity, "https://example.com")
	if user.ID != "entity-x" {
		t.Errorf("expected id entity-x, got %s", user.ID)
	}
	if user.UserName != "testuser" {
		t.Errorf("expected userName testuser, got %s", user.UserName)
	}
	if user.Name != nil {
		t.Error("expected nil name when no name metadata")
	}
	if len(user.Emails) != 0 {
		t.Errorf("expected no emails, got %v", user.Emails)
	}
	if !user.Active {
		t.Error("expected active true")
	}
}

func TestEntityToUserDisabled(t *testing.T) {
	entity := &openbao.Entity{
		ID:       "entity-d",
		Name:     "disabled",
		Metadata: map[string]string{"email": "d@example.com"},
		Disabled: true,
	}

	user := entityToUser(entity, "https://example.com")
	if user.Active {
		t.Error("expected active false for disabled entity")
	}
}

func TestEntityToUserEmptyMetadata(t *testing.T) {
	entity := &openbao.Entity{
		ID:       "entity-e",
		Name:     "empty",
		Metadata: map[string]string{},
		Disabled: false,
	}

	user := entityToUser(entity, "https://example.com")
	if user.Name != nil {
		t.Error("expected nil name for empty metadata")
	}
	if len(user.Emails) != 0 {
		t.Error("expected no emails for empty metadata")
	}
}

func TestEntityToUserPartialName(t *testing.T) {
	entity := &openbao.Entity{
		ID:       "entity-p",
		Name:     "partial",
		Metadata: map[string]string{"given_name": "Alice"},
		Disabled: false,
	}

	user := entityToUser(entity, "https://example.com")
	if user.Name == nil {
		t.Fatal("expected name")
	}
	if user.Name.GivenName != "Alice" {
		t.Errorf("expected givenName Alice, got %s", user.Name.GivenName)
	}
	if user.Name.Formatted != "Alice" {
		t.Errorf("expected formatted 'Alice', got '%s'", user.Name.Formatted)
	}
}

func TestOpenbaoGroupToSCIM(t *testing.T) {
	group := &openbao.Group{
		ID:              "grp-1",
		Name:            "testgroup",
		MemberEntityIDs: []string{"e1", "e2"},
		Type:            "internal",
	}

	scimGroup := openbaoGroupToSCIM(group, "https://example.com")
	if scimGroup.ID != "grp-1" {
		t.Errorf("expected id grp-1, got %s", scimGroup.ID)
	}
	if scimGroup.DisplayName != "testgroup" {
		t.Errorf("expected displayName testgroup, got %s", scimGroup.DisplayName)
	}
	if len(scimGroup.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(scimGroup.Members))
	}
	if scimGroup.Members[0].Value != "e1" {
		t.Errorf("expected first member e1, got %s", scimGroup.Members[0].Value)
	}
	if scimGroup.Members[0].Ref != "https://example.com/scim/v2/Users/e1" {
		t.Errorf("unexpected member ref: %s", scimGroup.Members[0].Ref)
	}
	if scimGroup.Meta.Location != "https://example.com/scim/v2/Groups/grp-1" {
		t.Errorf("unexpected location: %s", scimGroup.Meta.Location)
	}
}

func TestOpenbaoGroupToSCIMNoMembers(t *testing.T) {
	group := &openbao.Group{
		ID:              "grp-2",
		Name:            "empty",
		MemberEntityIDs: nil,
	}

	scimGroup := openbaoGroupToSCIM(group, "https://example.com")
	if len(scimGroup.Members) != 0 {
		t.Errorf("expected 0 members, got %d", len(scimGroup.Members))
	}
}

func TestCreateGroupMembersWithEmptyValue(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	// Members with an empty value should be skipped
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"newgroup","members":[{"value":"entity-1"},{"value":""}]}`
	w := doRequest(h, http.MethodPost, "/scim/v2/Groups", body)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

// Test ListResponse serialization
func TestListResponseSerialization(t *testing.T) {
	users := []User{{Schemas: []string{SchemaUser}, ID: "u1", UserName: "test"}}
	resourcesJSON, _ := json.Marshal(users)
	resp := ListResponse{
		Schemas:      []string{SchemaListResponse},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: 1,
		Resources:    resourcesJSON,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ListResponse
	json.Unmarshal(data, &decoded)
	if decoded.TotalResults != 1 {
		t.Errorf("expected totalResults 1, got %d", decoded.TotalResults)
	}

	// Check Resources can be unmarshaled to users
	var decodedUsers []User
	json.Unmarshal(decoded.Resources, &decodedUsers)
	if len(decodedUsers) != 1 || decodedUsers[0].ID != "u1" {
		t.Errorf("unexpected decoded users: %v", decodedUsers)
	}
}

// Test ListUsers when individual entity fetch fails (should skip)
func TestListUsersWithFetchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/identity/entity/name", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "LIST" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"keys": []string{"good", "bad"},
				},
			})
			return
		}
	})
	mux.HandleFunc("/v1/identity/entity/name/good", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "entity-good",
				"name": "good",
			},
		})
	})
	mux.HandleFunc("/v1/identity/entity/name/bad", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodGet, "/scim/v2/Users", "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp ListResponse
	json.NewDecoder(w.Body).Decode(&resp)
	// Should have 1 user (the one that succeeded)
	if resp.TotalResults != 1 {
		t.Errorf("expected 1 result (skipping failed), got %d", resp.TotalResults)
	}
}

// Test ListGroups when individual group fetch fails (should skip)
func TestListGroupsWithFetchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/identity/group/name", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "LIST" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"keys": []string{"good", "bad"},
				},
			})
			return
		}
	})
	mux.HandleFunc("/v1/identity/group/name/good", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "group-good",
				"name": "good",
			},
		})
	})
	mux.HandleFunc("/v1/identity/group/name/bad", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	w := doRequest(h, http.MethodGet, "/scim/v2/Groups", "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp ListResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.TotalResults != 1 {
		t.Errorf("expected 1 result (skipping failed), got %d", resp.TotalResults)
	}
}

// Test UpdateUser where re-fetch after update fails
func TestUpdateUserRefetchFails(t *testing.T) {
	mux := http.NewServeMux()
	fetchCount := 0
	mux.HandleFunc("/v1/identity/entity/id/entity-1", func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		if fetchCount == 1 {
			// First fetch succeeds
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"id":   "entity-1",
					"name": "alice",
				},
			})
		} else {
			// Re-fetch after update returns nil
			w.WriteHeader(http.StatusNotFound)
		}
	})
	mux.HandleFunc("/v1/identity/entity/name/alice", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusNoContent)
		}
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"alice","active":true}`
	w := doRequest(h, http.MethodPut, "/scim/v2/Users/entity-1", body)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test UpdateGroup where re-fetch after update fails
func TestUpdateGroupRefetchFails(t *testing.T) {
	mux := http.NewServeMux()
	fetchCount := 0
	mux.HandleFunc("/v1/identity/group/id/group-1", func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		if fetchCount == 1 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"id":   "group-1",
					"name": "admins",
				},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	mux.HandleFunc("/v1/identity/group", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"admins","members":[]}`
	w := doRequest(h, http.MethodPut, "/scim/v2/Groups/group-1", body)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Test CreateUser where CreateEntity returns 204 (success, but the json unmarshal fails on empty body)
func TestCreateUserNoEmailNoName(t *testing.T) {
	ts, client := mockBaoServer(t)
	defer ts.Close()

	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"minimal","active":true}`
	w := doRequest(h, http.MethodPost, "/scim/v2/Users", body)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

// Test CreateUser where userpass creation fails (should still succeed)
func TestCreateUserUserpassFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/identity/entity", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "new-entity",
				"name": "newuser",
			},
		})
	})
	mux.HandleFunc("/v1/auth/userpass/users/newuser", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // userpass fails
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	h := &Handler{Client: client, BaseURL: "https://example.com"}
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"newuser","password":"secret"}`
	w := doRequest(h, http.MethodPost, "/scim/v2/Users", body)

	// Should still return 201 since entity was created (userpass is best-effort)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

// Verify unused bytes import is gone
var _ = bytes.NewBufferString
