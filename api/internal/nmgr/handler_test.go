package nmgr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/types"
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

func TestList_Empty(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("GET", "/api/v1/nmgr/node", nil)
	rec := httptest.NewRecorder()
	h.list(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var page httputil.PageResponse
	json.NewDecoder(rec.Body).Decode(&page)
	if page.Total != 0 {
		t.Errorf("expected 0 nodes, got %d", page.Total)
	}
}

func TestList_WithNodes(t *testing.T) {
	h := newTestHandler()
	h.store.Create(Node{ID: "node-001", IP: "192.168.1.1", Status: "Ready"})
	h.store.Create(Node{ID: "node-002", IP: "192.168.1.2", Status: "Ready"})

	req := newAuthRequest("GET", "/api/v1/nmgr/node", nil)
	rec := httptest.NewRecorder()
	h.list(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var page httputil.PageResponse
	json.NewDecoder(rec.Body).Decode(&page)
	if page.Total != 2 {
		t.Errorf("expected 2 nodes, got %d", page.Total)
	}
}

func TestCreate_Happy(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/nmgr/node", map[string]interface{}{
		"name":     "test-node",
		"host":     "192.168.1.10",
		"user":     "convocate",
		"location": "rack-1",
		"tags":     []string{"gpu"},
	})
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreate_MissingHost(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/nmgr/node", map[string]string{
		"user": "convocate",
	})
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreate_MissingUser(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/nmgr/node", map[string]string{
		"host": "192.168.1.10",
	})
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreate_InvalidNodeName(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/nmgr/node", map[string]string{
		"name": "INVALID_NAME",
		"host": "192.168.1.10",
		"user": "convocate",
	})
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreate_NodeNameLeadingHyphen(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/nmgr/node", map[string]string{
		"name": "-bad-name",
		"host": "192.168.1.10",
		"user": "convocate",
	})
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreate_NodeNameTrailingHyphen(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/nmgr/node", map[string]string{
		"name": "bad-name-",
		"host": "192.168.1.10",
		"user": "convocate",
	})
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreate_DuplicateName(t *testing.T) {
	h := newTestHandler()
	h.store.Create(Node{ID: "existing-node", IP: "192.168.1.1"})

	req := newAuthRequest("POST", "/api/v1/nmgr/node", map[string]string{
		"name": "existing-node",
		"host": "192.168.1.20",
		"user": "convocate",
	})
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestCreate_DuplicateIP(t *testing.T) {
	h := newTestHandler()
	h.store.Create(Node{ID: "existing-node", IP: "192.168.1.1"})

	req := newAuthRequest("POST", "/api/v1/nmgr/node", map[string]string{
		"name": "new-node",
		"host": "192.168.1.1",
		"user": "convocate",
	})
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestCreate_BadBody(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest("POST", "/api/v1/nmgr/node", bytes.NewReader([]byte("bad")))
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGet_Happy(t *testing.T) {
	h := newTestHandler()
	h.store.Create(Node{ID: "node-001", IP: "192.168.1.1", Status: "Ready"})

	req := newAuthRequest("GET", "/api/v1/nmgr/node/node-001", nil)
	req.SetPathValue("nodeId", "node-001")
	rec := httptest.NewRecorder()
	h.get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGet_NotFound(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("GET", "/api/v1/nmgr/node/nonexistent", nil)
	req.SetPathValue("nodeId", "nonexistent")
	rec := httptest.NewRecorder()
	h.get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdate_Happy(t *testing.T) {
	h := newTestHandler()
	h.store.Create(Node{ID: "node-001", IP: "192.168.1.1", Status: "Ready"})

	loc := "rack-2"
	req := newAuthRequest("PATCH", "/api/v1/nmgr/node/node-001", map[string]interface{}{
		"location": &loc,
		"tags":     []string{"gpu", "ssd"},
	})
	req.SetPathValue("nodeId", "node-001")
	rec := httptest.NewRecorder()
	h.update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("PATCH", "/api/v1/nmgr/node/nonexistent", map[string]interface{}{})
	req.SetPathValue("nodeId", "nonexistent")
	rec := httptest.NewRecorder()
	h.update(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdate_BadBody(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest("PATCH", "/api/v1/nmgr/node/node-001", bytes.NewReader([]byte("bad")))
	req.SetPathValue("nodeId", "node-001")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDelete_Happy(t *testing.T) {
	h := newTestHandler()
	h.store.Create(Node{ID: "node-001", IP: "192.168.1.1", Status: "Pending"})

	req := newAuthRequest("DELETE", "/api/v1/nmgr/node/node-001", nil)
	req.SetPathValue("nodeId", "node-001")
	rec := httptest.NewRecorder()
	h.del(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func TestDelete_NotFound(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("DELETE", "/api/v1/nmgr/node/nonexistent", nil)
	req.SetPathValue("nodeId", "nonexistent")
	rec := httptest.NewRecorder()
	h.del(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestStart_Happy(t *testing.T) {
	h := newTestHandler()
	h.store.Create(Node{ID: "node-001", IP: "192.168.1.1", Status: "SchedulingDisabled"})

	req := newAuthRequest("POST", "/api/v1/nmgr/node/node-001/start", nil)
	req.SetPathValue("nodeId", "node-001")
	rec := httptest.NewRecorder()
	h.start(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func TestStart_NotFound(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/nmgr/node/nonexistent/start", nil)
	req.SetPathValue("nodeId", "nonexistent")
	rec := httptest.NewRecorder()
	h.start(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestStop_InsufficientNodes(t *testing.T) {
	h := newTestHandler()
	// Only 1 Ready node — below minimum of 4
	h.store.Create(Node{ID: "node-001", IP: "192.168.1.1", Status: "Ready"})

	req := newAuthRequest("POST", "/api/v1/nmgr/node/node-001/stop", nil)
	req.SetPathValue("nodeId", "node-001")
	rec := httptest.NewRecorder()
	h.stop(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestStop_Happy(t *testing.T) {
	h := newTestHandler()
	// Need at least minReadyNodes (4) Ready nodes to allow stop
	for i := 0; i < 5; i++ {
		h.store.Create(Node{ID: fmt.Sprintf("node-%03d", i+1), IP: fmt.Sprintf("192.168.1.%d", i+1), Status: "Ready"})
	}

	req := newAuthRequest("POST", "/api/v1/nmgr/node/node-001/stop", nil)
	req.SetPathValue("nodeId", "node-001")
	rec := httptest.NewRecorder()
	h.stop(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func TestStop_NotFound(t *testing.T) {
	h := newTestHandler()
	// Need enough nodes to pass the safety check
	for i := 0; i < 5; i++ {
		h.store.Create(Node{ID: fmt.Sprintf("node-%03d", i+1), IP: fmt.Sprintf("192.168.1.%d", i+1), Status: "Ready"})
	}

	req := newAuthRequest("POST", "/api/v1/nmgr/node/nonexistent/stop", nil)
	req.SetPathValue("nodeId", "nonexistent")
	rec := httptest.NewRecorder()
	h.stop(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestListNotes_Empty(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("GET", "/api/v1/nmgr/node/node-001/note", nil)
	req.SetPathValue("nodeId", "node-001")
	rec := httptest.NewRecorder()
	h.listNotes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAddNote_Happy(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/nmgr/node/node-001/note", map[string]string{
		"text": "Disk replaced",
	})
	req.SetPathValue("nodeId", "node-001")
	rec := httptest.NewRecorder()
	h.addNote(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var note Note
	json.NewDecoder(rec.Body).Decode(&note)
	if note.Text != "Disk replaced" {
		t.Errorf("expected text 'Disk replaced', got %q", note.Text)
	}
	if note.Author != "admin" {
		t.Errorf("expected author 'admin', got %q", note.Author)
	}
}

func TestAddNote_EmptyText(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/nmgr/node/node-001/note", map[string]string{
		"text": "",
	})
	req.SetPathValue("nodeId", "node-001")
	rec := httptest.NewRecorder()
	h.addNote(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAddNote_BadBody(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest("POST", "/api/v1/nmgr/node/node-001/note", bytes.NewReader([]byte("bad")))
	req.SetPathValue("nodeId", "node-001")
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Username: "admin", Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.addNote(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestIngestMetrics_Happy(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/nmgr/metrics", types.NodeMetricsReport{
		NodeName:      "node-001",
		MemUsedBytes:  1024 * 1024 * 1024,
		MemTotalBytes: 8 * 1024 * 1024 * 1024,
	})
	rec := httptest.NewRecorder()
	h.ingestMetrics(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestIngestMetrics_MissingNodeName(t *testing.T) {
	h := newTestHandler()
	req := newAuthRequest("POST", "/api/v1/nmgr/metrics", types.NodeMetricsReport{})
	rec := httptest.NewRecorder()
	h.ingestMetrics(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestIngestMetrics_BadBody(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest("POST", "/api/v1/nmgr/metrics", bytes.NewReader([]byte("bad")))
	ctx := httputil.ContextWithPrincipal(req.Context(), &httputil.Principal{Roles: []string{"admin"}})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ingestMetrics(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestMergeNodeMetrics(t *testing.T) {
	h := newTestHandler()
	// Store a metrics entry
	h.nodeMetrics.Store("node-001", metricsEntry{
		report: types.NodeMetricsReport{
			NodeName:       "node-001",
			MemUsedBytes:   2 * 1024 * 1024 * 1024,
			MemTotalBytes:  8 * 1024 * 1024 * 1024,
			DiskUsedBytes:  50 * 1024 * 1024 * 1024,
			DiskTotalBytes: 100 * 1024 * 1024 * 1024,
			UptimeSeconds:  3600,
			KubeletVersion: "v1.30.0",
			CPUCount:       4,
		},
		received: time.Now(),
	})

	node := &types.Node{ID: "node-001"}
	h.mergeNodeMetrics(node)

	if node.MemTotalGB == 0 {
		t.Error("expected MemTotalGB to be set")
	}
	if node.DiskTotalGB == 0 {
		t.Error("expected DiskTotalGB to be set")
	}
	if node.UptimeSeconds != 3600 {
		t.Errorf("expected UptimeSeconds 3600, got %d", node.UptimeSeconds)
	}
	if node.KubeletVersion != "v1.30.0" {
		t.Errorf("expected KubeletVersion 'v1.30.0', got %q", node.KubeletVersion)
	}
	if node.CPUCount != 4 {
		t.Errorf("expected CPUCount 4, got %d", node.CPUCount)
	}
}

func TestMergeNodeMetrics_NoData(t *testing.T) {
	h := newTestHandler()
	node := &types.Node{ID: "node-no-metrics"}
	h.mergeNodeMetrics(node) // should not panic
	if node.MemTotalGB != 0 {
		t.Error("expected no metrics to be merged")
	}
}

func TestMergeNodeMetrics_Stale(t *testing.T) {
	h := newTestHandler()
	h.nodeMetrics.Store("node-stale", metricsEntry{
		report: types.NodeMetricsReport{
			NodeName:      "node-stale",
			MemTotalBytes: 8 * 1024 * 1024 * 1024,
		},
		received: time.Now().Add(-20 * time.Second), // stale
	})
	node := &types.Node{ID: "node-stale"}
	h.mergeNodeMetrics(node)
	if node.MemTotalGB != 0 {
		t.Error("expected stale data to be skipped")
	}
}

func TestMergeNodeMetrics_SwapAndDisk(t *testing.T) {
	h := newTestHandler()
	h.nodeMetrics.Store("node-full", metricsEntry{
		report: types.NodeMetricsReport{
			NodeName:       "node-full",
			MemUsedBytes:   2 * 1024 * 1024 * 1024,
			MemTotalBytes:  8 * 1024 * 1024 * 1024,
			SwapUsedBytes:  1 * 1024 * 1024 * 1024,
			SwapTotalBytes: 4 * 1024 * 1024 * 1024,
			DiskUsedBytes:  50 * 1024 * 1024 * 1024,
			DiskTotalBytes: 100 * 1024 * 1024 * 1024,
			UptimeSeconds:  7200,
			KubeletVersion: "v1.30.1",
			CPUCount:       8,
		},
		received: time.Now(),
	})
	node := &types.Node{ID: "node-full"}
	h.mergeNodeMetrics(node)
	if node.SwapTotalGB == 0 {
		t.Error("expected SwapTotalGB to be set")
	}
	if node.SwapUsedGB == 0 {
		t.Error("expected SwapUsedGB to be set")
	}
	if node.CPUCount != 8 {
		t.Errorf("expected CPUCount 8, got %d", node.CPUCount)
	}
}

func TestGetNotesFromDB_MockMode(t *testing.T) {
	h := newTestHandler()
	// db.Pool is nil, so should use store
	h.store.AddNote("node-001", Note{Author: "admin", Text: "test note"})

	notes := h.getNotesFromDB("node-001")
	if len(notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(notes))
	}
	if notes[0].Text != "test note" {
		t.Errorf("expected text 'test note', got %q", notes[0].Text)
	}
}

func TestGetNotesFromDB_Empty(t *testing.T) {
	h := newTestHandler()
	notes := h.getNotesFromDB("node-empty")
	if len(notes) != 0 {
		t.Errorf("expected 0 notes, got %d", len(notes))
	}
}

func TestJitterMetrics(t *testing.T) {
	s := NewStore()
	s.Create(Node{ID: "node-001", IP: "1.2.3.4", Status: "Ready",
		LoadAvg:     LoadAvg{One: 1.0, Five: 1.0, Fifteen: 1.0},
		MemUsedGB:   4.0, MemTotalGB: 16.0,
		DiskUsedGB:  50.0, DiskTotalGB: 200.0})

	s.JitterMetrics()
	nodes := s.List()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	// Just verify it didn't panic and values are still reasonable
	if nodes[0].MemUsedGB < 0 || nodes[0].MemUsedGB > nodes[0].MemTotalGB {
		t.Errorf("MemUsedGB out of range: %f", nodes[0].MemUsedGB)
	}
}

func TestJitterMetrics_NonReady(t *testing.T) {
	s := NewStore()
	s.Create(Node{ID: "node-001", IP: "1.2.3.4", Status: "Pending",
		LoadAvg: LoadAvg{One: 1.0, Five: 1.0, Fifteen: 1.0}})

	original := s.List()[0].LoadAvg
	s.JitterMetrics()
	after := s.List()[0].LoadAvg
	// Non-Ready nodes should not be jittered
	if original.One != after.One {
		t.Error("non-Ready node should not be jittered")
	}
}

func TestCountReadyNodes(t *testing.T) {
	h := newTestHandler()
	h.store.Create(Node{ID: "n1", IP: "1.1.1.1", Status: "Ready"})
	h.store.Create(Node{ID: "n2", IP: "1.1.1.2", Status: "Ready"})
	h.store.Create(Node{ID: "n3", IP: "1.1.1.3", Status: "Pending"})

	count := h.countReadyNodes(nil)
	if count != 2 {
		t.Errorf("expected 2 Ready nodes, got %d", count)
	}
}

func TestCreate_HostTooLong(t *testing.T) {
	h := newTestHandler()
	longHost := strings.Repeat("a", 254)
	req := newAuthRequest("POST", "/api/v1/nmgr/node", map[string]string{
		"host": longHost,
		"user": "convocate",
	})
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreate_NodeNameTooLong(t *testing.T) {
	h := newTestHandler()
	longName := strings.Repeat("a", 64)
	req := newAuthRequest("POST", "/api/v1/nmgr/node", map[string]string{
		"name": longName,
		"host": "192.168.1.10",
		"user": "convocate",
	})
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestStoreCreate_AutoID(t *testing.T) {
	s := NewStore()
	n := s.Create(Node{IP: "1.2.3.4"})
	if n.ID == "" {
		t.Error("expected auto-generated ID")
	}
	if n.Status != "Pending" {
		t.Errorf("expected status Pending, got %q", n.Status)
	}
}

func TestStoreUpdate_Tags(t *testing.T) {
	s := NewStore()
	s.Create(Node{ID: "n1", IP: "1.2.3.4"})

	loc := "rack-1"
	n, ok := s.Update("n1", &loc, []string{"gpu"})
	if !ok {
		t.Fatal("expected ok=true")
	}
	if n.Location != "rack-1" {
		t.Errorf("expected location 'rack-1', got %q", n.Location)
	}
	if len(n.Tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(n.Tags))
	}
}

func TestStoreUpdate_NotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.Update("nonexistent", nil, nil)
	if ok {
		t.Error("expected ok=false")
	}
}

func TestRegister(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/nmgr/node", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestListNotes_WithNotes(t *testing.T) {
	h := newTestHandler()
	h.store.AddNote("node-001", Note{Author: "admin", Text: "note 1"})
	h.store.AddNote("node-001", Note{Author: "admin", Text: "note 2"})

	req := newAuthRequest("GET", "/api/v1/nmgr/node/node-001/note", nil)
	req.SetPathValue("nodeId", "node-001")
	rec := httptest.NewRecorder()
	h.listNotes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAddNote_NoPrincipal(t *testing.T) {
	h := newTestHandler()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{"text": "note text"})
	req := httptest.NewRequest("POST", "/api/v1/nmgr/node/node-001/note", &buf)
	req.SetPathValue("nodeId", "node-001")
	req.Header.Set("Content-Type", "application/json")
	// No principal in context
	rec := httptest.NewRecorder()
	h.addNote(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
	var note Note
	json.NewDecoder(rec.Body).Decode(&note)
	if note.Author != "system" {
		t.Errorf("expected author 'system', got %q", note.Author)
	}
}

func TestStoreSetStatus(t *testing.T) {
	s := NewStore()
	s.Create(Node{ID: "n1", IP: "1.2.3.4"})
	if !s.SetStatus("n1", "Ready") {
		t.Error("expected true")
	}
	n, _ := s.Get("n1")
	if n.Status != "Ready" {
		t.Errorf("expected status 'Ready', got %q", n.Status)
	}
}

func TestStoreSetStatus_NotFound(t *testing.T) {
	s := NewStore()
	if s.SetStatus("nonexistent", "Ready") {
		t.Error("expected false")
	}
}

func TestStoreDelete(t *testing.T) {
	s := NewStore()
	s.Create(Node{ID: "n1", IP: "1.2.3.4"})
	if !s.Delete("n1") {
		t.Error("expected true")
	}
	if s.Delete("n1") {
		t.Error("expected false after deletion")
	}
}

func TestStoreAddNote(t *testing.T) {
	s := NewStore()
	note := s.AddNote("n1", Note{Author: "admin", Text: "hello"})
	if note.CreatedAt == "" {
		t.Error("expected createdAt to be set")
	}
	notes := s.ListNotes("n1")
	if len(notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(notes))
	}
}

func TestStoreGet(t *testing.T) {
	s := NewStore()
	_, ok := s.Get("nonexistent")
	if ok {
		t.Error("expected ok=false")
	}
	s.Create(Node{ID: "n1", IP: "1.2.3.4"})
	n, ok := s.Get("n1")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if n.IP != "1.2.3.4" {
		t.Errorf("expected IP '1.2.3.4', got %q", n.IP)
	}
}
