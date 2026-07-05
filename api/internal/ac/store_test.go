package ac

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockBao creates an httptest server that simulates OpenBao responses.
func mockBao(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// userpass users LIST
	mux.HandleFunc("/v1/auth/userpass/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "LIST" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"keys": []string{"alice", "bob"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// userpass user CRUD
	mux.HandleFunc("/v1/auth/userpass/users/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PUT":
			w.WriteHeader(http.StatusNoContent)
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{})
		}
	})

	// identity entity by name
	mux.HandleFunc("/v1/identity/entity/name/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/v1/identity/entity/name/")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":       "eid-" + name,
				"name":     name,
				"disabled": false,
				"metadata": map[string]any{
					"email": name + "@test.com",
					"name":  strings.Title(name),
				},
				"group_ids": []string{"grp-001"},
			},
		})
	})

	// identity entity POST (create)
	mux.HandleFunc("/v1/identity/entity", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id": "eid-new",
				},
			})
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// identity entity by ID
	mux.HandleFunc("/v1/identity/entity/id/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/v1/identity/entity/id/")
		switch r.Method {
		case "GET":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":   id,
					"name": "testuser",
					"metadata": map[string]any{
						"email": "test@test.com",
						"name":  "Test User",
					},
					"group_ids": []string{},
				},
			})
		case "POST":
			w.WriteHeader(http.StatusNoContent)
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// identity group name LIST
	mux.HandleFunc("/v1/identity/group/name", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "LIST" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"keys": []string{"admins"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// identity group by name
	mux.HandleFunc("/v1/identity/group/name/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":                "grp-001",
				"name":             "admins",
				"metadata":         map[string]any{"builtin": "true"},
				"member_entity_ids": []string{"eid-alice"},
				"policies":         []string{"admin"},
			},
		})
	})

	// identity group POST (create)
	mux.HandleFunc("/v1/identity/group", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id": "grp-new",
				},
			})
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// identity group by ID
	mux.HandleFunc("/v1/identity/group/id/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/v1/identity/group/id/")
		switch r.Method {
		case "GET":
			builtin := "false"
			if id == "grp-builtin" {
				builtin = "true"
			}
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":                id,
					"name":             "test-group",
					"metadata":         map[string]any{"builtin": builtin},
					"member_entity_ids": []string{"eid-alice"},
					"policies":         []string{"admin"},
				},
			})
		case "POST":
			w.WriteHeader(http.StatusNoContent)
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// KV settings
	mux.HandleFunc("/v1/convocate/data/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"data": map[string]any{
						"requireMfa":            true,
						"sessionTimeoutMinutes": float64(60),
						"passwordMinLength":     float64(16),
					},
				},
			})
		case "PUT":
			w.WriteHeader(http.StatusNoContent)
		}
	})

	return httptest.NewServer(mux)
}

func newTestStore(t *testing.T) (*Store, *httptest.Server) {
	srv := mockBao(t)
	s := &Store{
		addr:   srv.URL,
		token:  "test-token",
		client: srv.Client(),
		roles:  NewStore().roles,
	}
	return s, srv
}

