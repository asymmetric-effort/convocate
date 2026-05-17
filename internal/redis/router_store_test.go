package redis

import (
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/protocol"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

func newTestRouterStore() *RouterStore {
	return NewRouterStore(NewMockConn())
}

func TestRouterStorePing(t *testing.T) {
	store := newTestRouterStore()
	err := store.Ping()
	if err != nil {
		t.Fatalf("Ping() error: %v", err)
	}
}

func TestRouterStoreContainerMap(t *testing.T) {
	store := newTestRouterStore()
	projectID := uuid.MustNew()

	entry := protocol.ContainerMapEntry{
		ContainerID: "container-abc",
		HostID:      "host-1",
		ProjectID:   projectID,
		State:       protocol.ContainerRunning,
		Image:       "convocate-agent:v0.2.0",
		CreatedAt:   time.Now().Truncate(time.Second),
		UpdatedAt:   time.Now().Truncate(time.Second),
	}

	t.Run("set and get", func(t *testing.T) {
		err := store.SetContainer(&entry)
		if err != nil {
			t.Fatalf("SetContainer error: %v", err)
		}
		got, err := store.GetContainer("container-abc")
		if err != nil {
			t.Fatalf("GetContainer error: %v", err)
		}
		if got == nil {
			t.Fatal("GetContainer returned nil")
		}
		if got.ContainerID != "container-abc" {
			t.Errorf("ContainerID: got %q, want %q", got.ContainerID, "container-abc")
		}
		if got.HostID != "host-1" {
			t.Errorf("HostID: got %q, want %q", got.HostID, "host-1")
		}
		if got.State != protocol.ContainerRunning {
			t.Errorf("State: got %q, want %q", got.State, protocol.ContainerRunning)
		}
	})

	t.Run("get nonexistent", func(t *testing.T) {
		got, err := store.GetContainer("nonexistent")
		if err != nil {
			t.Fatalf("GetContainer error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("delete", func(t *testing.T) {
		err := store.DeleteContainer("container-abc")
		if err != nil {
			t.Fatalf("DeleteContainer error: %v", err)
		}
		got, err := store.GetContainer("container-abc")
		if err != nil {
			t.Fatalf("GetContainer error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil after delete, got %+v", got)
		}
	})
}

func TestRouterStoreProjectRouting(t *testing.T) {
	store := newTestRouterStore()
	projectID := uuid.MustNew()

	entry := protocol.ProjectRouteEntry{
		ProjectID:   projectID,
		Repository:  "org/repo",
		HostID:      "host-1",
		ContainerID: "container-abc",
	}

	t.Run("set and get by project ID", func(t *testing.T) {
		err := store.SetRoute(entry)
		if err != nil {
			t.Fatalf("SetRoute error: %v", err)
		}
		got, err := store.GetRoute(projectID)
		if err != nil {
			t.Fatalf("GetRoute error: %v", err)
		}
		if got == nil {
			t.Fatal("GetRoute returned nil")
		}
		if got.Repository != "org/repo" {
			t.Errorf("Repository: got %q, want %q", got.Repository, "org/repo")
		}
	})

	t.Run("get by repo", func(t *testing.T) {
		got, err := store.GetRouteByRepo("org/repo")
		if err != nil {
			t.Fatalf("GetRouteByRepo error: %v", err)
		}
		if got == nil {
			t.Fatal("GetRouteByRepo returned nil")
		}
		if got.ProjectID != projectID {
			t.Errorf("ProjectID: got %s, want %s", got.ProjectID, projectID)
		}
	})

	t.Run("get by repo nonexistent", func(t *testing.T) {
		got, err := store.GetRouteByRepo("org/nonexistent")
		if err != nil {
			t.Fatalf("GetRouteByRepo error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("delete", func(t *testing.T) {
		err := store.DeleteRoute(projectID, "org/repo")
		if err != nil {
			t.Fatalf("DeleteRoute error: %v", err)
		}
		got, err := store.GetRoute(projectID)
		if err != nil {
			t.Fatalf("GetRoute error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil after delete, got %+v", got)
		}
	})
}

func TestRouterStoreAllowlist(t *testing.T) {
	store := newTestRouterStore()

	t.Run("add and check", func(t *testing.T) {
		err := store.AllowlistAdd("org/repo")
		if err != nil {
			t.Fatalf("AllowlistAdd error: %v", err)
		}
		contains, err := store.AllowlistContains("org/repo")
		if err != nil {
			t.Fatalf("AllowlistContains error: %v", err)
		}
		if !contains {
			t.Error("AllowlistContains returned false, want true")
		}
	})

	t.Run("check nonexistent", func(t *testing.T) {
		contains, err := store.AllowlistContains("org/other")
		if err != nil {
			t.Fatalf("AllowlistContains error: %v", err)
		}
		if contains {
			t.Error("AllowlistContains returned true for nonexistent repo")
		}
	})

	t.Run("remove", func(t *testing.T) {
		err := store.AllowlistRemove("org/repo")
		if err != nil {
			t.Fatalf("AllowlistRemove error: %v", err)
		}
		contains, err := store.AllowlistContains("org/repo")
		if err != nil {
			t.Fatalf("AllowlistContains error: %v", err)
		}
		if contains {
			t.Error("AllowlistContains returned true after remove")
		}
	})
}

func TestRouterStoreJobLedger(t *testing.T) {
	store := newTestRouterStore()
	jobID := uuid.MustNew()

	key := protocol.IdempotencyKey{
		Repository:  "org/repo",
		IssueNumber: 42,
		RunID:       12345,
	}

	t.Run("record and lookup", func(t *testing.T) {
		err := store.RecordJob(key, jobID)
		if err != nil {
			t.Fatalf("RecordJob error: %v", err)
		}
		got, err := store.LookupJobByKey(key)
		if err != nil {
			t.Fatalf("LookupJobByKey error: %v", err)
		}
		if got != jobID {
			t.Errorf("got %s, want %s", got, jobID)
		}
	})

	t.Run("lookup nonexistent", func(t *testing.T) {
		otherKey := protocol.IdempotencyKey{
			Repository:  "org/other",
			IssueNumber: 1,
			RunID:       99999,
		}
		got, err := store.LookupJobByKey(otherKey)
		if err != nil {
			t.Fatalf("LookupJobByKey error: %v", err)
		}
		if !got.IsZero() {
			t.Errorf("expected zero UUID, got %s", got)
		}
	})
}

func TestRouterStoreJobMetadata(t *testing.T) {
	store := newTestRouterStore()
	now := time.Now().Truncate(time.Second)
	jobID := uuid.MustNew()

	meta := protocol.JobMetadata{
		JobID:       jobID,
		Repository:  "org/repo",
		IssueNumber: 42,
		IssueTitle:  "Fix bug",
		IssueBody:   "The bug is...",
		IssueAuthor: "alice",
		Status:      protocol.JobClaimed,
		HostID:      "host-1",
		ContainerID: "container-abc",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	t.Run("set and get", func(t *testing.T) {
		err := store.SetJobMetadata(&meta)
		if err != nil {
			t.Fatalf("SetJobMetadata error: %v", err)
		}
		got, err := store.GetJobMetadata(jobID)
		if err != nil {
			t.Fatalf("GetJobMetadata error: %v", err)
		}
		if got == nil {
			t.Fatal("GetJobMetadata returned nil")
		}
		if got.Repository != "org/repo" {
			t.Errorf("Repository: got %q, want %q", got.Repository, "org/repo")
		}
		if got.Status != protocol.JobClaimed {
			t.Errorf("Status: got %q, want %q", got.Status, protocol.JobClaimed)
		}
	})

	t.Run("get nonexistent", func(t *testing.T) {
		got, err := store.GetJobMetadata(uuid.MustNew())
		if err != nil {
			t.Fatalf("GetJobMetadata error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("delete", func(t *testing.T) {
		err := store.DeleteJobMetadata(jobID)
		if err != nil {
			t.Fatalf("DeleteJobMetadata error: %v", err)
		}
		got, err := store.GetJobMetadata(jobID)
		if err != nil {
			t.Fatalf("GetJobMetadata error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil after delete, got %+v", got)
		}
	})
}

func TestRouterStoreAPIToken(t *testing.T) {
	store := newTestRouterStore()

	t.Run("set and validate", func(t *testing.T) {
		err := store.SetAPIToken("org/repo", "tok_abc123")
		if err != nil {
			t.Fatalf("SetAPIToken error: %v", err)
		}
		valid, err := store.ValidateAPIToken("org/repo", "tok_abc123")
		if err != nil {
			t.Fatalf("ValidateAPIToken error: %v", err)
		}
		if !valid {
			t.Error("ValidateAPIToken returned false, want true")
		}
	})

	t.Run("validate wrong token", func(t *testing.T) {
		valid, err := store.ValidateAPIToken("org/repo", "wrong_token")
		if err != nil {
			t.Fatalf("ValidateAPIToken error: %v", err)
		}
		if valid {
			t.Error("ValidateAPIToken returned true for wrong token")
		}
	})

	t.Run("validate nonexistent repo", func(t *testing.T) {
		valid, err := store.ValidateAPIToken("org/other", "tok_abc123")
		if err != nil {
			t.Fatalf("ValidateAPIToken error: %v", err)
		}
		if valid {
			t.Error("ValidateAPIToken returned true for nonexistent repo")
		}
	})

	t.Run("get", func(t *testing.T) {
		token, err := store.GetAPIToken("org/repo")
		if err != nil {
			t.Fatalf("GetAPIToken error: %v", err)
		}
		if token != "tok_abc123" {
			t.Errorf("got %q, want %q", token, "tok_abc123")
		}
	})

	t.Run("delete", func(t *testing.T) {
		err := store.DeleteAPIToken("org/repo")
		if err != nil {
			t.Fatalf("DeleteAPIToken error: %v", err)
		}
		token, err := store.GetAPIToken("org/repo")
		if err != nil {
			t.Fatalf("GetAPIToken error: %v", err)
		}
		if token != "" {
			t.Errorf("expected empty token after delete, got %q", token)
		}
	})
}

func TestRouterStoreHeartbeat(t *testing.T) {
	store := newTestRouterStore()

	heartbeat := protocol.HeartbeatRequest{
		HostID:         "host-1",
		ContainerCount: 3,
		CPUPercent:     45.5,
		MemoryPercent:  60.0,
		Timestamp:      time.Now().Truncate(time.Second),
	}

	t.Run("cache and get", func(t *testing.T) {
		err := store.CacheHeartbeat(heartbeat)
		if err != nil {
			t.Fatalf("CacheHeartbeat error: %v", err)
		}
		got, err := store.GetHeartbeat("host-1")
		if err != nil {
			t.Fatalf("GetHeartbeat error: %v", err)
		}
		if got == nil {
			t.Fatal("GetHeartbeat returned nil")
		}
		if got.ContainerCount != 3 {
			t.Errorf("ContainerCount: got %d, want 3", got.ContainerCount)
		}
	})

	t.Run("get nonexistent", func(t *testing.T) {
		got, err := store.GetHeartbeat("nonexistent")
		if err != nil {
			t.Fatalf("GetHeartbeat error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})
}

func TestRouterStoreClusterAuth(t *testing.T) {
	store := newTestRouterStore()

	t.Run("set and get API key mode", func(t *testing.T) {
		err := store.SetClusterAuth(protocol.AuthModeAnthropicKey)
		if err != nil {
			t.Fatalf("SetClusterAuth error: %v", err)
		}
		mode, err := store.GetClusterAuth()
		if err != nil {
			t.Fatalf("GetClusterAuth error: %v", err)
		}
		if mode != protocol.AuthModeAnthropicKey {
			t.Errorf("mode: got %q, want %q", mode, protocol.AuthModeAnthropicKey)
		}
	})

	t.Run("switch to session mode", func(t *testing.T) {
		err := store.SetClusterAuth(protocol.AuthModeClaudeSession)
		if err != nil {
			t.Fatalf("SetClusterAuth error: %v", err)
		}
		mode, err := store.GetClusterAuth()
		if err != nil {
			t.Fatalf("GetClusterAuth error: %v", err)
		}
		if mode != protocol.AuthModeClaudeSession {
			t.Errorf("mode: got %q, want %q", mode, protocol.AuthModeClaudeSession)
		}
	})

	t.Run("delete", func(t *testing.T) {
		err := store.DeleteClusterAuth()
		if err != nil {
			t.Fatalf("DeleteClusterAuth error: %v", err)
		}
		mode, err := store.GetClusterAuth()
		if err != nil {
			t.Fatalf("GetClusterAuth error: %v", err)
		}
		if mode != "" {
			t.Errorf("mode after delete: got %q, want empty", mode)
		}
	})
}

func TestRouterStoreProjectInfo(t *testing.T) {
	store := newTestRouterStore()
	projectID := uuid.MustNew()

	info := protocol.ProjectInfo{
		ProjectID:      projectID,
		Repository:     "org/repo",
		HostID:         "host-1",
		ContainerID:    "container-abc",
		ContainerState: protocol.ContainerRunning,
		ContainerImage: "convocate-agent:v0.2.0",
		ActiveJobs:     2,
		CreatedAt:      time.Now().Truncate(time.Second),
	}

	t.Run("set and get", func(t *testing.T) {
		err := store.SetProjectInfo(&info)
		if err != nil {
			t.Fatalf("SetProjectInfo error: %v", err)
		}
		got, err := store.GetProjectInfo(projectID)
		if err != nil {
			t.Fatalf("GetProjectInfo error: %v", err)
		}
		if got == nil {
			t.Fatal("GetProjectInfo returned nil")
		}
		if got.Repository != "org/repo" {
			t.Errorf("Repository: got %q, want %q", got.Repository, "org/repo")
		}
	})

	t.Run("get by repo", func(t *testing.T) {
		gotID, err := store.GetProjectIDByRepo("org/repo")
		if err != nil {
			t.Fatalf("GetProjectIDByRepo error: %v", err)
		}
		if gotID != projectID {
			t.Errorf("got %s, want %s", gotID, projectID)
		}
	})

	t.Run("get by repo nonexistent", func(t *testing.T) {
		gotID, err := store.GetProjectIDByRepo("org/nonexistent")
		if err != nil {
			t.Fatalf("GetProjectIDByRepo error: %v", err)
		}
		if !gotID.IsZero() {
			t.Errorf("expected zero UUID, got %s", gotID)
		}
	})

	t.Run("delete", func(t *testing.T) {
		err := store.DeleteProjectInfo(projectID, "org/repo")
		if err != nil {
			t.Fatalf("DeleteProjectInfo error: %v", err)
		}
		got, err := store.GetProjectInfo(projectID)
		if err != nil {
			t.Fatalf("GetProjectInfo error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil after delete, got %+v", got)
		}
	})
}

func TestRouterStoreNow(t *testing.T) {
	store := newTestRouterStore()
	now := store.Now()
	if now.IsZero() {
		t.Error("Now() returned zero time")
	}
}

func TestRouterStoreCountContainersByHost(t *testing.T) {
	store := newTestRouterStore()

	// Add containers for two hosts.
	for _, containerID := range []string{"c1", "c2", "c3"} {
		entry := protocol.ContainerMapEntry{
			ContainerID: containerID,
			HostID:      "host-1",
			ProjectID:   uuid.MustNew(),
			State:       protocol.ContainerRunning,
		}
		err := store.SetContainer(&entry)
		if err != nil {
			t.Fatalf("SetContainer error: %v", err)
		}
	}
	entry := protocol.ContainerMapEntry{
		ContainerID: "c4",
		HostID:      "host-2",
		ProjectID:   uuid.MustNew(),
		State:       protocol.ContainerRunning,
	}
	err := store.SetContainer(&entry)
	if err != nil {
		t.Fatalf("SetContainer error: %v", err)
	}

	count, err := store.CountContainersByHost("host-1")
	if err != nil {
		t.Fatalf("CountContainersByHost error: %v", err)
	}
	if count != 3 {
		t.Errorf("host-1 count: got %d, want 3", count)
	}

	count, err = store.CountContainersByHost("host-2")
	if err != nil {
		t.Fatalf("CountContainersByHost error: %v", err)
	}
	if count != 1 {
		t.Errorf("host-2 count: got %d, want 1", count)
	}

	count, err = store.CountContainersByHost("host-3")
	if err != nil {
		t.Fatalf("CountContainersByHost error: %v", err)
	}
	if count != 0 {
		t.Errorf("host-3 count: got %d, want 0", count)
	}
}

func TestRouterStoreFlushNamespace(t *testing.T) {
	store := newTestRouterStore()

	// Add some data.
	store.AllowlistAdd("org/repo")
	store.SetAPIToken("org/repo", "tok")
	store.SetClusterAuth(protocol.AuthModeAnthropicKey)

	err := store.FlushNamespace()
	if err != nil {
		t.Fatalf("FlushNamespace error: %v", err)
	}

	// Verify everything is gone.
	contains, _ := store.AllowlistContains("org/repo")
	if contains {
		t.Error("allowlist should be empty after flush")
	}
	token, _ := store.GetAPIToken("org/repo")
	if token != "" {
		t.Error("token should be empty after flush")
	}
}
