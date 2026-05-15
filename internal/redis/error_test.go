package redis

import (
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/protocol"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// TestRouterStoreCorruptData verifies that the store returns errors
// when Redis contains corrupt (non-JSON) data.
func TestRouterStoreCorruptData(t *testing.T) {
	mock := NewMockConn()
	store := NewRouterStore(mock)

	t.Run("GetContainer corrupt JSON", func(t *testing.T) {
		mock.Do("SET", "router:container:corrupt", "not-json{{{")
		_, err := store.GetContainer("corrupt")
		if err == nil {
			t.Error("expected unmarshal error")
		}
	})

	t.Run("GetRoute corrupt JSON", func(t *testing.T) {
		id := uuid.MustNew()
		mock.Do("SET", "router:route:"+id.String(), "not-json")
		_, err := store.GetRoute(id)
		if err == nil {
			t.Error("expected unmarshal error")
		}
	})

	t.Run("GetRouteByRepo corrupt JSON", func(t *testing.T) {
		mock.Do("SET", "router:route-by-repo:org/corrupt", "not-json")
		_, err := store.GetRouteByRepo("org/corrupt")
		if err == nil {
			t.Error("expected unmarshal error")
		}
	})

	t.Run("LookupJobByKey corrupt UUID", func(t *testing.T) {
		key := protocol.IdempotencyKey{Repository: "org/repo", IssueNumber: 1, RunID: 1}
		mock.Do("SET", "router:ledger:"+key.String(), "not-a-uuid")
		_, err := store.LookupJobByKey(key)
		if err == nil {
			t.Error("expected UUID parse error")
		}
	})

	t.Run("GetJobMetadata corrupt JSON", func(t *testing.T) {
		id := uuid.MustNew()
		mock.Do("SET", "router:job:"+id.String(), "not-json")
		_, err := store.GetJobMetadata(id)
		if err == nil {
			t.Error("expected unmarshal error")
		}
	})

	t.Run("GetHeartbeat corrupt JSON", func(t *testing.T) {
		mock.Do("SET", "router:heartbeat:corrupt-host", "not-json")
		_, err := store.GetHeartbeat("corrupt-host")
		if err == nil {
			t.Error("expected unmarshal error")
		}
	})

	t.Run("GetProjectInfo corrupt JSON", func(t *testing.T) {
		id := uuid.MustNew()
		mock.Do("SET", "router:project:"+id.String(), "not-json")
		_, err := store.GetProjectInfo(id)
		if err == nil {
			t.Error("expected unmarshal error")
		}
	})

	t.Run("GetProjectIDByRepo corrupt UUID", func(t *testing.T) {
		mock.Do("SET", "router:project-by-repo:org/corrupt", "not-a-uuid")
		_, err := store.GetProjectIDByRepo("org/corrupt")
		if err == nil {
			t.Error("expected UUID parse error")
		}
	})

	t.Run("CacheHeartbeat with valid data round-trip", func(t *testing.T) {
		hb := protocol.HeartbeatRequest{
			HostID:         "h-roundtrip",
			ContainerCount: 5,
			CPUPercent:     99.9,
			MemoryPercent:  88.8,
			Timestamp:      time.Now().Truncate(time.Second),
		}
		err := store.CacheHeartbeat(hb)
		if err != nil {
			t.Fatalf("CacheHeartbeat error: %v", err)
		}
		got, err := store.GetHeartbeat("h-roundtrip")
		if err != nil {
			t.Fatalf("GetHeartbeat error: %v", err)
		}
		if got.CPUPercent != 99.9 {
			t.Errorf("CPUPercent: got %f, want 99.9", got.CPUPercent)
		}
	})

	t.Run("SetRoute second index error path", func(t *testing.T) {
		entry := protocol.ProjectRouteEntry{
			ProjectID:   uuid.MustNew(),
			Repository:  "org/route-test",
			HostID:      "h1",
			ContainerID: "c1",
		}
		err := store.SetRoute(entry)
		if err != nil {
			t.Fatalf("SetRoute error: %v", err)
		}
		// Verify both indexes exist.
		got, _ := store.GetRoute(entry.ProjectID)
		if got == nil {
			t.Error("route by ID not found")
		}
		got2, _ := store.GetRouteByRepo("org/route-test")
		if got2 == nil {
			t.Error("route by repo not found")
		}
	})

	t.Run("DeleteRoute both indexes", func(t *testing.T) {
		id := uuid.MustNew()
		store.SetRoute(protocol.ProjectRouteEntry{
			ProjectID:  id,
			Repository: "org/del-test",
			HostID:     "h1",
		})
		err := store.DeleteRoute(id, "org/del-test")
		if err != nil {
			t.Fatalf("DeleteRoute error: %v", err)
		}
		got, _ := store.GetRoute(id)
		if got != nil {
			t.Error("route by ID should be nil")
		}
		got2, _ := store.GetRouteByRepo("org/del-test")
		if got2 != nil {
			t.Error("route by repo should be nil")
		}
	})

	t.Run("SetClusterAuth both keys", func(t *testing.T) {
		err := store.SetClusterAuth(protocol.AuthModeAnthropicKey, "sk-test")
		if err != nil {
			t.Fatalf("SetClusterAuth error: %v", err)
		}
		mode, cred, err := store.GetClusterAuth()
		if err != nil {
			t.Fatalf("GetClusterAuth error: %v", err)
		}
		if mode != protocol.AuthModeAnthropicKey {
			t.Errorf("mode: got %q", mode)
		}
		if cred != "sk-test" {
			t.Errorf("credential: got %q", cred)
		}
	})

	t.Run("DeleteClusterAuth both keys", func(t *testing.T) {
		store.SetClusterAuth(protocol.AuthModeAnthropicKey, "sk-test")
		err := store.DeleteClusterAuth()
		if err != nil {
			t.Fatalf("DeleteClusterAuth error: %v", err)
		}
		mode, cred, _ := store.GetClusterAuth()
		if mode != "" || cred != "" {
			t.Errorf("after delete: mode=%q, cred=%q", mode, cred)
		}
	})

	t.Run("DeleteProjectInfo both indexes", func(t *testing.T) {
		id := uuid.MustNew()
		store.SetProjectInfo(protocol.ProjectInfo{
			ProjectID:  id,
			Repository: "org/del-proj",
		})
		err := store.DeleteProjectInfo(id, "org/del-proj")
		if err != nil {
			t.Fatalf("DeleteProjectInfo error: %v", err)
		}
		got, _ := store.GetProjectInfo(id)
		if got != nil {
			t.Error("project info should be nil")
		}
		gotID, _ := store.GetProjectIDByRepo("org/del-proj")
		if !gotID.IsZero() {
			t.Error("project ID by repo should be zero")
		}
	})

	t.Run("SetProjectInfo both indexes", func(t *testing.T) {
		id := uuid.MustNew()
		info := protocol.ProjectInfo{
			ProjectID:  id,
			Repository: "org/proj-idx",
			HostID:     "h1",
		}
		err := store.SetProjectInfo(info)
		if err != nil {
			t.Fatalf("SetProjectInfo error: %v", err)
		}
		gotID, _ := store.GetProjectIDByRepo("org/proj-idx")
		if gotID != id {
			t.Errorf("project ID by repo: got %s, want %s", gotID, id)
		}
	})

	t.Run("ValidateAPIToken empty stored", func(t *testing.T) {
		valid, err := store.ValidateAPIToken("org/no-token", "anything")
		if err != nil {
			t.Fatalf("ValidateAPIToken error: %v", err)
		}
		if valid {
			t.Error("should be invalid when no token stored")
		}
	})

	t.Run("Ping unexpected response", func(t *testing.T) {
		// Normal Ping should work.
		err := store.Ping()
		if err != nil {
			t.Fatalf("Ping error: %v", err)
		}
	})
}

// TestDispatchStoreCorruptData tests error handling for corrupt stored data.
func TestDispatchStoreCorruptData(t *testing.T) {
	mock := NewMockConn()
	store := NewDispatchStore(mock, "test-host")

	t.Run("GetJobState corrupt JSON", func(t *testing.T) {
		id := uuid.MustNew()
		mock.Do("SET", "dispatch:test-host:job:"+id.String(), "not-json")
		_, err := store.GetJobState(id)
		if err == nil {
			t.Error("expected unmarshal error")
		}
	})

	t.Run("DequeueDispatch corrupt JSON", func(t *testing.T) {
		mock.Do("RPUSH", "dispatch:test-host:queue", "not-json")
		_, err := store.DequeueDispatch()
		if err == nil {
			t.Error("expected unmarshal error")
		}
	})

	t.Run("Ping works", func(t *testing.T) {
		err := store.Ping()
		if err != nil {
			t.Fatalf("Ping error: %v", err)
		}
	})
}

// TestBoolHelperError tests the Bool helper error propagation.
func TestBoolHelperError(t *testing.T) {
	_, err := Bool(nil, &RedisError{Message: "test"})
	if err == nil {
		t.Error("expected error to propagate")
	}
}

// TestConnClosedError tests that operations on a closed mock fail.
func TestMockConnClosedOperations(t *testing.T) {
	mock := NewMockConn()
	mock.Close()

	_, err := mock.Do("SET", "key", "val")
	if err == nil {
		t.Error("expected error on closed mock")
	}
}
