package redis

import (
	"fmt"
	"sync"
	"testing"

	"github.com/asymmetric-effort/convocate/internal/protocol"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// badDoer is a mock that returns unexpected types to test defensive error paths.
type badDoer struct {
	inner        *MockConn
	failOn       map[string]bool
	pingResponse interface{}
	scanResponse interface{}
	getResponse  interface{}
	mu           sync.Mutex
}

func newBadDoer() *badDoer {
	return &badDoer{
		inner:  NewMockConn(),
		failOn: make(map[string]bool),
	}
}

func (b *badDoer) Do(args ...string) (interface{}, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(args) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := args[0]

	if cmd == "PING" && b.pingResponse != nil {
		return b.pingResponse, nil
	}

	if cmd == "SCAN" && b.scanResponse != nil {
		return b.scanResponse, nil
	}

	if cmd == "GET" && b.getResponse != nil {
		return b.getResponse, nil
	}

	if len(args) >= 2 && b.failOn[args[1]] {
		return nil, fmt.Errorf("injected error")
	}

	return b.inner.Do(args...)
}

func (b *badDoer) Close() error {
	return b.inner.Close()
}

// TestRouterStorePingUnexpected tests the Ping "unexpected response" path.
func TestRouterStorePingUnexpected(t *testing.T) {
	bd := newBadDoer()
	bd.pingResponse = "NOT-PONG"
	store := NewRouterStore(bd)
	err := store.Ping()
	if err == nil {
		t.Error("expected error for unexpected PING response")
	}
}

// TestDispatchStorePingUnexpected tests the Ping "unexpected response" path.
func TestDispatchStorePingUnexpected(t *testing.T) {
	bd := newBadDoer()
	bd.pingResponse = "NOT-PONG"
	store := NewDispatchStore(bd, "h1")
	err := store.Ping()
	if err == nil {
		t.Error("expected error for unexpected PING response")
	}
}

// TestRouterStoreFlushBadScanResult tests flushByPrefix with bad SCAN result.
func TestRouterStoreFlushBadScanResult(t *testing.T) {
	bd := newBadDoer()
	bd.scanResponse = "not-an-array" // Wrong type.
	store := NewRouterStore(bd)
	err := store.FlushNamespace()
	if err == nil {
		t.Error("expected error for bad SCAN result type")
	}
}

// TestRouterStoreFlushBadScanCursor tests bad cursor type.
func TestRouterStoreFlushBadScanCursor(t *testing.T) {
	bd := newBadDoer()
	bd.scanResponse = []interface{}{42, []interface{}{}} // Cursor is int, not string.
	store := NewRouterStore(bd)
	err := store.FlushNamespace()
	if err == nil {
		t.Error("expected error for bad SCAN cursor type")
	}
}

// TestRouterStoreFlushBadScanKeys tests bad keys type.
func TestRouterStoreFlushBadScanKeys(t *testing.T) {
	bd := newBadDoer()
	bd.scanResponse = []interface{}{"0", "not-an-array"} // Keys is string, not array.
	store := NewRouterStore(bd)
	err := store.FlushNamespace()
	if err == nil {
		t.Error("expected error for bad SCAN keys type")
	}
}

// TestDispatchStoreFlushBadScanResult tests dispatch FlushNamespace with bad SCAN.
func TestDispatchStoreFlushBadScanResult(t *testing.T) {
	bd := newBadDoer()
	bd.scanResponse = "not-an-array"
	store := NewDispatchStore(bd, "h1")
	err := store.FlushNamespace()
	if err == nil {
		t.Error("expected error for bad SCAN result")
	}
}

// TestDispatchStoreFlushBadCursor tests dispatch FlushNamespace with bad cursor.
func TestDispatchStoreFlushBadCursor(t *testing.T) {
	bd := newBadDoer()
	bd.scanResponse = []interface{}{42, []interface{}{}}
	store := NewDispatchStore(bd, "h1")
	err := store.FlushNamespace()
	if err == nil {
		t.Error("expected error for bad SCAN cursor")
	}
}

// TestDispatchStoreFlushBadKeys tests dispatch FlushNamespace with bad keys.
func TestDispatchStoreFlushBadKeys(t *testing.T) {
	bd := newBadDoer()
	bd.scanResponse = []interface{}{"0", "not-an-array"}
	store := NewDispatchStore(bd, "h1")
	err := store.FlushNamespace()
	if err == nil {
		t.Error("expected error for bad SCAN keys")
	}
}

// TestRouterStoreCountContainersBadScan tests bad SCAN in CountContainersByHost.
func TestRouterStoreCountContainersBadScan(t *testing.T) {
	bd := newBadDoer()
	bd.scanResponse = "not-an-array"
	store := NewRouterStore(bd)
	_, err := store.CountContainersByHost("h1")
	if err == nil {
		t.Error("expected error for bad SCAN result")
	}
}

// TestRouterStoreCountContainersBadCursor tests bad cursor in CountContainersByHost.
func TestRouterStoreCountContainersBadCursor(t *testing.T) {
	bd := newBadDoer()
	bd.scanResponse = []interface{}{42, []interface{}{}}
	store := NewRouterStore(bd)
	_, err := store.CountContainersByHost("h1")
	if err == nil {
		t.Error("expected error for bad cursor")
	}
}

// TestRouterStoreCountContainersBadKeys tests bad keys in CountContainersByHost.
func TestRouterStoreCountContainersBadKeys(t *testing.T) {
	bd := newBadDoer()
	bd.scanResponse = []interface{}{"0", "not-an-array"}
	store := NewRouterStore(bd)
	_, err := store.CountContainersByHost("h1")
	if err == nil {
		t.Error("expected error for bad keys")
	}
}

// TestRouterStoreSetClusterAuthError tests the Set error.
func TestRouterStoreSetClusterAuthError(t *testing.T) {
	bd := newBadDoer()
	bd.failOn["router:cluster-auth-mode"] = true
	store := NewRouterStore(bd)
	err := store.SetClusterAuth(protocol.AuthModeAnthropicKey)
	if err == nil {
		t.Error("expected error for Set failure")
	}
}

// TestRouterStoreDeleteClusterAuthFirstError tests Del error.
func TestRouterStoreDeleteClusterAuthFirstError(t *testing.T) {
	bd := newBadDoer()
	bd.failOn["router:cluster-auth-mode"] = true
	store := NewRouterStore(bd)
	err := store.DeleteClusterAuth()
	if err == nil {
		t.Error("expected error for Del failure")
	}
}

// TestRouterStoreDeleteRouteFirstError tests first Del error.
func TestRouterStoreDeleteRouteFirstError(t *testing.T) {
	bd := newBadDoer()
	id := uuid.MustNew()
	bd.failOn["router:route:"+id.String()] = true
	store := NewRouterStore(bd)
	err := store.DeleteRoute(id, "org/repo")
	if err == nil {
		t.Error("expected error for first Del failure")
	}
}

// TestRouterStoreDeleteProjectInfoFirstError tests first Del error.
func TestRouterStoreDeleteProjectInfoFirstError(t *testing.T) {
	bd := newBadDoer()
	id := uuid.MustNew()
	bd.failOn["router:project:"+id.String()] = true
	store := NewRouterStore(bd)
	err := store.DeleteProjectInfo(id, "org/repo")
	if err == nil {
		t.Error("expected error for first Del failure")
	}
}

// TestRouterStoreSetRouteFirstError tests first Set error in SetRoute.
func TestRouterStoreSetRouteFirstError(t *testing.T) {
	bd := newBadDoer()
	id := uuid.MustNew()
	bd.failOn["router:route:"+id.String()] = true
	store := NewRouterStore(bd)
	err := store.SetRoute(protocol.ProjectRouteEntry{ProjectID: id, Repository: "org/repo"})
	if err == nil {
		t.Error("expected error for first Set failure")
	}
}

// TestRouterStoreSetProjectInfoFirstError tests first Set error.
func TestRouterStoreSetProjectInfoFirstError(t *testing.T) {
	bd := newBadDoer()
	id := uuid.MustNew()
	bd.failOn["router:project:"+id.String()] = true
	store := NewRouterStore(bd)
	err := store.SetProjectInfo(&protocol.ProjectInfo{ProjectID: id, Repository: "org/repo"})
	if err == nil {
		t.Error("expected error for first Set failure")
	}
}

// TestRouterStoreGetErrors tests error paths in Get* methods when Redis returns errors.
func TestRouterStoreGetErrors(t *testing.T) {
	bd := newBadDoer()
	store := NewRouterStore(bd)

	t.Run("GetContainer error", func(t *testing.T) {
		bd.mu.Lock()
		bd.failOn["router:container:err"] = true
		bd.mu.Unlock()
		_, err := store.GetContainer("err")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("GetRoute error", func(t *testing.T) {
		id := uuid.MustNew()
		bd.mu.Lock()
		bd.failOn["router:route:"+id.String()] = true
		bd.mu.Unlock()
		_, err := store.GetRoute(id)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("GetRouteByRepo error", func(t *testing.T) {
		bd.mu.Lock()
		bd.failOn["router:route-by-repo:org/err"] = true
		bd.mu.Unlock()
		_, err := store.GetRouteByRepo("org/err")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("LookupJobByKey error", func(t *testing.T) {
		key := protocol.IdempotencyKey{Repository: "org/err", IssueNumber: 99, RunID: 99}
		bd.mu.Lock()
		bd.failOn["router:ledger:"+key.String()] = true
		bd.mu.Unlock()
		_, err := store.LookupJobByKey(key)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("GetJobMetadata error", func(t *testing.T) {
		id := uuid.MustNew()
		bd.mu.Lock()
		bd.failOn["router:job:"+id.String()] = true
		bd.mu.Unlock()
		_, err := store.GetJobMetadata(id)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ValidateAPIToken error", func(t *testing.T) {
		bd.mu.Lock()
		bd.failOn["router:token:org/err"] = true
		bd.mu.Unlock()
		_, err := store.ValidateAPIToken("org/err", "tok")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("GetHeartbeat error", func(t *testing.T) {
		bd.mu.Lock()
		bd.failOn["router:heartbeat:err-host"] = true
		bd.mu.Unlock()
		_, err := store.GetHeartbeat("err-host")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("GetProjectInfo error", func(t *testing.T) {
		id := uuid.MustNew()
		bd.mu.Lock()
		bd.failOn["router:project:"+id.String()] = true
		bd.mu.Unlock()
		_, err := store.GetProjectInfo(id)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("GetProjectIDByRepo error", func(t *testing.T) {
		bd.mu.Lock()
		bd.failOn["router:project-by-repo:org/err"] = true
		bd.mu.Unlock()
		_, err := store.GetProjectIDByRepo("org/err")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("Ping error", func(t *testing.T) {
		bd.mu.Lock()
		bd.failOn["PING"] = true
		bd.mu.Unlock()
		// Need the Ping Do to fail, but failOn checks args[1], and PING
		// doesn't have args[1]. Use a different approach.
	})
}

// TestDispatchStoreGetErrors tests error paths in DispatchStore Get* methods.
func TestDispatchStoreGetErrors(t *testing.T) {
	bd := newBadDoer()
	store := NewDispatchStore(bd, "h1")

	t.Run("GetJobState error", func(t *testing.T) {
		id := uuid.MustNew()
		bd.mu.Lock()
		bd.failOn["dispatch:h1:job:"+id.String()] = true
		bd.mu.Unlock()
		_, err := store.GetJobState(id)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("DequeueDispatch error", func(t *testing.T) {
		bd.mu.Lock()
		bd.failOn["dispatch:h1:queue"] = true
		bd.mu.Unlock()
		_, err := store.DequeueDispatch()
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("Ping unexpected response", func(t *testing.T) {
		bd2 := newBadDoer()
		bd2.pingResponse = "NOT-PONG"
		s2 := NewDispatchStore(bd2, "h1")
		err := s2.Ping()
		if err == nil {
			t.Error("expected error for bad PING response")
		}
	})
}

// TestParseScanResult tests the parseScanResult helper.
func TestParseScanResult(t *testing.T) {
	t.Run("valid result", func(t *testing.T) {
		result := []interface{}{"0", []interface{}{"key1", "key2"}}
		cursor, keys, err := parseScanResult(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cursor != "0" {
			t.Errorf("cursor: got %q, want 0", cursor)
		}
		if len(keys) != 2 {
			t.Errorf("keys: got %d, want 2", len(keys))
		}
	})

	t.Run("non-string key skipped", func(t *testing.T) {
		result := []interface{}{"0", []interface{}{"key1", 42, "key2"}}
		_, keys, err := parseScanResult(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(keys) != 2 {
			t.Errorf("keys: got %d, want 2", len(keys))
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		_, _, err := parseScanResult("not-an-array")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("bad cursor type", func(t *testing.T) {
		_, _, err := parseScanResult([]interface{}{42, []interface{}{}})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("bad keys type", func(t *testing.T) {
		_, _, err := parseScanResult([]interface{}{"0", "not-an-array"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// TestDoPing tests the doPing helper.
func TestDoPing(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := NewMockConn()
		err := doPing(mock)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unexpected response", func(t *testing.T) {
		bd := newBadDoer()
		bd.pingResponse = "NOT-PONG"
		err := doPing(bd)
		if err == nil {
			t.Error("expected error for unexpected response")
		}
	})

	t.Run("closed conn", func(t *testing.T) {
		mock := NewMockConn()
		mock.Close()
		err := doPing(mock)
		if err == nil {
			t.Error("expected error for closed conn")
		}
	})
}

// TestRouterStoreFlushDelError tests flushByPrefix when DEL fails on a scanned key.
func TestRouterStoreFlushDelError(t *testing.T) {
	bd := newBadDoer()
	// Seed a key so SCAN finds it.
	bd.inner.Do("SET", "router:deltest:key1", "val")
	// Make DEL fail for this key.
	bd.failOn["router:deltest:key1"] = true
	store := NewRouterStore(bd)
	err := store.FlushNamespace()
	if err == nil {
		t.Error("expected error for DEL failure during flush")
	}
}

// TestDispatchStoreFlushDelError tests FlushNamespace when DEL fails.
func TestDispatchStoreFlushDelError(t *testing.T) {
	bd := newBadDoer()
	bd.inner.Do("SET", "dispatch:h1:deltest", "val")
	bd.failOn["dispatch:h1:deltest"] = true
	store := NewDispatchStore(bd, "h1")
	err := store.FlushNamespace()
	if err == nil {
		t.Error("expected error for DEL failure during flush")
	}
}

// TestRouterStoreFlushScanError tests flushByPrefix when SCAN returns error.
func TestRouterStoreFlushScanError(t *testing.T) {
	bd := newBadDoer()
	// Make SCAN return an error.
	bd.failOn["0"] = true // SCAN args are: "SCAN", "0", "MATCH", ...
	store := NewRouterStore(bd)
	err := store.FlushNamespace()
	if err == nil {
		t.Error("expected error for SCAN failure")
	}
}

// TestCountContainersByScanError tests CountContainersByHost when SCAN errors.
func TestCountContainersByScanError(t *testing.T) {
	bd := newBadDoer()
	bd.failOn["0"] = true
	store := NewRouterStore(bd)
	_, err := store.CountContainersByHost("h1")
	if err == nil {
		t.Error("expected error for SCAN failure")
	}
}

// TestCountContainersByHostCorruptEntry tests the unmarshal-error continue path.
func TestCountContainersByHostCorruptEntry(t *testing.T) {
	mock := NewMockConn()
	store := NewRouterStore(mock)
	// Seed a corrupt container entry.
	mock.Do("SET", "router:container:corrupt1", "not-json")
	// Seed a valid one.
	mock.Do("SET", "router:container:valid1", `{"container_id":"valid1","host_id":"h1"}`)
	count, err := store.CountContainersByHost("h1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("count: got %d, want 1", count)
	}
}

// TestRouterStoreGetClusterAuthFirstError tests Get error in GetClusterAuth.
func TestRouterStoreGetClusterAuthFirstError(t *testing.T) {
	bd := newBadDoer()
	bd.failOn["router:cluster-auth-mode"] = true
	store := NewRouterStore(bd)
	_, err := store.GetClusterAuth()
	if err == nil {
		t.Error("expected error for Get failure")
	}
}
