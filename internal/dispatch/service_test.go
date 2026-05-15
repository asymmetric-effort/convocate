package dispatch

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/protocol"
	redispkg "github.com/asymmetric-effort/convocate/internal/redis"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// mockContainerManager is a test double for ContainerManager.
type mockContainerManager struct {
	mu              sync.Mutex
	deliveredPrompts map[string][]string // containerID -> prompts
	healthy         map[string]bool
	stopped         map[string]bool
	terminated      map[string][]uuid.UUID
	containerCount  int
	cpuPercent      float64
	memPercent      float64
}

func newMockContainerManager() *mockContainerManager {
	return &mockContainerManager{
		deliveredPrompts: make(map[string][]string),
		healthy:          make(map[string]bool),
		stopped:          make(map[string]bool),
		terminated:       make(map[string][]uuid.UUID),
		containerCount:   1,
		cpuPercent:       30.0,
		memPercent:       50.0,
	}
}

func (m *mockContainerManager) DeliverPrompt(containerID string, jobID uuid.UUID, prompt string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deliveredPrompts[containerID] = append(m.deliveredPrompts[containerID], prompt)
	return nil
}

func (m *mockContainerManager) StopContainer(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped[containerID] = true
	return nil
}

func (m *mockContainerManager) ProvisionContainer(containerID string, projectID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthy[containerID] = true
	return nil
}

func (m *mockContainerManager) IsHealthy(containerID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.healthy[containerID]
}

func (m *mockContainerManager) TerminateTask(containerID string, jobID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.terminated[containerID] = append(m.terminated[containerID], jobID)
	return nil
}

func (m *mockContainerManager) ContainerCount() int     { return m.containerCount }
func (m *mockContainerManager) CPUPercent() float64      { return m.cpuPercent }
func (m *mockContainerManager) MemoryPercent() float64   { return m.memPercent }

func (m *mockContainerManager) getPrompts(containerID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deliveredPrompts[containerID]
}

// mockRouterAPI captures status and heartbeat posts.
type mockRouterAPI struct {
	mu         sync.Mutex
	statuses   []protocol.StatusTransitionRequest
	heartbeats []protocol.HeartbeatRequest
}

