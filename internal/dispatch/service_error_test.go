package dispatch

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/protocol"
	redispkg "github.com/asymmetric-effort/convocate/internal/redis"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// TestReportStatusUnreachableServer tests that ReportStatus logs and
// does not panic when the Router API is unreachable.
func TestReportStatusUnreachableServer(t *testing.T) {
	mockCM := newMockContainerManager()
	mockConn := redispkg.NewMockConn()
	store := redispkg.NewDispatchStore(mockConn, "unreachable-host")

	svc, err := NewService(Config{
		HostID:       "unreachable-host",
		ControlURL:   "http://127.0.0.1:1", // Port 1 is unreachable.
		Store:        store,
		ContainerMgr: mockCM,
		Logger:       log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Stop()

	// Should not panic even though server is unreachable.
	svc.ReportStatus(uuid.MustNew(), "c1", protocol.JobClaimed, protocol.JobRunning, "")
}

// TestSendHeartbeatUnreachableServer tests that SendHeartbeat logs and
// does not panic when the Router API is unreachable.
func TestSendHeartbeatUnreachableServer(t *testing.T) {
	mockCM := newMockContainerManager()
	mockConn := redispkg.NewMockConn()
	store := redispkg.NewDispatchStore(mockConn, "unreachable-hb")

	svc, err := NewService(Config{
		HostID:       "unreachable-hb",
		ControlURL:   "http://127.0.0.1:1",
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

// TestReportStatusComplete verifies PR URL is set correctly when
// reporting a complete status.
func TestReportStatusComplete(t *testing.T) {
	svc, _, mockRouter := testService(t)

	jobID := uuid.MustNew()
	svc.ReportStatus(jobID, "c1", protocol.JobRunning, protocol.JobComplete, "https://github.com/org/repo/pull/1")

	time.Sleep(50 * time.Millisecond)

	mockRouter.mu.Lock()
	defer mockRouter.mu.Unlock()
	if len(mockRouter.statuses) < 1 {
		t.Fatal("expected at least 1 status report")
	}
	last := mockRouter.statuses[len(mockRouter.statuses)-1]
	if last.PullURL != "https://github.com/org/repo/pull/1" {
		t.Errorf("PullURL: got %q", last.PullURL)
	}
	if last.Reason != "" {
		t.Errorf("Reason should be empty for complete with PR URL, got %q", last.Reason)
	}
}

// TestCompleteJobNotFound verifies CompleteJob handles missing state gracefully.
func TestCompleteJobNotFound(t *testing.T) {
	svc, _, _ := testService(t)
	// Should not panic.
	svc.CompleteJob(uuid.MustNew(), "c-notfound", "")
}

// TestFailJobNotFound verifies FailJob handles missing state gracefully.
func TestFailJobNotFound(t *testing.T) {
	svc, _, _ := testService(t)
	// Should not panic.
	svc.FailJob(uuid.MustNew(), "c-notfound", "reason")
}

// TestHandleDispatchEventSetJobStateError tests the SetJobState error path
// in HandleDispatchEvent.
func TestHandleDispatchEventSetJobStateError(t *testing.T) {
	mockCM := newMockContainerManager()
	mockCM.healthy["c1"] = true

	mockRouter, routerServer := newMockRouterAPI()
	defer routerServer.Close()
	_ = mockRouter

	// Use a mock conn that is already closed to force errors.
	mockConn := redispkg.NewMockConn()
	mockConn.Close()
	store := redispkg.NewDispatchStore(mockConn, "err-host")

	svc, err := NewService(Config{
		HostID:       "err-host",
		ControlURL:   routerServer.URL,
		Store:        store,
		ContainerMgr: mockCM,
		Logger:       log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Stop()

	event := &protocol.DispatchEvent{
		JobID:       uuid.MustNew(),
		ContainerID: "c1",
		Repository:  "org/repo",
		IssueNumber: 1,
		IssueTitle:  "test",
	}

	// SetJobState should fail because conn is closed.
	err = svc.HandleDispatchEvent(event)
	if err == nil {
		t.Error("expected error for SetJobState failure")
	}
}

// TestTerminateJobGetJobStateError tests the GetJobState error path in TerminateJob.
func TestTerminateJobGetJobStateError(t *testing.T) {
	mockCM := newMockContainerManager()
	mockCM.healthy["c1"] = true

	mockRouter, routerServer := newMockRouterAPI()
	defer routerServer.Close()
	_ = mockRouter

	mockConn := redispkg.NewMockConn()
	store := redispkg.NewDispatchStore(mockConn, "err-host2")

	svc, err := NewService(Config{
		HostID:       "err-host2",
		ControlURL:   routerServer.URL,
		Store:        store,
		ContainerMgr: mockCM,
		Logger:       log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Stop()

	jobID := uuid.MustNew()

	// Handle a dispatch event first so the job is active.
	event := &protocol.DispatchEvent{
		JobID:       jobID,
		ContainerID: "c1",
		Repository:  "org/repo",
		IssueNumber: 1,
		IssueTitle:  "test",
	}
	err = svc.HandleDispatchEvent(event)
	if err != nil {
		t.Fatalf("HandleDispatchEvent: %v", err)
	}

	// Close the conn to make GetJobState fail.
	mockConn.Close()

	err = svc.TerminateJob(jobID)
	if err == nil {
		t.Error("expected error for GetJobState failure")
	}
}

// TestStartHeartbeatLoopStopChannel verifies the stop channel works.
func TestStartHeartbeatLoopStopChannel(t *testing.T) {
	// Build a service without testService to avoid the cleanup double-close.
	mockCM := newMockContainerManager()
	mockCM.healthy["container-abc"] = true

	mockRouter, routerServer := newMockRouterAPI()
	defer routerServer.Close()
	_ = mockRouter

	mockConn := redispkg.NewMockConn()
	store := redispkg.NewDispatchStore(mockConn, "stop-test-host")

	svc, err := NewService(Config{
		HostID:       "stop-test-host",
		ControlURL:   routerServer.URL,
		Store:        store,
		ContainerMgr: mockCM,
		Logger:       log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		svc.StartHeartbeatLoop(ctx)
		close(done)
	}()

	// Give initial heartbeat time to fire.
	time.Sleep(50 * time.Millisecond)
	svc.Stop()

	select {
	case <-done:
		// Exited cleanly via stopCh.
	case <-time.After(2 * time.Second):
		t.Error("StartHeartbeatLoop did not exit after Stop()")
	}
}
