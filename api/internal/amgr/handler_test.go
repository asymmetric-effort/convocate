package amgr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/types"
)

// ---------------------------------------------------------------------------
// Mock AgentManager
// ---------------------------------------------------------------------------

type mockAgentManager struct {
	listFn            func(ctx context.Context) ([]types.Agent, error)
	getFn             func(ctx context.Context, name string) (*types.Agent, error)
	createFn          func(ctx context.Context, req types.CreateAgentRequest, owner string) (*types.Agent, error)
	deleteFn          func(ctx context.Context, name string) error
	stopFn            func(ctx context.Context, name string) error
	updateConfigMapFn func(ctx context.Context, podName, claudeMd string) error
	getIPFn           func(ctx context.Context, name string) (string, error)
}

func (m *mockAgentManager) ListAgentPods(ctx context.Context) ([]types.Agent, error) {
	return m.listFn(ctx)
}
func (m *mockAgentManager) GetAgentPod(ctx context.Context, name string) (*types.Agent, error) {
	return m.getFn(ctx, name)
}
func (m *mockAgentManager) CreateAgentPod(ctx context.Context, req types.CreateAgentRequest, owner string) (*types.Agent, error) {
	return m.createFn(ctx, req, owner)
}
func (m *mockAgentManager) DeleteAgentPod(ctx context.Context, name string) error {
	return m.deleteFn(ctx, name)
}
func (m *mockAgentManager) StopAgentPod(ctx context.Context, name string) error {
	return m.stopFn(ctx, name)
}
func (m *mockAgentManager) UpdateAgentConfigMap(ctx context.Context, podName, claudeMd string) error {
	return m.updateConfigMapFn(ctx, podName, claudeMd)
}
func (m *mockAgentManager) GetAgentPodIP(ctx context.Context, name string) (string, error) {
	return m.getIPFn(ctx, name)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

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

func newNonAdminRequest(method, path string, body interface{}) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{
		ID: "usr-002", Username: "user", Roles: []string{"agent-update"},
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

// newK8sTestHandler creates a Handler with useK8s=true and a mock manager.
func newK8sTestHandler(mgr *mockAgentManager) *Handler {
	return &Handler{
		store:   NewStore(),
		useK8s:  true,
		saToken: "test-sa-token",
		mgr:     mgr,
	}
}

// ---------------------------------------------------------------------------
// Non-K8s tests (existing, preserved)
// ---------------------------------------------------------------------------

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
	req := newNonAdminRequest("POST", "/api/v1/amgr/agent", body)
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
	req := newNonAdminRequest("PATCH", "/api/v1/amgr/agent/agt-7f3a-01", body)
	req.SetPathValue("agentId", "agt-7f3a-01")
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

// ---------------------------------------------------------------------------
// K8s mock tests — list
// ---------------------------------------------------------------------------

func TestListK8s_Happy(t *testing.T) {
	mgr := &mockAgentManager{
		listFn: func(ctx context.Context) ([]types.Agent, error) {
			return []types.Agent{
				{ID: "agent-a", Project: "proj-a", Status: "Running"},
				{ID: "agent-b", Project: "proj-b", Status: "Running"},
			}, nil
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("GET", "/api/v1/amgr/agent", nil)
	rec := httptest.NewRecorder()
	h.list(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var page httputil.PageResponse
	json.NewDecoder(rec.Body).Decode(&page)
	if page.Total != 2 {
		t.Errorf("expected 2 agents, got %d", page.Total)
	}
}

func TestListK8s_Error(t *testing.T) {
	mgr := &mockAgentManager{
		listFn: func(ctx context.Context) ([]types.Agent, error) {
			return nil, fmt.Errorf("k8s unavailable")
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("GET", "/api/v1/amgr/agent", nil)
	rec := httptest.NewRecorder()
	h.list(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// K8s mock tests — create
// ---------------------------------------------------------------------------

func TestCreateK8s_Happy(t *testing.T) {
	mgr := &mockAgentManager{
		createFn: func(ctx context.Context, req types.CreateAgentRequest, owner string) (*types.Agent, error) {
			return &types.Agent{ID: "agent-test", Project: req.Project, Status: "Running"}, nil
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("POST", "/api/v1/amgr/agent", map[string]string{
		"project": "test-project",
	})
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateK8s_Error(t *testing.T) {
	mgr := &mockAgentManager{
		createFn: func(ctx context.Context, req types.CreateAgentRequest, owner string) (*types.Agent, error) {
			return nil, fmt.Errorf("pod creation failed")
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("POST", "/api/v1/amgr/agent", map[string]string{
		"project": "test-project",
	})
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateK8s_NoPrincipal(t *testing.T) {
	mgr := &mockAgentManager{
		createFn: func(ctx context.Context, req types.CreateAgentRequest, owner string) (*types.Agent, error) {
			if owner != "system" {
				t.Errorf("expected owner 'system', got %q", owner)
			}
			return &types.Agent{ID: "agent-test", Project: req.Project, Status: "Running"}, nil
		},
	}
	h := newK8sTestHandler(mgr)
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"project": "test"})
	req := httptest.NewRequest("POST", "/api/v1/amgr/agent", &buf)
	req.Header.Set("Content-Type", "application/json")
	// No principal in context
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateK8s_SecurityNonAdmin(t *testing.T) {
	mgr := &mockAgentManager{}
	h := newK8sTestHandler(mgr)
	body := map[string]interface{}{
		"project":  "test",
		"security": map[string]interface{}{"dockerAccess": true},
	}
	req := newNonAdminRequest("POST", "/api/v1/amgr/agent", body)
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestCreateK8s_SecurityNoPrincipal(t *testing.T) {
	mgr := &mockAgentManager{}
	h := newK8sTestHandler(mgr)
	body := map[string]interface{}{
		"project":  "test",
		"security": map[string]interface{}{"dockerAccess": true},
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest("POST", "/api/v1/amgr/agent", &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// K8s mock tests — get
// ---------------------------------------------------------------------------

func TestGetK8s_Happy(t *testing.T) {
	mgr := &mockAgentManager{
		getFn: func(ctx context.Context, name string) (*types.Agent, error) {
			return &types.Agent{ID: name, Project: "proj-a", Status: "Running"}, nil
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("GET", "/api/v1/amgr/agent/agent-a", nil)
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetK8s_NotFound(t *testing.T) {
	mgr := &mockAgentManager{
		getFn: func(ctx context.Context, name string) (*types.Agent, error) {
			return nil, fmt.Errorf("not found")
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("GET", "/api/v1/amgr/agent/nonexistent", nil)
	req.SetPathValue("agentId", "nonexistent")
	rec := httptest.NewRecorder()
	h.get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// K8s mock tests — update
// ---------------------------------------------------------------------------

func TestUpdateK8s_Happy(t *testing.T) {
	mgr := &mockAgentManager{
		updateConfigMapFn: func(ctx context.Context, podName, claudeMd string) error {
			return nil
		},
		getFn: func(ctx context.Context, name string) (*types.Agent, error) {
			return &types.Agent{ID: name, Project: "proj-a", Status: "Running"}, nil
		},
	}
	h := newK8sTestHandler(mgr)
	claudeMd := "# test"
	req := newAuthRequest("PATCH", "/api/v1/amgr/agent/agent-a", map[string]interface{}{
		"claudeMd": &claudeMd,
	})
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateK8s_ConfigMapError(t *testing.T) {
	mgr := &mockAgentManager{
		updateConfigMapFn: func(ctx context.Context, podName, claudeMd string) error {
			return fmt.Errorf("configmap update failed")
		},
	}
	h := newK8sTestHandler(mgr)
	claudeMd := "# test"
	req := newAuthRequest("PATCH", "/api/v1/amgr/agent/agent-a", map[string]interface{}{
		"claudeMd": &claudeMd,
	})
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.update(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestUpdateK8s_NoClaudeMd(t *testing.T) {
	mgr := &mockAgentManager{
		getFn: func(ctx context.Context, name string) (*types.Agent, error) {
			return &types.Agent{ID: name, Project: "proj-a", Status: "Running"}, nil
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("PATCH", "/api/v1/amgr/agent/agent-a", map[string]interface{}{
		"project": "new-proj",
	})
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestUpdateK8s_GetNotFound(t *testing.T) {
	mgr := &mockAgentManager{
		getFn: func(ctx context.Context, name string) (*types.Agent, error) {
			return nil, fmt.Errorf("not found")
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("PATCH", "/api/v1/amgr/agent/nonexistent", map[string]interface{}{})
	req.SetPathValue("agentId", "nonexistent")
	rec := httptest.NewRecorder()
	h.update(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateK8s_SecurityNonAdmin(t *testing.T) {
	mgr := &mockAgentManager{}
	h := newK8sTestHandler(mgr)
	body := map[string]interface{}{
		"security": map[string]interface{}{"dockerAccess": true},
	}
	req := newNonAdminRequest("PATCH", "/api/v1/amgr/agent/agent-a", body)
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.update(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestUpdateK8s_SecurityNoPrincipal(t *testing.T) {
	mgr := &mockAgentManager{}
	h := newK8sTestHandler(mgr)
	body := map[string]interface{}{
		"security": map[string]interface{}{"dockerAccess": true},
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest("PATCH", "/api/v1/amgr/agent/agent-a", &buf)
	req.SetPathValue("agentId", "agent-a")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.update(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// K8s mock tests — delete
// ---------------------------------------------------------------------------

func TestDeleteK8s_Happy(t *testing.T) {
	mgr := &mockAgentManager{
		deleteFn: func(ctx context.Context, name string) error {
			return nil
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("DELETE", "/api/v1/amgr/agent/agent-a", nil)
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.del(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestDeleteK8s_NotFound(t *testing.T) {
	mgr := &mockAgentManager{
		deleteFn: func(ctx context.Context, name string) error {
			return fmt.Errorf("not found")
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("DELETE", "/api/v1/amgr/agent/nonexistent", nil)
	req.SetPathValue("agentId", "nonexistent")
	rec := httptest.NewRecorder()
	h.del(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// K8s mock tests — start
// ---------------------------------------------------------------------------

func TestStartK8s_Happy(t *testing.T) {
	mgr := &mockAgentManager{
		getFn: func(ctx context.Context, name string) (*types.Agent, error) {
			return nil, fmt.Errorf("not found") // pod doesn't exist yet
		},
		createFn: func(ctx context.Context, req types.CreateAgentRequest, owner string) (*types.Agent, error) {
			return &types.Agent{ID: "agent-myproj", Project: req.Project, Status: "Running"}, nil
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("POST", "/api/v1/amgr/agent/agent-myproj/start", nil)
	req.SetPathValue("agentId", "agent-myproj")
	rec := httptest.NewRecorder()
	h.start(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestStartK8s_AlreadyRunning(t *testing.T) {
	mgr := &mockAgentManager{
		getFn: func(ctx context.Context, name string) (*types.Agent, error) {
			return &types.Agent{ID: name, Status: "Running"}, nil // pod exists
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("POST", "/api/v1/amgr/agent/agent-a/start", nil)
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.start(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestStartK8s_CreateError(t *testing.T) {
	mgr := &mockAgentManager{
		getFn: func(ctx context.Context, name string) (*types.Agent, error) {
			return nil, fmt.Errorf("not found")
		},
		createFn: func(ctx context.Context, req types.CreateAgentRequest, owner string) (*types.Agent, error) {
			return nil, fmt.Errorf("creation failed")
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("POST", "/api/v1/amgr/agent/agent-a/start", nil)
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.start(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestStartK8s_NoPrincipal(t *testing.T) {
	mgr := &mockAgentManager{
		getFn: func(ctx context.Context, name string) (*types.Agent, error) {
			return nil, fmt.Errorf("not found")
		},
		createFn: func(ctx context.Context, req types.CreateAgentRequest, owner string) (*types.Agent, error) {
			if owner != "system" {
				t.Errorf("expected owner 'system', got %q", owner)
			}
			return &types.Agent{ID: "agent-test", Status: "Running"}, nil
		},
	}
	h := newK8sTestHandler(mgr)
	req := httptest.NewRequest("POST", "/api/v1/amgr/agent/agent-test/start", nil)
	req.SetPathValue("agentId", "agent-test")
	rec := httptest.NewRecorder()
	h.start(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestStartK8s_ShortId(t *testing.T) {
	// ID shorter than 6 chars — project should equal id
	mgr := &mockAgentManager{
		getFn: func(ctx context.Context, name string) (*types.Agent, error) {
			return nil, fmt.Errorf("not found")
		},
		createFn: func(ctx context.Context, req types.CreateAgentRequest, owner string) (*types.Agent, error) {
			if req.Project != "abc" {
				t.Errorf("expected project 'abc', got %q", req.Project)
			}
			return &types.Agent{ID: "abc", Status: "Running"}, nil
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("POST", "/api/v1/amgr/agent/abc/start", nil)
	req.SetPathValue("agentId", "abc")
	rec := httptest.NewRecorder()
	h.start(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// K8s mock tests — stop
// ---------------------------------------------------------------------------

func TestStopK8s_Happy(t *testing.T) {
	mgr := &mockAgentManager{
		stopFn: func(ctx context.Context, name string) error {
			return nil
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("POST", "/api/v1/amgr/agent/agent-a/stop", nil)
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.stop(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func TestStopK8s_NotFound(t *testing.T) {
	mgr := &mockAgentManager{
		stopFn: func(ctx context.Context, name string) error {
			return fmt.Errorf("not found")
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("POST", "/api/v1/amgr/agent/nonexistent/stop", nil)
	req.SetPathValue("agentId", "nonexistent")
	rec := httptest.NewRecorder()
	h.stop(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// K8s mock tests — proxy with mock IP
// ---------------------------------------------------------------------------

func newCancelledRequest(method, path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	ctx, cancel := context.WithCancel(req.Context())
	cancel() // immediately cancelled
	ctx = httputil.ContextWithPrincipal(ctx, &httputil.Principal{
		ID: "usr-001", Username: "admin", Roles: []string{"admin"},
	})
	return req.WithContext(ctx)
}

func TestProxyStdinK8s_UpstreamError(t *testing.T) {
	mgr := &mockAgentManager{
		getIPFn: func(ctx context.Context, name string) (string, error) {
			return "127.0.0.1", nil
		},
	}
	h := &Handler{
		store:   NewStore(),
		useK8s:  true,
		saToken: "test-token",
		mgr:     mgr,
	}
	req := newCancelledRequest("POST", "/api/v1/amgr/agent/agent-a/stdin", bytes.NewReader([]byte("hello")))
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.proxyStdin(rec, req)

	// Should get 502 because the context is cancelled
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestProxyStdinK8s_IPNotFound(t *testing.T) {
	mgr := &mockAgentManager{
		getIPFn: func(ctx context.Context, name string) (string, error) {
			return "", fmt.Errorf("agent not found")
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("POST", "/api/v1/amgr/agent/nonexistent/stdin", nil)
	req.SetPathValue("agentId", "nonexistent")
	rec := httptest.NewRecorder()
	h.proxyStdin(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestProxyStdoutK8s_IPNotFound(t *testing.T) {
	mgr := &mockAgentManager{
		getIPFn: func(ctx context.Context, name string) (string, error) {
			return "", fmt.Errorf("agent not found")
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("GET", "/api/v1/amgr/agent/nonexistent/stdout", nil)
	req.SetPathValue("agentId", "nonexistent")
	rec := httptest.NewRecorder()
	h.proxyStdout(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestProxyStderrK8s_IPNotFound(t *testing.T) {
	mgr := &mockAgentManager{
		getIPFn: func(ctx context.Context, name string) (string, error) {
			return "", fmt.Errorf("agent not found")
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("GET", "/api/v1/amgr/agent/nonexistent/stderr", nil)
	req.SetPathValue("agentId", "nonexistent")
	rec := httptest.NewRecorder()
	h.proxyStderr(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestProxyMetricsK8s_IPNotFound(t *testing.T) {
	mgr := &mockAgentManager{
		getIPFn: func(ctx context.Context, name string) (string, error) {
			return "", fmt.Errorf("agent not found")
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("GET", "/api/v1/amgr/agent/nonexistent/metrics", nil)
	req.SetPathValue("agentId", "nonexistent")
	rec := httptest.NewRecorder()
	h.proxyMetrics(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestProxyMetricsK8s_UpstreamError(t *testing.T) {
	mgr := &mockAgentManager{
		getIPFn: func(ctx context.Context, name string) (string, error) {
			return "127.0.0.1", nil
		},
	}
	h := newK8sTestHandler(mgr)
	req := newCancelledRequest("GET", "/api/v1/amgr/agent/agent-a/metrics", nil)
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.proxyMetrics(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestProxyStreamK8s_UpstreamError(t *testing.T) {
	mgr := &mockAgentManager{
		getIPFn: func(ctx context.Context, name string) (string, error) {
			return "127.0.0.1", nil
		},
	}
	h := newK8sTestHandler(mgr)
	req := newCancelledRequest("GET", "/api/v1/amgr/agent/agent-a/stdout", nil)
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.proxyStream(rec, req, "stdout")

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestProxyStreamK8s_UpstreamBadGateway(t *testing.T) {
	mgr := &mockAgentManager{
		getIPFn: func(ctx context.Context, name string) (string, error) {
			return "127.0.0.1", nil
		},
	}
	h := newK8sTestHandler(mgr)
	req := newCancelledRequest("GET", "/api/v1/amgr/agent/agent-a/stderr", nil)
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.proxyStream(rec, req, "stderr")

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestProxyStdinK8s_NoSAToken(t *testing.T) {
	mgr := &mockAgentManager{
		getIPFn: func(ctx context.Context, name string) (string, error) {
			return "127.0.0.1", nil
		},
	}
	h := &Handler{
		store:   NewStore(),
		useK8s:  true,
		saToken: "",
		mgr:     mgr,
	}
	req := newCancelledRequest("POST", "/api/v1/amgr/agent/agent-a/stdin", bytes.NewReader([]byte("hello")))
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.proxyStdin(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestProxyMetricsK8s_NoSAToken(t *testing.T) {
	mgr := &mockAgentManager{
		getIPFn: func(ctx context.Context, name string) (string, error) {
			return "127.0.0.1", nil
		},
	}
	h := &Handler{
		store:   NewStore(),
		useK8s:  true,
		saToken: "",
		mgr:     mgr,
	}
	req := newCancelledRequest("GET", "/api/v1/amgr/agent/agent-a/metrics", nil)
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.proxyMetrics(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestProxyStreamK8s_NoSAToken(t *testing.T) {
	mgr := &mockAgentManager{
		getIPFn: func(ctx context.Context, name string) (string, error) {
			return "127.0.0.1", nil
		},
	}
	h := &Handler{
		store:   NewStore(),
		useK8s:  true,
		saToken: "",
		mgr:     mgr,
	}
	req := newCancelledRequest("GET", "/api/v1/amgr/agent/agent-a/stdout", nil)
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.proxyStream(rec, req, "stdout")

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestProxyK8s_SuccessPaths(t *testing.T) {
	// Start a test server on port 8443 so the handler's hardcoded URL works
	mux := http.NewServeMux()
	mux.HandleFunc("/stdin", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"cpu":0.5}`))
	})
	mux.HandleFunc("/stdout", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: hello\n\n"))
	})

	ln, err := net.Listen("tcp", "127.0.0.1:8443")
	if err != nil {
		t.Skipf("cannot bind to port 8443: %v", err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()
	defer ln.Close()

	mgr := &mockAgentManager{
		getIPFn: func(ctx context.Context, name string) (string, error) {
			return "127.0.0.1", nil
		},
	}
	h := &Handler{store: NewStore(), useK8s: true, saToken: "test-token", mgr: mgr}

	t.Run("stdin", func(t *testing.T) {
		raw := httptest.NewRequest("POST", "/api/v1/amgr/agent/agent-a/stdin", bytes.NewReader([]byte("hello")))
		raw.SetPathValue("agentId", "agent-a")
		raw.Header.Set("Content-Type", "application/octet-stream")
		ctx := httputil.ContextWithPrincipal(raw.Context(), &httputil.Principal{
			ID: "usr-001", Username: "admin", Roles: []string{"admin"},
		})
		req := raw.WithContext(ctx)
		rec := httptest.NewRecorder()
		h.proxyStdin(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "hello" {
			t.Errorf("expected body 'hello', got %q", rec.Body.String())
		}
	})

	t.Run("metrics", func(t *testing.T) {
		req := newAuthRequest("GET", "/api/v1/amgr/agent/agent-a/metrics", nil)
		req.SetPathValue("agentId", "agent-a")
		rec := httptest.NewRecorder()
		h.proxyMetrics(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("stream", func(t *testing.T) {
		req := newAuthRequest("GET", "/api/v1/amgr/agent/agent-a/stdout", nil)
		req.SetPathValue("agentId", "agent-a")
		rec := httptest.NewRecorder()
		h.proxyStream(rec, req, "stdout")

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte("hello")) {
			t.Errorf("expected body to contain 'hello', got %q", rec.Body.String())
		}
	})
}

func TestCreateK8s_SecurityAdmin(t *testing.T) {
	mgr := &mockAgentManager{
		createFn: func(ctx context.Context, req types.CreateAgentRequest, owner string) (*types.Agent, error) {
			return &types.Agent{ID: "agent-test", Project: req.Project, Status: "Running"}, nil
		},
	}
	h := newK8sTestHandler(mgr)
	body := map[string]interface{}{
		"project":  "test",
		"security": map[string]interface{}{"dockerAccess": true},
	}
	req := newAuthRequest("POST", "/api/v1/amgr/agent", body) // admin role
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateK8s_SecurityAdmin(t *testing.T) {
	mgr := &mockAgentManager{
		getFn: func(ctx context.Context, name string) (*types.Agent, error) {
			return &types.Agent{ID: name, Project: "proj-a", Status: "Running"}, nil
		},
	}
	h := newK8sTestHandler(mgr)
	body := map[string]interface{}{
		"security": map[string]interface{}{"dockerAccess": true},
	}
	req := newAuthRequest("PATCH", "/api/v1/amgr/agent/agent-a", body) // admin role
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestShellK8s_Delegates(t *testing.T) {
	mgr := &mockAgentManager{
		getIPFn: func(ctx context.Context, name string) (string, error) {
			return "", fmt.Errorf("agent not found")
		},
	}
	h := newK8sTestHandler(mgr)
	req := newAuthRequest("GET", "/api/v1/amgr/agent/agent-a/shell", nil)
	req.SetPathValue("agentId", "agent-a")
	rec := httptest.NewRecorder()
	h.shell(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
