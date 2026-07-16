package ide

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/llm"
)

func llmEndpoint() string     { return llm.Endpoint() }
func setLLMEndpoint(s string) { llm.SetEndpoint(s) }
func setLLMKey(s string)      { llm.SetAPIKey(s) }

func newAuthRequest(method, path string, body interface{}) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{
		ID: "usr-001", Username: "testuser", Roles: []string{"admin"},
	})
	return req.WithContext(ctx)
}

func newHandler() *Handler {
	return &Handler{store: NewStore()}
}

func TestListProjects(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/ide/project", nil)
	rec := httptest.NewRecorder()
	h.listProjects(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var page httputil.PageResponse
	json.NewDecoder(rec.Body).Decode(&page)
	if page.Total != 1 {
		t.Errorf("expected 1 project, got %d", page.Total)
	}
}

func TestCreateProject(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/ide/project", map[string]string{"name": "my-project"})
	rec := httptest.NewRecorder()
	h.createProject(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var p Project
	json.NewDecoder(rec.Body).Decode(&p)
	if p.Name != "my-project" {
		t.Errorf("expected name 'my-project', got %q", p.Name)
	}
	if p.ID == "" {
		t.Error("project ID should not be empty")
	}
}

func TestCreateProject_BadBody(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest("POST", "/api/v1/ide/project", bytes.NewReader([]byte("bad")))
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.createProject(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateProject_Happy(t *testing.T) {
	h := newHandler()
	name := "renamed"
	req := newAuthRequest("PATCH", "/api/v1/ide/project/prj-001", map[string]*string{"name": &name})
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	h.updateProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var p Project
	json.NewDecoder(rec.Body).Decode(&p)
	if p.Name != "renamed" {
		t.Errorf("expected name 'renamed', got %q", p.Name)
	}
}

func TestUpdateProject_NotFound(t *testing.T) {
	h := newHandler()
	name := "x"
	req := newAuthRequest("PATCH", "/api/v1/ide/project/prj-999", map[string]*string{"name": &name})
	req.SetPathValue("projectId", "prj-999")
	rec := httptest.NewRecorder()
	h.updateProject(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateProject_BadBody(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest("PATCH", "/api/v1/ide/project/prj-001", bytes.NewReader([]byte("bad")))
	req.SetPathValue("projectId", "prj-001")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.updateProject(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteProject_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("DELETE", "/api/v1/ide/project/prj-001", nil)
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	h.deleteProject(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestDeleteProject_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("DELETE", "/api/v1/ide/project/prj-999", nil)
	req.SetPathValue("projectId", "prj-999")
	rec := httptest.NewRecorder()
	h.deleteProject(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestTree(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/ide/project/prj-001/tree", nil)
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	h.tree(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var entries []FileEntry
	json.NewDecoder(rec.Body).Decode(&entries)
	if len(entries) < 1 {
		t.Error("expected at least 1 file entry")
	}
}

func TestTree_EmptyProject(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/ide/project/prj-999/tree", nil)
	req.SetPathValue("projectId", "prj-999")
	rec := httptest.NewRecorder()
	h.tree(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetFile_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/ide/project/prj-001/file/src/main.ts", nil)
	req.SetPathValue("projectId", "prj-001")
	req.SetPathValue("path", "src/main.ts")
	rec := httptest.NewRecorder()
	h.getFile(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var f FileContent
	json.NewDecoder(rec.Body).Decode(&f)
	if f.Content != "console.log('hello');" {
		t.Errorf("unexpected content: %q", f.Content)
	}
}

func TestGetFile_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/ide/project/prj-001/file/nonexistent.txt", nil)
	req.SetPathValue("projectId", "prj-001")
	req.SetPathValue("path", "nonexistent.txt")
	rec := httptest.NewRecorder()
	h.getFile(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestPutFile(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("PUT", "/api/v1/ide/project/prj-001/file/newfile.ts", map[string]string{
		"content": "const x = 1;",
	})
	req.SetPathValue("projectId", "prj-001")
	req.SetPathValue("path", "newfile.ts")
	rec := httptest.NewRecorder()
	h.putFile(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var f FileContent
	json.NewDecoder(rec.Body).Decode(&f)
	if f.Content != "const x = 1;" {
		t.Errorf("unexpected content: %q", f.Content)
	}
}

func TestPutFile_BadBody(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest("PUT", "/api/v1/ide/project/prj-001/file/x.ts", bytes.NewReader([]byte("bad")))
	req.SetPathValue("projectId", "prj-001")
	req.SetPathValue("path", "x.ts")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.putFile(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteFile_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("DELETE", "/api/v1/ide/project/prj-001/file/src/main.ts", nil)
	req.SetPathValue("projectId", "prj-001")
	req.SetPathValue("path", "src/main.ts")
	rec := httptest.NewRecorder()
	h.deleteFile(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestDeleteFile_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("DELETE", "/api/v1/ide/project/prj-001/file/nonexistent.txt", nil)
	req.SetPathValue("projectId", "prj-001")
	req.SetPathValue("path", "nonexistent.txt")
	rec := httptest.NewRecorder()
	h.deleteFile(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestRenameFile_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/ide/project/prj-001/rename-file", map[string]string{
		"oldPath": "src/main.ts",
		"newPath": "src/app.ts",
	})
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	h.renameFile(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var f FileContent
	json.NewDecoder(rec.Body).Decode(&f)
	if f.Path != "src/app.ts" {
		t.Errorf("expected path 'src/app.ts', got %q", f.Path)
	}
}

func TestRenameFile_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/ide/project/prj-001/rename-file", map[string]string{
		"oldPath": "nonexistent.ts",
		"newPath": "new.ts",
	})
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	h.renameFile(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestRenameFile_DestinationExists(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/ide/project/prj-001/rename-file", map[string]string{
		"oldPath": "src/main.ts",
		"newPath": "SPECIFICATION.md",
	})
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	h.renameFile(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (destination exists), got %d", rec.Code)
	}
}

func TestRenderBoard_NoSpec(t *testing.T) {
	h := newHandler()
	// Delete the SPECIFICATION.md file
	h.store.DeleteFile("prj-001", "SPECIFICATION.md")

	req := newAuthRequest("POST", "/api/v1/ide/project/prj-001/render-board", nil)
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	h.renderBoard(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestRegister(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/ide/project", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRenderBoard_LLMError(t *testing.T) {
	h := newHandler()
	// SPECIFICATION.md exists in default store; set LLM to fail
	origEndpoint := llmEndpoint()
	setLLMEndpoint("http://localhost:1") // unreachable
	setLLMKey("test-key")
	defer func() {
		setLLMEndpoint(origEndpoint)
		setLLMKey("")
	}()

	req := newAuthRequest("POST", "/api/v1/ide/project/prj-001/render-board", nil)
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	h.renderBoard(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestRenderBoard_Success(t *testing.T) {
	// Start a fake LLM server that returns valid decomposition JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"content": []map[string]string{
				{"type": "text", "text": `{"containers":[],"cards":[{"title":"Task 1","status":"todo","content":"do it","containerIndex":0,"x":10,"y":10,"w":100,"h":50}],"edges":[]}`},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	setLLMEndpoint(server.URL)
	setLLMKey("test-key")
	defer func() {
		setLLMEndpoint("")
		setLLMKey("")
	}()

	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/ide/project/prj-001/render-board", nil)
	req.SetPathValue("projectId", "prj-001")
	rec := httptest.NewRecorder()
	h.renderBoard(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestRenameFile_BadBody(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest("POST", "/api/v1/ide/project/prj-001/rename-file", bytes.NewReader([]byte("bad")))
	req.SetPathValue("projectId", "prj-001")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.renameFile(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
