package repo

import (
	"bytes"
	"encoding/json"
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
		ID: "usr-001", Username: "testuser", Roles: []string{"admin"},
	})
	return req.WithContext(ctx)
}

func newHandler() *Handler {
	return &Handler{store: NewStore()}
}

func TestListRepos(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/repo/repo", nil)
	rec := httptest.NewRecorder()
	h.listRepos(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var page httputil.PageResponse
	json.NewDecoder(rec.Body).Decode(&page)
	if page.Total != 2 {
		t.Errorf("expected 2 repos, got %d", page.Total)
	}
}

func TestCreateRepo(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/repo/repo", map[string]string{
		"name":       "new-repo",
		"visibility": "private",
	})
	rec := httptest.NewRecorder()
	h.createRepo(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var r Repo
	json.NewDecoder(rec.Body).Decode(&r)
	if r.Name != "new-repo" {
		t.Errorf("expected name 'new-repo', got %q", r.Name)
	}
	if r.Visibility != "private" {
		t.Errorf("expected visibility 'private', got %q", r.Visibility)
	}
}

func TestCreateRepo_BadBody(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest("POST", "/api/v1/repo/repo", bytes.NewReader([]byte("bad")))
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.createRepo(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListFiles(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/repo/repo/repo-001/file", nil)
	req.SetPathValue("repoId", "repo-001")
	rec := httptest.NewRecorder()
	h.listFiles(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var files []RepoFile
	json.NewDecoder(rec.Body).Decode(&files)
	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}
}

func TestListPRs(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/repo/repo/repo-001/pr", nil)
	req.SetPathValue("repoId", "repo-001")
	rec := httptest.NewRecorder()
	h.listPRs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var page httputil.PageResponse
	json.NewDecoder(rec.Body).Decode(&page)
	if page.Total != 1 {
		t.Errorf("expected 1 PR, got %d", page.Total)
	}
}

func TestListPRs_Empty(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/repo/repo/repo-999/pr", nil)
	req.SetPathValue("repoId", "repo-999")
	rec := httptest.NewRecorder()
	h.listPRs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetPR_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/repo/repo/repo-001/pr/pr-001", nil)
	req.SetPathValue("repoId", "repo-001")
	req.SetPathValue("prId", "pr-001")
	rec := httptest.NewRecorder()
	h.getPR(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var pr PullRequest
	json.NewDecoder(rec.Body).Decode(&pr)
	if pr.Title != "Add user auth" {
		t.Errorf("expected title 'Add user auth', got %q", pr.Title)
	}
}

func TestGetPR_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("GET", "/api/v1/repo/repo/repo-001/pr/pr-999", nil)
	req.SetPathValue("repoId", "repo-001")
	req.SetPathValue("prId", "pr-999")
	rec := httptest.NewRecorder()
	h.getPR(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestMergePR_Happy(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/repo/repo/repo-001/pr/pr-001/merge", nil)
	req.SetPathValue("repoId", "repo-001")
	req.SetPathValue("prId", "pr-001")
	rec := httptest.NewRecorder()
	h.mergePR(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var pr PullRequest
	json.NewDecoder(rec.Body).Decode(&pr)
	if pr.Status != "merged" {
		t.Errorf("expected status 'merged', got %q", pr.Status)
	}
}

func TestRegister(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/repo/repo"},
		{"POST", "/api/v1/repo/repo"},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tt.method, tt.path, nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401, got %d", tt.method, tt.path, rec.Code)
		}
	}
}

func TestMergePR_NotFound(t *testing.T) {
	h := newHandler()
	req := newAuthRequest("POST", "/api/v1/repo/repo/repo-001/pr/pr-999/merge", nil)
	req.SetPathValue("repoId", "repo-001")
	req.SetPathValue("prId", "pr-999")
	rec := httptest.NewRecorder()
	h.mergePR(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
