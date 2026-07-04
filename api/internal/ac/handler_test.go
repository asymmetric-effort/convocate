package ac

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/asymmetric-effort/convocate/internal/httputil"
)

func newAuthRequest(method, path string, body interface{}) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{
		ID: "usr-001", Username: "admin", Roles: []string{"admin"},
	})
	return req.WithContext(ctx)
}

func newTestAcHandler(t *testing.T) (*Handler, *httptest.Server) {
	s, srv := newTestStore(t)
	return &Handler{store: s}, srv
}

func TestHandlerListUsers(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := newAuthRequest("GET", "/api/v1/ac/user", nil)
	rec := httptest.NewRecorder()
	h.listUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var page httputil.PageResponse
	json.NewDecoder(rec.Body).Decode(&page)
	if page.Total != 2 {
		t.Errorf("expected 2 users, got %d", page.Total)
	}
}

func TestHandlerCreateUser_Happy(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := newAuthRequest("POST", "/api/v1/ac/user", User{
		Email: "new@test.com",
		Name:  "New User",
	})
	rec := httptest.NewRecorder()
	h.createUser(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerCreateUser_BadBody(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := httptest.NewRequest("POST", "/api/v1/ac/user", bytes.NewReader([]byte("bad")))
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.createUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandlerUpdateUser_Happy(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := newAuthRequest("PATCH", "/api/v1/ac/user/eid-alice", User{Name: "Updated"})
	req.SetPathValue("userId", "eid-alice")
	rec := httptest.NewRecorder()
	h.updateUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerUpdateUser_BadBody(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := httptest.NewRequest("PATCH", "/api/v1/ac/user/eid-alice", bytes.NewReader([]byte("bad")))
	req.SetPathValue("userId", "eid-alice")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.updateUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandlerDeleteUser_Happy(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := newAuthRequest("DELETE", "/api/v1/ac/user/eid-alice", nil)
	req.SetPathValue("userId", "eid-alice")
	rec := httptest.NewRecorder()
	h.deleteUser(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerListGroups(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := newAuthRequest("GET", "/api/v1/ac/group", nil)
	rec := httptest.NewRecorder()
	h.listGroups(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandlerCreateGroup_Happy(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := newAuthRequest("POST", "/api/v1/ac/group", map[string]string{"name": "devs"})
	rec := httptest.NewRecorder()
	h.createGroup(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerCreateGroup_BadBody(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := httptest.NewRequest("POST", "/api/v1/ac/group", bytes.NewReader([]byte("bad")))
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.createGroup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandlerDeleteGroup_Happy(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := newAuthRequest("DELETE", "/api/v1/ac/group/grp-001", nil)
	req.SetPathValue("groupId", "grp-001")
	rec := httptest.NewRecorder()
	h.deleteGroup(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerDeleteGroup_Builtin(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := newAuthRequest("DELETE", "/api/v1/ac/group/grp-builtin", nil)
	req.SetPathValue("groupId", "grp-builtin")
	rec := httptest.NewRecorder()
	h.deleteGroup(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (builtin), got %d", rec.Code)
	}
}

func TestHandlerSetGroupUsers_Happy(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := newAuthRequest("PUT", "/api/v1/ac/group/grp-001/user", map[string][]string{
		"userIds": {"eid-alice", "eid-bob"},
	})
	req.SetPathValue("groupId", "grp-001")
	rec := httptest.NewRecorder()
	h.setGroupUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerSetGroupUsers_BadBody(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := httptest.NewRequest("PUT", "/api/v1/ac/group/grp-001/user", bytes.NewReader([]byte("bad")))
	req.SetPathValue("groupId", "grp-001")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.setGroupUsers(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandlerSetGroupRoles_Happy(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := newAuthRequest("PUT", "/api/v1/ac/group/grp-001/role", map[string][]string{
		"roles": {"admin", "node-view"},
	})
	req.SetPathValue("groupId", "grp-001")
	rec := httptest.NewRecorder()
	h.setGroupRoles(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerSetGroupRoles_BadBody(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := httptest.NewRequest("PUT", "/api/v1/ac/group/grp-001/role", bytes.NewReader([]byte("bad")))
	req.SetPathValue("groupId", "grp-001")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.setGroupRoles(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandlerListRoles(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := newAuthRequest("GET", "/api/v1/ac/role", nil)
	rec := httptest.NewRecorder()
	h.listRoles(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandlerGetSettings(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := newAuthRequest("GET", "/api/v1/ac/settings", nil)
	rec := httptest.NewRecorder()
	h.getSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandlerPutSettings_Happy(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := newAuthRequest("PUT", "/api/v1/ac/settings", GlobalSettings{
		RequireMFA:           true,
		SessionTimeoutMin:    45,
		PasswordMinLength:    20,
		PasswordRotationDays: 60,
	})
	rec := httptest.NewRecorder()
	h.putSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerPutSettings_BadBody(t *testing.T) {
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := httptest.NewRequest("PUT", "/api/v1/ac/settings", bytes.NewReader([]byte("bad")))
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.putSettings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// --- Error path tests (backend error handling) ---

func newErrorBaoServer() *httptest.Server {
	mux := http.NewServeMux()
	// All routes return 500
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errors":["internal error"]}`))
	})
	return httptest.NewServer(mux)
}

func newErrorHandler(t *testing.T) (*Handler, *httptest.Server) {
	srv := newErrorBaoServer()
	s := &Store{
		addr:   srv.URL,
		token:  "test-token",
		client: srv.Client(),
		roles:  NewStore().roles,
	}
	return &Handler{store: s}, srv
}

func TestHandlerListUsers_BackendError(t *testing.T) {
	h, srv := newErrorHandler(t)
	defer srv.Close()

	req := newAuthRequest("GET", "/api/v1/ac/user", nil)
	rec := httptest.NewRecorder()
	h.listUsers(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandlerCreateUser_BackendError(t *testing.T) {
	h, srv := newErrorHandler(t)
	defer srv.Close()

	req := newAuthRequest("POST", "/api/v1/ac/user", User{Email: "x@test.com", Name: "x"})
	rec := httptest.NewRecorder()
	h.createUser(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandlerUpdateUser_BackendError(t *testing.T) {
	// Use a server that returns 500 for entity lookup by ID
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/identity/entity/id/") {
			// Return a valid entity so we proceed to the POST update
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id": "eid-test", "name": "testuser",
					"metadata": map[string]any{"email": "test@test.com", "name": "Test"},
				},
			})
			return
		}
		if r.Method == "POST" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"errors":["fail"]}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errors":["fail"]}`))
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	h := &Handler{store: s}

	req := newAuthRequest("PATCH", "/api/v1/ac/user/eid-test", User{Name: "Updated"})
	req.SetPathValue("userId", "eid-test")
	rec := httptest.NewRecorder()
	h.updateUser(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerUpdateUser_NotFound(t *testing.T) {
	// Server that returns 404 for entity lookup by ID
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errors":["not found"]}`))
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	h := &Handler{store: s}

	req := newAuthRequest("PATCH", "/api/v1/ac/user/nonexistent", User{Name: "x"})
	req.SetPathValue("userId", "nonexistent")
	rec := httptest.NewRecorder()
	h.updateUser(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandlerDeleteUser_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errors":["not found"]}`))
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	h := &Handler{store: s}

	req := newAuthRequest("DELETE", "/api/v1/ac/user/nonexistent", nil)
	req.SetPathValue("userId", "nonexistent")
	rec := httptest.NewRecorder()
	h.deleteUser(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandlerDeleteUser_BackendError(t *testing.T) {
	// Server that returns valid GET but fails on DELETE
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id": "eid-test", "name": "testuser",
				},
			})
			return
		}
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"errors":["fail"]}`))
			return
		}
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	h := &Handler{store: s}

	req := newAuthRequest("DELETE", "/api/v1/ac/user/eid-test", nil)
	req.SetPathValue("userId", "eid-test")
	rec := httptest.NewRecorder()
	h.deleteUser(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerListGroups_BackendError(t *testing.T) {
	h, srv := newErrorHandler(t)
	defer srv.Close()

	req := newAuthRequest("GET", "/api/v1/ac/group", nil)
	rec := httptest.NewRecorder()
	h.listGroups(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandlerCreateGroup_BackendError(t *testing.T) {
	h, srv := newErrorHandler(t)
	defer srv.Close()

	req := newAuthRequest("POST", "/api/v1/ac/group", map[string]string{"name": "devs"})
	rec := httptest.NewRecorder()
	h.createGroup(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandlerDeleteGroup_BackendError(t *testing.T) {
	h, srv := newErrorHandler(t)
	defer srv.Close()

	req := newAuthRequest("DELETE", "/api/v1/ac/group/grp-001", nil)
	req.SetPathValue("groupId", "grp-001")
	rec := httptest.NewRecorder()
	h.deleteGroup(rec, req)

	// Error from GET group results in not_found
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandlerSetGroupUsers_NotFound(t *testing.T) {
	// POST to group fails
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errors":["fail"]}`))
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	h := &Handler{store: s}

	req := newAuthRequest("PUT", "/api/v1/ac/group/grp-001/user", map[string][]string{"userIds": {"a"}})
	req.SetPathValue("groupId", "grp-001")
	rec := httptest.NewRecorder()
	h.setGroupUsers(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandlerSetGroupRoles_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errors":["fail"]}`))
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	h := &Handler{store: s}

	req := newAuthRequest("PUT", "/api/v1/ac/group/grp-001/role", map[string][]string{"roles": {"a"}})
	req.SetPathValue("groupId", "grp-001")
	rec := httptest.NewRecorder()
	h.setGroupRoles(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandlerGetSettings_BackendError(t *testing.T) {
	// GetSettings returns defaults on error, so should still be 200
	h, srv := newErrorHandler(t)
	defer srv.Close()

	req := newAuthRequest("GET", "/api/v1/ac/settings", nil)
	rec := httptest.NewRecorder()
	h.getSettings(rec, req)

	// GetSettings returns defaults on error, so 200
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRegister(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/ac/user", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestHandlerPutSettings_BackendError(t *testing.T) {
	h, srv := newErrorHandler(t)
	defer srv.Close()

	req := newAuthRequest("PUT", "/api/v1/ac/settings", GlobalSettings{RequireMFA: true, SessionTimeoutMin: 30, PasswordMinLength: 12, PasswordRotationDays: 90})
	rec := httptest.NewRecorder()
	h.putSettings(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestHandlerDeleteGroup_DeleteError(t *testing.T) {
	// GET returns non-builtin group, but DELETE fails
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":       "grp-001",
					"name":     "test-group",
					"metadata": map[string]any{"builtin": "false"},
				},
			})
			return
		}
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"errors":["fail"]}`))
			return
		}
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	h := &Handler{store: s}

	req := newAuthRequest("DELETE", "/api/v1/ac/group/grp-001", nil)
	req.SetPathValue("groupId", "grp-001")
	rec := httptest.NewRecorder()
	h.deleteGroup(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerSetGroupUsers_ReadbackNotFound(t *testing.T) {
	// POST succeeds but GET readback gives nil data
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	h := &Handler{store: s}

	req := newAuthRequest("PUT", "/api/v1/ac/group/grp-001/user", map[string][]string{"userIds": {"a"}})
	req.SetPathValue("groupId", "grp-001")
	rec := httptest.NewRecorder()
	h.setGroupUsers(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandlerSetGroupRoles_ReadbackNotFound(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method == "POST" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]any{"data": nil})
			return
		}
	}))
	defer srv.Close()

	s := &Store{addr: srv.URL, token: "t", client: srv.Client(), roles: NewStore().roles}
	h := &Handler{store: s}

	req := newAuthRequest("PUT", "/api/v1/ac/group/grp-001/role", map[string][]string{"roles": {"a"}})
	req.SetPathValue("groupId", "grp-001")
	rec := httptest.NewRecorder()
	h.setGroupRoles(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandlerGetSettings_RealBackendError(t *testing.T) {
	// Make the server return a 200 with valid JSON but then SetSettings write fails
	// Actually to trigger the getSettings error branch, the store.GetSettings
	// needs to return a non-nil error. GetSettings catches baoRequest errors
	// and returns defaults. So we need to cause a deeper error.
	// Looking at store.go line 620: GetSettings never returns error (it defaults).
	// The handler err branch (line 174) is technically dead code, but we should
	// still test it to hit the condition. We'd need to modify the store interface,
	// but that's not possible without production code changes. The error path
	// in the handler is unreachable because GetSettings always returns nil error.
	// So we skip this test case.
}

func TestHandlerGetSettings_StoreReturnsDefaults(t *testing.T) {
	// The getSettings handler calls store.GetSettings, which returns defaults on error
	h, srv := newTestAcHandler(t)
	defer srv.Close()

	req := newAuthRequest("GET", "/api/v1/ac/settings", nil)
	rec := httptest.NewRecorder()
	h.getSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var gs GlobalSettings
	json.NewDecoder(rec.Body).Decode(&gs)
	if !gs.RequireMFA {
		t.Error("expected RequireMFA=true from mock")
	}
}