func TestListUsers(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestCreateUser(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	u, err := s.CreateUser(User{Email: "new@test.com", Name: "New User"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.ID == "" {
		t.Error("user ID should not be empty")
	}
	if u.Email != "new@test.com" {
		t.Errorf("expected email 'new@test.com', got %q", u.Email)
	}
}

func TestUpdateUser(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	u, ok, err := s.UpdateUser("eid-alice", User{Name: "Updated Alice"})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
	if u.ID == "" {
		t.Error("user ID should not be empty")
	}
}

func TestUpdateUser_DisableStatus(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	_, ok, err := s.UpdateUser("eid-alice", User{Status: "disabled"})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
}

func TestUpdateUser_EnableStatus(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	_, ok, err := s.UpdateUser("eid-alice", User{Status: "active"})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
}

func TestDeleteUser(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	ok, err := s.DeleteUser("eid-alice")
	if err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
}

func TestListGroups(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	groups, err := s.ListGroups()
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Builtin != true {
		t.Error("expected builtin=true")
	}
}

func TestCreateGroup(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	g, err := s.CreateGroup("developers", []string{"node-view", "node-create"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if g.Name != "developers" {
		t.Errorf("expected name 'developers', got %q", g.Name)
	}
	if g.ID == "" {
		t.Error("group ID should not be empty")
	}
	if len(g.Roles) != 2 || g.Roles[0] != "node-view" || g.Roles[1] != "node-create" {
		t.Errorf("expected roles [node-view, node-create], got %v", g.Roles)
	}
}

func TestDeleteGroup(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	ok, err := s.DeleteGroup("grp-001")
	if err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
}

func TestDeleteGroup_Builtin(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	ok, err := s.DeleteGroup("grp-builtin")
	if err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	if ok {
		t.Error("expected ok=false for builtin group")
	}
}

func TestSetGroupUsers(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	g, ok, err := s.SetGroupUsers("grp-001", []string{"eid-alice", "eid-bob"})
	if err != nil {
		t.Fatalf("SetGroupUsers: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
	if g.ID == "" {
		t.Error("group ID should not be empty")
	}
}

func TestSetGroupRoles(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	g, ok, err := s.SetGroupRoles("grp-001", []string{"admin", "node-view"})
	if err != nil {
		t.Fatalf("SetGroupRoles: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
	if g.ID == "" {
		t.Error("group ID should not be empty")
	}
}

func TestListRoles(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	roles := s.ListRoles()
	if len(roles) == 0 {
		t.Error("expected roles to not be empty")
	}
}

func TestGetSettings(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	gs, err := s.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if !gs.RequireMFA {
		t.Error("expected RequireMFA=true")
	}
	if gs.SessionTimeoutMin != 60 {
		t.Errorf("expected SessionTimeoutMin=60, got %d", gs.SessionTimeoutMin)
	}
	if gs.PasswordMinLength != 16 {
		t.Errorf("expected PasswordMinLength=16, got %d", gs.PasswordMinLength)
	}
}

func TestSetSettings(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	gs, err := s.SetSettings(GlobalSettings{
		RequireMFA:           true,
		SessionTimeoutMin:    45,
		PasswordMinLength:    20,
	})
	if err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	if gs.SessionTimeoutMin != 45 {
		t.Errorf("expected SessionTimeoutMin=45, got %d", gs.SessionTimeoutMin)
	}
}

func TestMapStr(t *testing.T) {
	m := map[string]any{
		"a": map[string]any{
			"b": "value",
		},
	}
	if got := mapStr(m, "a", "b"); got != "value" {
		t.Errorf("expected 'value', got %q", got)
	}
	if got := mapStr(m, "a", "c"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
	if got := mapStr(m, "x"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestBaoRequest_ErrorPaths(t *testing.T) {
	// Test with a server that returns bad JSON
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	_, err := s.baoRequest("GET", "/v1/test", nil)
	if err == nil {
		t.Error("expected error for bad JSON")
	}
}

func TestBaoList_ErrorPaths(t *testing.T) {
	// Test with a server that returns 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errors":["fail"]}`))
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	_, err := s.baoList("/v1/test")
	if err == nil {
		t.Error("expected error for 500")
	}
}

func TestBaoList_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	_, err := s.baoList("/v1/test")
	if err == nil {
		t.Error("expected error for bad JSON")
	}
}

func TestBaoList_NoDataKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"nodata": true})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	keys, err := s.baoList("/v1/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if keys != nil {
		t.Errorf("expected nil keys, got %v", keys)
	}
}

func TestBaoList_DataNoKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"nokeys": true}})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	keys, err := s.baoList("/v1/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if keys != nil {
		t.Errorf("expected nil keys, got %v", keys)
	}
}

func TestBaoList_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	keys, err := s.baoList("/v1/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if keys != nil {
		t.Errorf("expected nil keys for 404, got %v", keys)
	}
}

func TestGetSettings_NilData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	gs, err := s.GetSettings()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return defaults
	if gs.SessionTimeoutMin != 30 {
		t.Errorf("expected default 30, got %d", gs.SessionTimeoutMin)
	}
}

func TestGetSettings_NilInnerData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"data": nil}})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	gs, err := s.GetSettings()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gs.PasswordMinLength != 12 {
		t.Errorf("expected default 12, got %d", gs.PasswordMinLength)
	}
}

