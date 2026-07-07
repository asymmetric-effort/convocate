package projects

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

// The projects package makes internal HTTP calls to localhost:8443,
// so we need to spin up a mock server for the sub-APIs.
func startMockSubAPIs(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// Mock IDE project list
	mux.HandleFunc("/api/v1/ide/project", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]string{
					{"id": "prj-001", "name": "demo-app", "repoId": "repo-001", "boardId": "brd-001", "agentId": "agt-001"},
				},
				"total":  1,
				"offset": 0,
				"limit":  200,
			})
		case "POST":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{
				"id":     "prj-002",
				"name":   "new-project",
				"repoId": "repo-002",
			})
		}
	})

	// Mock IDE project PATCH/DELETE
	mux.HandleFunc("/api/v1/ide/project/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"id": "prj-001", "name": "updated"})
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// Mock board creation
	mux.HandleFunc("/api/v1/pb/board", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "brd-002"})
	})

	// Mock agent creation
	mux.HandleFunc("/api/v1/amgr/agent", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"id": "agt-002"})
			return
		}
	})

	// Mock agent deletion
	mux.HandleFunc("/api/v1/amgr/agent/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	return httptest.NewServer(mux)
}

func TestListProjects(t *testing.T) {
	srv := startMockSubAPIs(t)
	defer srv.Close()

	// Override internalCall to use our test server
	origInternalCall := internalCallFn
	internalCallFn = func(method, path string, body interface{}) ([]byte, int, error) {
		return doInternalCall(srv.URL, method, path, body)
	}
	defer func() { internalCallFn = origInternalCall }()

	req := newAuthRequest("GET", "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	listProjects(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestGetProject_Happy(t *testing.T) {
	srv := startMockSubAPIs(t)
	defer srv.Close()

	origInternalCall := internalCallFn
	internalCallFn = func(method, path string, body interface{}) ([]byte, int, error) {
		return doInternalCall(srv.URL, method, path, body)
	}
	defer func() { internalCallFn = origInternalCall }()

	req := newAuthRequest("GET", "/api/v1/projects/prj-001", nil)
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	getProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestGetProject_NotFound(t *testing.T) {
	srv := startMockSubAPIs(t)
	defer srv.Close()

	origInternalCall := internalCallFn
	internalCallFn = func(method, path string, body interface{}) ([]byte, int, error) {
		return doInternalCall(srv.URL, method, path, body)
	}
	defer func() { internalCallFn = origInternalCall }()

	req := newAuthRequest("GET", "/api/v1/projects/prj-999", nil)
	req.SetPathValue("projectId", "prj-999")
	rec := httptest.NewRecorder()
	getProject(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateProject_Happy(t *testing.T) {
	srv := startMockSubAPIs(t)
	defer srv.Close()

	origInternalCall := internalCallFn
	internalCallFn = func(method, path string, body interface{}) ([]byte, int, error) {
		return doInternalCall(srv.URL, method, path, body)
	}
	defer func() { internalCallFn = origInternalCall }()

	req := newAuthRequest("POST", "/api/v1/projects", map[string]string{"name": "new-project"})
	rec := httptest.NewRecorder()
	createProject(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateProject_MissingName(t *testing.T) {
	req := newAuthRequest("POST", "/api/v1/projects", map[string]string{})
	rec := httptest.NewRecorder()
	createProject(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateProject_BadBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewReader([]byte("bad")))
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	createProject(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateProject(t *testing.T) {
	srv := startMockSubAPIs(t)
	defer srv.Close()

	origInternalCall := internalCallFn
	internalCallFn = func(method, path string, body interface{}) ([]byte, int, error) {
		return doInternalCall(srv.URL, method, path, body)
	}
	defer func() { internalCallFn = origInternalCall }()

	req := newAuthRequest("PATCH", "/api/v1/projects/prj-001", map[string]string{"name": "updated"})
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	updateProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteProject_Happy(t *testing.T) {
	srv := startMockSubAPIs(t)
	defer srv.Close()

	origInternalCall := internalCallFn
	internalCallFn = func(method, path string, body interface{}) ([]byte, int, error) {
		return doInternalCall(srv.URL, method, path, body)
	}
	defer func() { internalCallFn = origInternalCall }()

	req := newAuthRequest("DELETE", "/api/v1/projects/prj-001", nil)
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	deleteProject(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestRegister(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestListProjects_Error(t *testing.T) {
	origInternalCall := internalCallFn
	internalCallFn = func(method, path string, body interface{}) ([]byte, int, error) {
		return nil, 0, fmt.Errorf("connection refused")
	}
	defer func() { internalCallFn = origInternalCall }()

	req := newAuthRequest("GET", "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	listProjects(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestGetProject_Error(t *testing.T) {
	origInternalCall := internalCallFn
	internalCallFn = func(method, path string, body interface{}) ([]byte, int, error) {
		return nil, 0, fmt.Errorf("connection refused")
	}
	defer func() { internalCallFn = origInternalCall }()

	req := newAuthRequest("GET", "/api/v1/projects/prj-001", nil)
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	getProject(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestUpdateProject_Error(t *testing.T) {
	origInternalCall := internalCallFn
	internalCallFn = func(method, path string, body interface{}) ([]byte, int, error) {
		return nil, 0, fmt.Errorf("connection refused")
	}
	defer func() { internalCallFn = origInternalCall }()

	req := newAuthRequest("PATCH", "/api/v1/projects/prj-001", map[string]string{"name": "x"})
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	updateProject(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestCreateProject_IDEFailure(t *testing.T) {
	origInternalCall := internalCallFn
	internalCallFn = func(method, path string, body interface{}) ([]byte, int, error) {
		if method == "POST" && path == "/api/v1/ide/project" {
			return []byte("IDE failed"), http.StatusInternalServerError, nil
		}
		return []byte("{}"), 200, nil
	}
	defer func() { internalCallFn = origInternalCall }()

	req := newAuthRequest("POST", "/api/v1/projects", map[string]string{"name": "fail-project"})
	rec := httptest.NewRecorder()
	createProject(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestCreateProject_IDEError(t *testing.T) {
	origInternalCall := internalCallFn
	internalCallFn = func(method, path string, body interface{}) ([]byte, int, error) {
		if method == "POST" && path == "/api/v1/ide/project" {
			return nil, 0, fmt.Errorf("connection refused")
		}
		return []byte("{}"), 200, nil
	}
	defer func() { internalCallFn = origInternalCall }()

	req := newAuthRequest("POST", "/api/v1/projects", map[string]string{"name": "fail-project"})
	rec := httptest.NewRecorder()
	createProject(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestDeleteProject_Error(t *testing.T) {
	origInternalCall := internalCallFn
	internalCallFn = func(method, path string, body interface{}) ([]byte, int, error) {
		return nil, 0, fmt.Errorf("connection refused")
	}
	defer func() { internalCallFn = origInternalCall }()

	req := newAuthRequest("DELETE", "/api/v1/projects/prj-001", nil)
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	deleteProject(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestDeleteProject_NotFound(t *testing.T) {
	srv := startMockSubAPIs(t)
	defer srv.Close()

	origInternalCall := internalCallFn
	internalCallFn = func(method, path string, body interface{}) ([]byte, int, error) {
		return doInternalCall(srv.URL, method, path, body)
	}
	defer func() { internalCallFn = origInternalCall }()

	req := newAuthRequest("DELETE", "/api/v1/projects/prj-999", nil)
	req.SetPathValue("projectId", "prj-999")
	rec := httptest.NewRecorder()
	deleteProject(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", rec.Code, rec.Body.String())
	}
}