func newMockRouterAPI() (*mockRouterAPI, *httptest.Server) {
	mock := &mockRouterAPI{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/v1/status":
			var req protocol.StatusTransitionRequest
			json.Unmarshal(body, &req)
			mock.mu.Lock()
			mock.statuses = append(mock.statuses, req)
			mock.mu.Unlock()
			json.NewEncoder(w).Encode(protocol.StatusTransitionResponse{Accepted: true})
		case "/v1/heartbeat":
			var req protocol.HeartbeatRequest
			json.Unmarshal(body, &req)
			mock.mu.Lock()
			mock.heartbeats = append(mock.heartbeats, req)
			mock.mu.Unlock()
			json.NewEncoder(w).Encode(protocol.HeartbeatResponse{Accepted: true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return mock, server
}

func testService(t *testing.T) (*Service, *mockContainerManager, *mockRouterAPI) {
	t.Helper()

	mockCM := newMockContainerManager()
	mockCM.healthy["container-abc"] = true

	mockRouter, routerServer := newMockRouterAPI()
	t.Cleanup(routerServer.Close)

	mockConn := redispkg.NewMockConn()
	store := redispkg.NewDispatchStore(mockConn, "test-host-1")

	svc, err := NewService(Config{
		HostID:       "test-host-1",
		ControlURL:   routerServer.URL,
		Store:        store,
		ContainerMgr: mockCM,
		Logger:       log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}
	t.Cleanup(svc.Stop)

	return svc, mockCM, mockRouter
}

func TestNewServiceValidation(t *testing.T) {
	mockConn := redispkg.NewMockConn()
	store := redispkg.NewDispatchStore(mockConn, "h1")
	cm := newMockContainerManager()

	tests := []struct {
		name   string
		config Config
	}{
		{"missing host ID", Config{ControlURL: "http://x", Store: store, ContainerMgr: cm}},
		{"missing control URL", Config{HostID: "h1", Store: store, ContainerMgr: cm}},
		{"missing store", Config{HostID: "h1", ControlURL: "http://x", ContainerMgr: cm}},
		{"missing container mgr", Config{HostID: "h1", ControlURL: "http://x", Store: store}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewService(testCase.config)
			if err == nil {
				t.Error("expected error for invalid config")
			}
		})
	}
}

func TestHandleDispatchEvent(t *testing.T) {
	svc, mockCM, mockRouter := testService(t)

	event := protocol.DispatchEvent{
		JobID:       uuid.MustNew(),
		ContainerID: "container-abc",
		Repository:  "org/repo",
		IssueNumber: 42,
		IssueTitle:  "Fix login bug",
		IssueBody:   "The login page crashes when...",
		IssueAuthor: "alice",
	}

	err := svc.HandleDispatchEvent(event)
	if err != nil {
		t.Fatalf("HandleDispatchEvent error: %v", err)
	}

	// Check prompt was delivered.
	prompts := mockCM.getPrompts("container-abc")
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if len(prompts[0]) == 0 {
		t.Error("prompt is empty")
	}

	// Check status was reported.
	mockRouter.mu.Lock()
	statusCount := len(mockRouter.statuses)
	mockRouter.mu.Unlock()
	// Should have at least one status report (claimed->running).
	if statusCount < 1 {
		t.Errorf("expected at least 1 status report, got %d", statusCount)
	}

	// Check active job count.
	if svc.ActiveJobCount() != 1 {
		t.Errorf("ActiveJobCount: got %d, want 1", svc.ActiveJobCount())
	}
}

func TestHandleDispatchEventUnhealthyContainer(t *testing.T) {
	svc, _, _ := testService(t)

	event := protocol.DispatchEvent{
		JobID:       uuid.MustNew(),
		ContainerID: "unhealthy-container",
		Repository:  "org/repo",
		IssueNumber: 1,
	}

	err := svc.HandleDispatchEvent(event)
	if err == nil {
		t.Error("expected error for unhealthy container")
	}
}

func TestHandleDispatchEventAdHoc(t *testing.T) {
	svc, mockCM, _ := testService(t)

	event := protocol.DispatchEvent{
		JobID:       uuid.MustNew(),
		ContainerID: "container-abc",
		Repository:  "org/repo",
		AdHoc:       true,
		Prompt:      "Add health check endpoint",
	}

	err := svc.HandleDispatchEvent(event)
	if err != nil {
		t.Fatalf("HandleDispatchEvent error: %v", err)
	}

	prompts := mockCM.getPrompts("container-abc")
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if prompts[0] != "Background task: Add health check endpoint" {
		t.Errorf("prompt: got %q", prompts[0])
	}
}

func TestTerminateJob(t *testing.T) {
	svc, mockCM, _ := testService(t)

	jobID := uuid.MustNew()
	event := protocol.DispatchEvent{
		JobID:       jobID,
		ContainerID: "container-abc",
		Repository:  "org/repo",
		IssueNumber: 1,
	}

	err := svc.HandleDispatchEvent(event)
	if err != nil {
		t.Fatalf("HandleDispatchEvent error: %v", err)
	}

	err = svc.TerminateJob(jobID)
	if err != nil {
		t.Fatalf("TerminateJob error: %v", err)
	}

	// Check the container was told to terminate.
	mockCM.mu.Lock()
	terminated := mockCM.terminated["container-abc"]
	mockCM.mu.Unlock()
	if len(terminated) != 1 {
		t.Errorf("expected 1 terminated task, got %d", len(terminated))
	}

	if svc.ActiveJobCount() != 0 {
		t.Errorf("ActiveJobCount after terminate: got %d, want 0", svc.ActiveJobCount())
	}
}

func TestTerminateJobNotActive(t *testing.T) {
	svc, _, _ := testService(t)
	err := svc.TerminateJob(uuid.MustNew())
	if err == nil {
		t.Error("expected error for terminating non-active job")
	}
}

func TestCompleteJob(t *testing.T) {
	svc, _, _ := testService(t)

	jobID := uuid.MustNew()
	event := protocol.DispatchEvent{
		JobID:       jobID,
		ContainerID: "container-abc",
		Repository:  "org/repo",
		IssueNumber: 1,
	}
	svc.HandleDispatchEvent(event)

	svc.CompleteJob(jobID, "container-abc", "https://github.com/org/repo/pull/1")

	if svc.ActiveJobCount() != 0 {
		t.Errorf("ActiveJobCount after complete: got %d, want 0", svc.ActiveJobCount())
	}
}

func TestFailJob(t *testing.T) {
	svc, _, _ := testService(t)

	jobID := uuid.MustNew()
	event := protocol.DispatchEvent{
		JobID:       jobID,
		ContainerID: "container-abc",
		Repository:  "org/repo",
		IssueNumber: 1,
	}
	svc.HandleDispatchEvent(event)

	svc.FailJob(jobID, "container-abc", "test failure")

	if svc.ActiveJobCount() != 0 {
		t.Errorf("ActiveJobCount after fail: got %d, want 0", svc.ActiveJobCount())
	}
}

func TestSendHeartbeat(t *testing.T) {
	svc, _, mockRouter := testService(t)

	svc.SendHeartbeat()

	mockRouter.mu.Lock()
	count := len(mockRouter.heartbeats)
	mockRouter.mu.Unlock()

	if count != 1 {
		t.Fatalf("expected 1 heartbeat, got %d", count)
	}

	mockRouter.mu.Lock()
	hb := mockRouter.heartbeats[0]
	mockRouter.mu.Unlock()

	if hb.HostID != "test-host-1" {
		t.Errorf("HostID: got %q, want %q", hb.HostID, "test-host-1")
	}
	if hb.ContainerCount != 1 {
		t.Errorf("ContainerCount: got %d, want 1", hb.ContainerCount)
	}
}

func TestBuildPromptIssue(t *testing.T) {
	event := protocol.DispatchEvent{
		Repository:  "org/repo",
		IssueNumber: 42,
		IssueTitle:  "Fix bug",
		IssueBody:   "The body text",
		IssueAuthor: "alice",
	}
	prompt := buildPrompt(event)
	if len(prompt) == 0 {
		t.Error("prompt is empty")
	}
	if prompt[:17] != "Background task: " {
		t.Errorf("prompt should start with 'Background task: ', got %q", prompt[:20])
	}
}

func TestBuildPromptAdHoc(t *testing.T) {
	event := protocol.DispatchEvent{
		AdHoc:  true,
		Prompt: "Do something",
	}
	prompt := buildPrompt(event)
	if prompt != "Background task: Do something" {
		t.Errorf("got %q, want %q", prompt, "Background task: Do something")
	}
}

func TestMultipleConcurrentJobs(t *testing.T) {
	svc, _, _ := testService(t)

	for i := range 5 {
		event := protocol.DispatchEvent{
			JobID:       uuid.MustNew(),
			ContainerID: "container-abc",
			Repository:  "org/repo",
			IssueNumber: i + 1,
			IssueTitle:  "Issue",
			IssueBody:   "Body",
		}
		err := svc.HandleDispatchEvent(event)
		if err != nil {
			t.Fatalf("HandleDispatchEvent %d error: %v", i, err)
		}
	}

	if svc.ActiveJobCount() != 5 {
		t.Errorf("ActiveJobCount: got %d, want 5", svc.ActiveJobCount())
	}
}

func TestStartHeartbeatLoop(t *testing.T) {
	svc, _, mockRouter := testService(t)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		svc.StartHeartbeatLoop(ctx)
		close(done)
	}()

	<-done

	mockRouter.mu.Lock()
	count := len(mockRouter.heartbeats)
	mockRouter.mu.Unlock()

	// Should have at least the initial heartbeat.
	if count < 1 {
		t.Errorf("expected at least 1 heartbeat, got %d", count)
	}
}