func TestGetUser_NoEntity(t *testing.T) {
	// Entity lookup returns error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errors":["not found"]}`))
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	u, err := s.getUser("unknown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != "unknown" {
		t.Errorf("expected ID 'unknown', got %q", u.ID)
	}
}

func TestGetUser_NilData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	u, err := s.getUser("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Name != "test" {
		t.Errorf("expected Name 'test', got %q", u.Name)
	}
}

func TestCreateUser_UsesNameWhenNoEmail(t *testing.T) {
	s, srv := newTestStore(t)
	defer srv.Close()

	u, err := s.CreateUser(User{Name: "just-name"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Name != "just-name" {
		t.Errorf("expected name 'just-name', got %q", u.Name)
	}
}

func TestUpdateUser_EntityNoName(t *testing.T) {
	// Entity has no name field
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":       "eid-test",
				"name":     "",
				"metadata": map[string]any{},
			},
		})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	_, ok, err := s.UpdateUser("eid-test", User{Name: "New"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false for entity with no name")
	}
}

func TestDeleteUser_EntityNoName(t *testing.T) {
	// Entity has no name (empty username means no userpass deletion)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":   "eid-test",
					"name": "",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	ok, err := s.DeleteUser("eid-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
}

func TestDeleteUser_EntityNilData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	ok, err := s.DeleteUser("eid-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false for nil data")
	}
}

func TestSetGroupUsers_ReadbackNilData(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// GET returns nil data
		json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	_, ok, err := s.SetGroupUsers("grp-001", []string{"eid-alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false for nil data readback")
	}
}

func TestSetGroupRoles_ReadbackNilData(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	_, ok, err := s.SetGroupRoles("grp-001", []string{"admin"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false for nil data readback")
	}
}

func TestMapStr_NonStringValue(t *testing.T) {
	m := map[string]any{"key": 123}
	if got := mapStr(m, "key"); got != "" {
		t.Errorf("expected empty for non-string, got %q", got)
	}
}

func TestMapStr_NonMapIntermediate(t *testing.T) {
	m := map[string]any{"key": "not a map"}
	if got := mapStr(m, "key", "sub"); got != "" {
		t.Errorf("expected empty for non-map intermediate, got %q", got)
	}
}

func TestMapStrSlice_NonSliceValue(t *testing.T) {
	m := map[string]any{"key": "not a slice"}
	got := mapStrSlice(m, "key")
	if got != nil {
		t.Errorf("expected nil for non-slice, got %v", got)
	}
}

func TestMapStrSlice_NonMapIntermediate(t *testing.T) {
	m := map[string]any{"key": "not a map"}
	got := mapStrSlice(m, "key", "sub")
	if got != nil {
		t.Errorf("expected nil for non-map intermediate, got %v", got)
	}
}

func TestListUsers_GetUserError(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "LIST" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"keys": []any{"alice"}},
			})
			return
		}
		// GET entity by name fails
		callCount++
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errors":["not found"]}`))
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// getUser returns a minimal user when entity lookup fails
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}
}

func TestGetUser_DisabledEntity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":       "eid-test",
				"name":     "testuser",
				"disabled": true,
				"metadata": map[string]any{
					"email": "test@test.com",
					"name":  "Test User",
				},
				"group_ids": []string{"grp-001"},
			},
		})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	u, err := s.getUser("testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Status != "disabled" {
		t.Errorf("expected status 'disabled', got %q", u.Status)
	}
}

