package dispatch

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asymmetric-effort/convocate/internal/protocol"
	redispkg "github.com/asymmetric-effort/convocate/internal/redis"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

var errDelivery = fmt.Errorf("delivery failed")

// TestHandleDispatchEventDeliverPromptFails tests that dispatch returns error
// when prompt delivery fails.
func TestHandleDispatchEventDeliverPromptFails(t *testing.T) {
	event := protocol.DispatchEvent{
		JobID:       uuid.MustNew(),
		ContainerID: "c-deliver-fail",
		Repository:  "org/repo",
		IssueNumber: 1,
	}

	// Normal delivery works fine for this container. To test failure,
	// we need a container manager that fails. Create a new service.
	failCM := &failingContainerManager{healthy: true}
	mockRouter, routerServer := newMockRouterAPI()
	defer routerServer.Close()
	_ = mockRouter

	mockConn := redispkg.NewMockConn()
	store := redispkg.NewDispatchStore(mockConn, "fail-host")

	failSvc, err := NewService(Config{
		HostID:       "fail-host",
		ControlURL:   routerServer.URL,
		Store:        store,
		ContainerMgr: failCM,
		Logger:       log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer failSvc.Stop()

	err = failSvc.HandleDispatchEvent(&event)
	if err == nil {
		t.Error("expected error when prompt delivery fails")
	}
}

// TestTerminateJobStoreStateMissing tests TerminateJob when the job state
// is not found in the dispatch store.
func TestTerminateJobStoreStateMissing(t *testing.T) {
	svc, _, _ := testService(t)

	jobID := uuid.MustNew()
	// Manually add to active jobs so TerminateJob doesn't error on "not active".
	svc.mu.Lock()
	_, cancel := context.WithCancel(context.Background())
	svc.activeJobs[jobID] = cancel
	svc.mu.Unlock()

	err := svc.TerminateJob(jobID)
	if err == nil {
		t.Error("expected error when job state not found in store")
	}
}

// TestTerminateJobTerminateTaskError tests the TerminateTask error path
// in TerminateJob (error is logged but job still transitions to terminated).
func TestTerminateJobTerminateTaskError(t *testing.T) {
	failCM := &failingContainerManager{healthy: true}
	mockRouter, routerServer := newMockRouterAPI()
	defer routerServer.Close()
	_ = mockRouter

	mockConn := redispkg.NewMockConn()
	store := redispkg.NewDispatchStore(mockConn, "term-err-host")

	svc, err := NewService(Config{
		HostID:       "term-err-host",
		ControlURL:   routerServer.URL,
		Store:        store,
		ContainerMgr: failCM,
		Logger:       log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Stop()

	jobID := uuid.MustNew()

	// Record state in store.
	store.SetJobState(redispkg.DispatchJobState{
		JobID:       jobID,
		ContainerID: "c1",
		State:       protocol.JobRunning,
		Repository:  "org/repo",
	})

	// Make the job active.
	svc.mu.Lock()
	_, cancel := context.WithCancel(context.Background())
	svc.activeJobs[jobID] = cancel
	svc.mu.Unlock()

	// TerminateJob should succeed even though TerminateTask errors.
	err = svc.TerminateJob(jobID)
	if err != nil {
		t.Errorf("TerminateJob should not fail: %v", err)
	}
}

// TestBuildPromptNoAuthor tests buildPrompt when IssueAuthor is empty.
func TestBuildPromptNoAuthor(t *testing.T) {
	event := protocol.DispatchEvent{
		Repository:  "org/repo",
		IssueNumber: 42,
		IssueTitle:  "Fix bug",
		IssueBody:   "The body text",
	}
	prompt := buildPrompt(&event)
	if prompt == "" {
		t.Error("prompt is empty")
	}
	// Should not contain "Original author" section.
	for i := range len(prompt) - 15 {
		if prompt[i:i+15] == "Original author" {
			t.Error("should not contain 'Original author' when IssueAuthor is empty")
			break
		}
	}
}

// TestNewServiceDefaultHTTPClient verifies default HTTP client is created.
func TestNewServiceDefaultHTTPClient(t *testing.T) {
	mockConn := redispkg.NewMockConn()
	store := redispkg.NewDispatchStore(mockConn, "default-client")
	cm := newMockContainerManager()

	svc, err := NewService(Config{
		HostID:       "default-client",
		ControlURL:   "http://localhost:1",
		Store:        store,
		ContainerMgr: cm,
		// HTTPClient intentionally nil - should use default.
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Stop()
	if svc.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

// TestReportStatusNonComplete verifies that non-complete status reports
// don't have PullURL set.
func TestReportStatusNonComplete(t *testing.T) {
	svc, _, mockRouter := testService(t)

	svc.ReportStatus(uuid.MustNew(), "c1", protocol.JobClaimed, protocol.JobFailed, "error msg")

	mockRouter.mu.Lock()
	defer mockRouter.mu.Unlock()
	if len(mockRouter.statuses) < 1 {
		t.Fatal("expected at least 1 status report")
	}
	last := mockRouter.statuses[len(mockRouter.statuses)-1]
	if last.PullURL != "" {
		t.Errorf("PullURL should be empty for failed status, got %q", last.PullURL)
	}
	if last.Reason != "error msg" {
		t.Errorf("Reason: got %q, want 'error msg'", last.Reason)
	}
}

// TestSendHeartbeatHTTPServerError verifies heartbeat with a server that returns errors.
func TestSendHeartbeatHTTPServerError(t *testing.T) {
	mockCM := newMockContainerManager()
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errServer.Close()

	mockConn := redispkg.NewMockConn()
	store := redispkg.NewDispatchStore(mockConn, "hb-err-host")

	svc, err := NewService(Config{
		HostID:       "hb-err-host",
		ControlURL:   errServer.URL,
		Store:        store,
		ContainerMgr: mockCM,
		Logger:       log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Stop()

	// Should not panic.
	svc.SendHeartbeat()
}

// TestReportStatusInvalidURL tests ReportStatus with an invalid control URL.
func TestReportStatusInvalidURL(t *testing.T) {
	mockCM := newMockContainerManager()
	mockConn := redispkg.NewMockConn()
	store := redispkg.NewDispatchStore(mockConn, "bad-url")

	svc, err := NewService(Config{
		HostID:       "bad-url",
		ControlURL:   "://invalid-url",
		Store:        store,
		ContainerMgr: mockCM,
		Logger:       log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Stop()

	// Should not panic.
	svc.ReportStatus(uuid.MustNew(), "c1", protocol.JobClaimed, protocol.JobRunning, "")
}

// TestSendHeartbeatInvalidURL tests SendHeartbeat with an invalid control URL.
func TestSendHeartbeatInvalidURL(t *testing.T) {
	mockCM := newMockContainerManager()
	mockConn := redispkg.NewMockConn()
	store := redispkg.NewDispatchStore(mockConn, "bad-url-hb")

	svc, err := NewService(Config{
		HostID:       "bad-url-hb",
		ControlURL:   "://invalid-url",
		Store:        store,
		ContainerMgr: mockCM,
		Logger:       log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Stop()

	// Should not panic.
	svc.SendHeartbeat()
}

// failingContainerManager always fails on DeliverPrompt.
type failingContainerManager struct {
	healthy bool
}

func (f *failingContainerManager) DeliverPrompt(string, uuid.UUID, string) error {
	return errDelivery
}
func (f *failingContainerManager) StopContainer(string) error {
	return nil
}

func (f *failingContainerManager) ProvisionContainer(string, uuid.UUID) error {
	return nil
}

func (f *failingContainerManager) IsHealthy(string) bool {
	return f.healthy
}

func (f *failingContainerManager) TerminateTask(string, uuid.UUID) error {
	return fmt.Errorf("terminate failed")
}

func (f *failingContainerManager) ContainerCount() int {
	return 0
}

func (f *failingContainerManager) CPUPercent() float64 {
	return 0
}

func (f *failingContainerManager) MemoryPercent() float64 {
	return 0
}
