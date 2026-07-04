package amgr

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
		ID: "usr-001", Username: "admin", Roles: []string{"admin"},
	})
	return req.WithContext(ctx)
}

// newTestHandler creates a Handler with useK8s=false (in-memory store).
func newTestHandler() *Handler {
	return &Handler{
		store:  NewStore(),
		useK8s: false,
	}
}

func TestList(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("GET", "/api/v1/amgr/agent", nil)
	rec := httptest.NewRecorder()
	h.list(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var page httputil.PageResponse
	json.NewDecoder(rec.Body).Decode(&page)
	if page.Total != 3 {
		t.Errorf("expected 3 agents, got %d", page.Total)
	}
}

func TestCreate_Happy(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/amgr/agent", map[string]string{
		"project": "test-project",
		"nodeId":  "node-001",
	})
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var a Agent
	json.NewDecoder(rec.Body).Decode(&a)
	if a.Project != "test-project" {
		t.Errorf("expected project 'test-project', got %q", a.Project)
	}
	if a.Status != "running" {
		t.Errorf("expected status 'running', got %q", a.Status)
	}
}

func TestCreate_BadBody(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest("POST", "/api/v1/amgr/agent", bytes.NewReader([]byte("bad")))
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreate_SecurityNonAdmin(t *testing.T) {
	h := newTestHandler()
	body := map[string]interface{}{
		"project":  "test",
		"security": map[string]interface{}{"dockerAccess": true},
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest("POST", "/api/v1/amgr/agent", &buf)
	req.Header.Set("Content-Type", "application/json")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{
		ID: "usr-002", Username: "user", Roles: []string{"agent-update"},
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestGet_Happy(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("GET", "/api/v1/amgr/agent/agt-7f3a-01", nil)
	req.SetPathValue("agentId", "agt-7f3a-01")
	rec := httptest.NewRecorder()
	h.get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var a Agent
	json.NewDecoder(rec.Body).Decode(&a)
	if a.Project != "demo-app" {
		t.Errorf("expected project 'demo-app', got %q", a.Project)
	}
}

func TestGet_NotFound(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("GET", "/api/v1/amgr/agent/nonexistent", nil)
	req.SetPathValue("agentId", "nonexistent")
	rec := httptest.NewRecorder()
	h.get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdate_Happy(t *testing.T) {
	h := newTestHandler()
	project := "updated-project"
	req := newAuthRequest("PATCH", "/api/v1/amgr/agent/agt-7f3a-01", map[string]*string{
		"project": &project,
	})
	req.SetPathValue("agentId", "agt-7f3a-01")
	rec := httptest.NewRecorder()
	h.update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var a Agent
	json.NewDecoder(rec.Body).Decode(&a)
	if a.Project != "updated-project" {
		t.Errorf("expected project 'updated-project', got %q", a.Project)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	h := newTestHandler()
	project := "x"
	req := newAuthRequest("PATCH", "/api/v1/amgr/agent/nonexistent", map[string]*string{
		"project": &project,
	})
	req.SetPathValue("agentId", "nonexistent")
	rec := httptest.NewRecorder()
	h.update(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdate_BadBody(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest("PATCH", "/api/v1/amgr/agent/agt-7f3a-01", bytes.NewReader([]byte("bad")))
	req.SetPathValue("agentId", "agt-7f3a-01")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdate_SecurityNonAdmin(t *testing.T) {
	h := newTestHandler()
	body := map[string]interface{}{
		"security": map[string]interface{}{"dockerAccess": true},
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest("PATCH", "/api/v1/amgr/agent/agt-7f3a-01", &buf)
	req.SetPathValue("agentId", "agt-7f3a-01")
	req.Header.Set("Content-Type", "application/json")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{
		ID: "usr-002", Username: "user", Roles: []string{"agent-update"},
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.update(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestDelete_Happy(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("DELETE", "/api/v1/amgr/agent/agt-7f3a-01", nil)
	req.SetPathValue("agentId", "agt-7f3a-01")
	rec := httptest.NewRecorder()
	h.del(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestDelete_NotFound(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("DELETE", "/api/v1/amgr/agent/nonexistent", nil)
	req.SetPathValue("agentId", "nonexistent")
	rec := httptest.NewRecorder()
	h.del(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestStart_Happy(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/amgr/agent/agt-7f3a-01/start", nil)
	req.SetPathValue("agentId", "agt-7f3a-01")
	rec := httptest.NewRecorder()
	h.start(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func TestStart_NotFound(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/amgr/agent/nonexistent/start", nil)
	req.SetPathValue("agentId", "nonexistent")
	rec := httptest.NewRecorder()
	h.start(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestStop_Happy(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/amgr/agent/agt-7f3a-01/stop", nil)
	req.SetPathValue("agentId", "agt-7f3a-01")
	rec := httptest.NewRecorder()
	h.stop(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func TestStop_NotFound(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/amgr/agent/nonexistent/stop", nil)
	req.SetPathValue("agentId", "nonexistent")
	rec := httptest.NewRecorder()
	h.stop(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestShell_NonK8s(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("GET", "/api/v1/amgr/agent/agt-7f3a-01/shell", nil)
	req.SetPathValue("agentId", "agt-7f3a-01")
	rec := httptest.NewRecorder()
	h.shell(rec, req)

	// In non-K8s mode, shell delegates to proxyStdout which calls getAgentPodIP
	// which returns an error in non-K8s mode
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (no K8s), got %d", rec.Code)
	}
}

func TestProxyStdin_NonK8s(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/amgr/agent/agt-7f3a-01/stdin", nil)
	req.SetPathValue("agentId", "agt-7f3a-01")
	rec := httptest.NewRecorder()
	h.proxyStdin(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (no K8s), got %d", rec.Code)
	}
}

func TestProxyStdout_NonK8s(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("GET", "/api/v1/amgr/agent/agt-7f3a-01/stdout", nil)
	req.SetPathValue("agentId", "agt-7f3a-01")
	rec := httptest.NewRecorder()
	h.proxyStdout(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (no K8s), got %d", rec.Code)
	}
}

func TestProxyStderr_NonK8s(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("GET", "/api/v1/amgr/agent/agt-7f3a-01/stderr", nil)
	req.SetPathValue("agentId", "agt-7f3a-01")
	rec := httptest.NewRecorder()
	h.proxyStderr(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (no K8s), got %d", rec.Code)
	}
}

func TestRegister(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/amgr/agent", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestProxyMetrics_NonK8s(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("GET", "/api/v1/amgr/agent/agt-7f3a-01/metrics", nil)
	req.SetPathValue("agentId", "agt-7f3a-01")
	rec := httptest.NewRecorder()
	h.proxyMetrics(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (no K8s), got %d", rec.Code)
	}
}