func TestGetUser_NoIDField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"name":     "testuser",
				"metadata": map[string]any{},
			},
		})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	u, err := s.getUser("testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When id is empty, falls back to username
	if u.ID != "testuser" {
		t.Errorf("expected ID 'testuser', got %q", u.ID)
	}
}

func TestGetUser_NoGroupIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":       "eid-test",
				"name":     "testuser",
				"metadata": map[string]any{"name": "Test", "email": "t@t.com"},
			},
		})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	u, err := s.getUser("testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Groups == nil || len(u.Groups) != 0 {
		t.Errorf("expected empty groups slice, got %v", u.Groups)
	}
}

func TestCreateUser_NilEntityResp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == "POST" {
			// Return 204 no content (nil response)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	u, err := s.CreateUser(User{Email: "test@test.com", Name: "Test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When entityResp is nil, ID falls back to username (name)
	if u.ID != "Test" {
		t.Errorf("expected ID 'Test', got %q", u.ID)
	}
}

func TestUpdateUser_NilEntityData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	_, ok, err := s.UpdateUser("eid-test", User{Name: "New"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false for nil data")
	}
}

func TestListGroups_SkipErrorGroups(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "LIST" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"keys": []any{"grp1", "grp2"}},
			})
			return
		}
		callCount++
		if callCount == 1 {
			// First group GET fails
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"errors":["fail"]}`))
			return
		}
		// Second group GET succeeds
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":                "grp-002",
				"name":             "grp2",
				"member_entity_ids": []string{},
				"policies":         []string{"admin"},
			},
		})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	groups, err := s.ListGroups()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First group should be skipped (GET error), second should work
	if len(groups) != 1 {
		t.Errorf("expected 1 group (skipped error), got %d", len(groups))
	}
}

func TestListGroups_NilGroupData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "LIST" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"keys": []any{"grp1"}},
			})
			return
		}
		// Return nil data
		json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	groups, err := s.ListGroups()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 groups (nil data skipped), got %d", len(groups))
	}
}

func TestListGroups_NonBuiltinNoPolicies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "LIST" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"keys": []any{"grp1"}},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":                "grp-001",
				"name":             "grp1",
				"member_entity_ids": []string{"a"},
			},
		})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	groups, err := s.ListGroups()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Builtin {
		t.Error("expected builtin=false")
	}
	if len(groups[0].Roles) != 0 {
		t.Errorf("expected 0 roles, got %d", len(groups[0].Roles))
	}
}

func TestSetGroupUsers_WithMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":                "grp-001",
				"name":             "test",
				"metadata":         map[string]any{"builtin": "true"},
				"member_entity_ids": []string{"a", "b"},
				"policies":         []string{"admin"},
			},
		})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	g, ok, err := s.SetGroupUsers("grp-001", []string{"a", "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
	if g.UserCount != 2 {
		t.Errorf("expected 2 users, got %d", g.UserCount)
	}
	if !g.Builtin {
		t.Error("expected builtin=true")
	}
}

func TestSetGroupRoles_WithMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":                "grp-001",
				"name":             "test",
				"metadata":         map[string]any{"builtin": "true"},
				"member_entity_ids": []string{"a"},
				"policies":         []string{"admin", "node-view"},
			},
		})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	g, ok, err := s.SetGroupRoles("grp-001", []string{"admin", "node-view"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
	if len(g.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(g.Roles))
	}
}

func TestSetGroupUsers_GetError(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// GET fails
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errors":["fail"]}`))
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	_, _, err := s.SetGroupUsers("grp-001", []string{"a"})
	if err == nil {
		t.Error("expected error from readback failure")
	}
}

