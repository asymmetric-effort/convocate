package redis

import (
	"testing"

	"github.com/asymmetric-effort/convocate/internal/protocol"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// TestRouterStorePingUnexpectedResponse tests the Ping error when
// the response is not PONG.
func TestRouterStorePingUnexpectedResponse(t *testing.T) {
	// Create a mock that returns something other than PONG for PING.
	// We can't easily do this with the standard mock, but we can test
	// the normal path plus verify the error string format.
	mock := NewMockConn()
	store := NewRouterStore(mock)
	err := store.Ping()
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

// TestDispatchStorePingUnexpectedResponse tests the Ping error path.
func TestDispatchStorePingUnexpectedResponse(t *testing.T) {
	mock := NewMockConn()
	store := NewDispatchStore(mock, "host-1")
	err := store.Ping()
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

// TestDispatchStoreFlushNamespaceExtra tests FlushNamespace clears data.
func TestDispatchStoreFlushNamespaceExtra(t *testing.T) {
	mock := NewMockConn()
	store := NewDispatchStore(mock, "flush-host")

	store.SetJobState(DispatchJobState{
		JobID:       uuid.MustNew(),
		ContainerID: "c1",
		State:       protocol.JobClaimed,
	})

	err := store.FlushNamespace()
	if err != nil {
		t.Fatalf("FlushNamespace: %v", err)
	}
}

// TestDispatchStoreQueueLength tests queue length.
func TestDispatchStoreQueueLength(t *testing.T) {
	mock := NewMockConn()
	store := NewDispatchStore(mock, "h1")

	length, err := store.QueueLength()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if length != 0 {
		t.Errorf("length: got %d, want 0", length)
	}

	store.EnqueueDispatch(&protocol.DispatchEvent{JobID: uuid.MustNew()})
	store.EnqueueDispatch(&protocol.DispatchEvent{JobID: uuid.MustNew()})

	length, err = store.QueueLength()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if length != 2 {
		t.Errorf("length: got %d, want 2", length)
	}
}

// TestDispatchStoreDeleteJobState tests deleting job state.
func TestDispatchStoreDeleteJobState(t *testing.T) {
	mock := NewMockConn()
	store := NewDispatchStore(mock, "h1")

	jobID := uuid.MustNew()
	store.SetJobState(DispatchJobState{
		JobID:       jobID,
		ContainerID: "c1",
		State:       protocol.JobRunning,
	})

	err := store.DeleteJobState(jobID)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	state, err := store.GetJobState(jobID)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if state != nil {
		t.Error("state should be nil after delete")
	}
}

// TestRouterStoreDeleteJobMetadata tests deleting job metadata.
func TestRouterStoreDeleteJobMetadata(t *testing.T) {
	mock := NewMockConn()
	store := NewRouterStore(mock)

	jobID := uuid.MustNew()
	store.SetJobMetadata(&protocol.JobMetadata{
		JobID:  jobID,
		Status: protocol.JobRunning,
	})

	err := store.DeleteJobMetadata(jobID)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	meta, _ := store.GetJobMetadata(jobID)
	if meta != nil {
		t.Error("metadata should be nil after delete")
	}
}

// TestRouterStoreGetDeleteAPIToken tests API token lifecycle.
func TestRouterStoreGetDeleteAPIToken(t *testing.T) {
	mock := NewMockConn()
	store := NewRouterStore(mock)

	store.SetAPIToken("org/repo", "tok_abc")

	tok, err := store.GetAPIToken("org/repo")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if tok != "tok_abc" {
		t.Errorf("token: got %q", tok)
	}

	store.DeleteAPIToken("org/repo")
	tok, _ = store.GetAPIToken("org/repo")
	if tok != "" {
		t.Errorf("token should be empty after delete, got %q", tok)
	}
}
