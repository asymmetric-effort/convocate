package redis

import (
	"testing"

	"github.com/asymmetric-effort/convocate/internal/protocol"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

func newTestDispatchStore() *DispatchStore {
	return NewDispatchStore(NewMockConn(), "dev-host-1")
}

func TestDispatchStorePing(t *testing.T) {
	store := newTestDispatchStore()
	err := store.Ping()
	if err != nil {
		t.Fatalf("Ping() error: %v", err)
	}
}

func TestDispatchStoreJobState(t *testing.T) {
	store := newTestDispatchStore()
	jobID := uuid.MustNew()

	state := DispatchJobState{
		JobID:       jobID,
		ContainerID: "container-abc",
		State:       protocol.JobRunning,
		Repository:  "org/repo",
		IssueNumber: 42,
	}

	t.Run("set and get", func(t *testing.T) {
		err := store.SetJobState(state)
		if err != nil {
			t.Fatalf("SetJobState error: %v", err)
		}
		got, err := store.GetJobState(jobID)
		if err != nil {
			t.Fatalf("GetJobState error: %v", err)
		}
		if got == nil {
			t.Fatal("GetJobState returned nil")
		}
		if got.ContainerID != "container-abc" {
			t.Errorf("ContainerID: got %q, want %q", got.ContainerID, "container-abc")
		}
		if got.State != protocol.JobRunning {
			t.Errorf("State: got %q, want %q", got.State, protocol.JobRunning)
		}
	})

	t.Run("get nonexistent", func(t *testing.T) {
		got, err := store.GetJobState(uuid.MustNew())
		if err != nil {
			t.Fatalf("GetJobState error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("update state", func(t *testing.T) {
		state.State = protocol.JobComplete
		err := store.SetJobState(state)
		if err != nil {
			t.Fatalf("SetJobState error: %v", err)
		}
		got, err := store.GetJobState(jobID)
		if err != nil {
			t.Fatalf("GetJobState error: %v", err)
		}
		if got.State != protocol.JobComplete {
			t.Errorf("State: got %q, want %q", got.State, protocol.JobComplete)
		}
	})

	t.Run("delete", func(t *testing.T) {
		err := store.DeleteJobState(jobID)
		if err != nil {
			t.Fatalf("DeleteJobState error: %v", err)
		}
		got, err := store.GetJobState(jobID)
		if err != nil {
			t.Fatalf("GetJobState error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil after delete, got %+v", got)
		}
	})
}

func TestDispatchStoreQueue(t *testing.T) {
	store := newTestDispatchStore()

	event1 := protocol.DispatchEvent{
		JobID:       uuid.MustNew(),
		ContainerID: "container-abc",
		Repository:  "org/repo",
		IssueNumber: 1,
		IssueTitle:  "First issue",
	}
	event2 := protocol.DispatchEvent{
		JobID:       uuid.MustNew(),
		ContainerID: "container-abc",
		Repository:  "org/repo",
		IssueNumber: 2,
		IssueTitle:  "Second issue",
	}

	t.Run("enqueue", func(t *testing.T) {
		err := store.EnqueueDispatch(event1)
		if err != nil {
			t.Fatalf("EnqueueDispatch error: %v", err)
		}
		err = store.EnqueueDispatch(event2)
		if err != nil {
			t.Fatalf("EnqueueDispatch error: %v", err)
		}
	})

	t.Run("queue length", func(t *testing.T) {
		length, err := store.QueueLength()
		if err != nil {
			t.Fatalf("QueueLength error: %v", err)
		}
		if length != 2 {
			t.Errorf("QueueLength: got %d, want 2", length)
		}
	})

	t.Run("dequeue FIFO order", func(t *testing.T) {
		got, err := store.DequeueDispatch()
		if err != nil {
			t.Fatalf("DequeueDispatch error: %v", err)
		}
		if got == nil {
			t.Fatal("DequeueDispatch returned nil")
		}
		if got.IssueNumber != 1 {
			t.Errorf("IssueNumber: got %d, want 1 (FIFO)", got.IssueNumber)
		}

		got, err = store.DequeueDispatch()
		if err != nil {
			t.Fatalf("DequeueDispatch error: %v", err)
		}
		if got == nil {
			t.Fatal("DequeueDispatch returned nil")
		}
		if got.IssueNumber != 2 {
			t.Errorf("IssueNumber: got %d, want 2 (FIFO)", got.IssueNumber)
		}
	})

	t.Run("dequeue empty", func(t *testing.T) {
		got, err := store.DequeueDispatch()
		if err != nil {
			t.Fatalf("DequeueDispatch error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil from empty queue, got %+v", got)
		}
	})

	t.Run("queue length after drain", func(t *testing.T) {
		length, err := store.QueueLength()
		if err != nil {
			t.Fatalf("QueueLength error: %v", err)
		}
		if length != 0 {
			t.Errorf("QueueLength after drain: got %d, want 0", length)
		}
	})
}

func TestDispatchStoreNamespaceIsolation(t *testing.T) {
	mock := NewMockConn()
	store1 := NewDispatchStore(mock, "host-1")
	store2 := NewDispatchStore(mock, "host-2")

	jobID := uuid.MustNew()
	state := DispatchJobState{
		JobID:       jobID,
		ContainerID: "c1",
		State:       protocol.JobRunning,
		Repository:  "org/repo",
		IssueNumber: 1,
	}

	err := store1.SetJobState(state)
	if err != nil {
		t.Fatalf("store1.SetJobState error: %v", err)
	}

	// store2 should not see store1's job.
	got, err := store2.GetJobState(jobID)
	if err != nil {
		t.Fatalf("store2.GetJobState error: %v", err)
	}
	if got != nil {
		t.Errorf("store2 should not see store1's job, got %+v", got)
	}
}

func TestDispatchStoreRouterNamespaceIsolation(t *testing.T) {
	mock := NewMockConn()
	dispatchStore := NewDispatchStore(mock, "host-1")
	routerStore := NewRouterStore(mock)

	// Write to router namespace.
	err := routerStore.AllowlistAdd("org/repo")
	if err != nil {
		t.Fatalf("AllowlistAdd error: %v", err)
	}

	// Dispatch store queue should be empty (different namespace).
	length, err := dispatchStore.QueueLength()
	if err != nil {
		t.Fatalf("QueueLength error: %v", err)
	}
	if length != 0 {
		t.Errorf("dispatch queue should be empty, got %d", length)
	}
}

func TestDispatchStoreAdHocDispatchEvent(t *testing.T) {
	store := newTestDispatchStore()

	event := protocol.DispatchEvent{
		JobID:       uuid.MustNew(),
		ContainerID: "container-abc",
		Repository:  "org/repo",
		AdHoc:       true,
		Prompt:      "Add a health check endpoint",
	}

	err := store.EnqueueDispatch(event)
	if err != nil {
		t.Fatalf("EnqueueDispatch error: %v", err)
	}

	got, err := store.DequeueDispatch()
	if err != nil {
		t.Fatalf("DequeueDispatch error: %v", err)
	}
	if got == nil {
		t.Fatal("DequeueDispatch returned nil")
	}
	if !got.AdHoc {
		t.Error("AdHoc: got false, want true")
	}
	if got.Prompt != "Add a health check endpoint" {
		t.Errorf("Prompt: got %q, want %q", got.Prompt, "Add a health check endpoint")
	}
}