func TestSetGroupRoles_GetError(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errors":["fail"]}`))
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	_, _, err := s.SetGroupRoles("grp-001", []string{"a"})
	if err == nil {
		t.Error("expected error from readback failure")
	}
}

func TestUpdateUser_PostMetadataFails(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 && r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id": "eid-test", "name": "testuser",
					"metadata": map[string]any{"email": "t@t.com", "name": "Test"},
				},
			})
			return
		}
		if r.Method == "POST" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"errors":["fail"]}`))
			return
		}
		// Fallback
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"id": "eid-test", "name": "testuser",
			"metadata": map[string]any{"email": "t@t.com", "name": "Test"},
		}})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	_, _, err := s.UpdateUser("eid-test", User{Name: "Updated", Email: "new@test.com"})
	if err == nil {
		t.Error("expected error from metadata update failure")
	}
}

func TestBaoRequest_NoBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "" {
			t.Error("expected no Content-Type for nil body")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	result, err := s.baoRequest("GET", "/v1/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for 204")
	}
}

func TestBaoRequest_400Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"errors": []string{"bad request"}})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	_, err := s.baoRequest("GET", "/v1/test", nil)
	if err == nil {
		t.Error("expected error for 400")
	}
}

func TestBaoRequest_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Empty body
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	result, err := s.baoRequest("GET", "/v1/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for empty body")
	}
}

func TestListUsers_GetUserReturnsError(t *testing.T) {
	// When getUser returns a real error, ListUsers should propagate it
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "LIST" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"keys": []any{"alice"}},
			})
			return
		}
		callCount++
		// getUser calls baoRequest("GET", "/v1/identity/entity/name/alice")
		// If it returns error, getUser returns minimal user with nil error
		// So ListUsers never gets an error from getUser - it's swallowed.
		// Let's verify that path works.
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errors":["not found"]}`))
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}
	// Minimal user fallback
	if users[0].Name != "alice" {
		t.Errorf("expected name 'alice', got %q", users[0].Name)
	}
}

func TestCreateUser_IdentityEntityError(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First PUT for userpass succeeds
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// POST for identity entity fails
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errors":["fail"]}`))
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	_, err := s.CreateUser(User{Email: "test@test.com", Name: "Test"})
	if err == nil {
		t.Error("expected error from identity entity creation failure")
	}
}

func TestSetGroupUsers_NilPolicies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":                "grp-001",
				"name":             "test",
				"member_entity_ids": []string{},
				// No policies field
			},
		})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	g, ok, err := s.SetGroupUsers("grp-001", []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
	if g.Roles == nil || len(g.Roles) != 0 {
		t.Errorf("expected empty roles, got %v", g.Roles)
	}
}

func TestSetGroupRoles_NilPolicies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":                "grp-001",
				"name":             "test",
				"member_entity_ids": []string{},
			},
		})
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	g, ok, err := s.SetGroupRoles("grp-001", []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
	if g.Roles == nil || len(g.Roles) != 0 {
		t.Errorf("expected empty roles, got %v", g.Roles)
	}
}

func TestMapStr_SingleKey(t *testing.T) {
	m := map[string]any{"key": "value"}
	if got := mapStr(m, "key"); got != "value" {
		t.Errorf("expected 'value', got %q", got)
	}
}

func TestMapStrSlice_SingleKey(t *testing.T) {
	m := map[string]any{"key": []any{"a", "b"}}
	got := mapStrSlice(m, "key")
	if len(got) != 2 {
		t.Errorf("expected 2 items, got %d", len(got))
	}
}

func TestMapStrSlice(t *testing.T) {
	m := map[string]any{
		"data": map[string]any{
			"keys": []any{"a", "b", "c"},
		},
	}
	got := mapStrSlice(m, "data", "keys")
	if len(got) != 3 {
		t.Errorf("expected 3 items, got %d", len(got))
	}
	got = mapStrSlice(m, "data", "missing")
	if got != nil {
		t.Error("expected nil for missing key")
	}
}
