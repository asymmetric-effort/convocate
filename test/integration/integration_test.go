//go:build integration

// Package integration provides integration tests for claude-shell.
package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/asymmetric-effort/claude-shell/internal/config"
	"github.com/asymmetric-effort/claude-shell/internal/session"
	"github.com/asymmetric-effort/claude-shell/internal/skel"
)

func TestSessionLifecycle(t *testing.T) {
	base := t.TempDir()
	skelDir := filepath.Join(base, config.SkelDir)

	// Setup skel
	if err := skel.Setup(skelDir); err != nil {
		t.Fatalf("skel.Setup failed: %v", err)
	}

	mgr := session.NewManager(base, skelDir)

	// Create sessions
	s1, err := mgr.Create("test-session-1")
	if err != nil {
		t.Fatalf("Create session 1 failed: %v", err)
	}

	s2, err := mgr.Create("test-session-2")
	if err != nil {
		t.Fatalf("Create session 2 failed: %v", err)
	}

	// List sessions
	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// Verify session directories have CLAUDE.md
	for _, s := range []session.Metadata{s1, s2} {
		claudeMD := filepath.Join(base, s.UUID, "CLAUDE.md")
		if _, err := os.Stat(claudeMD); os.IsNotExist(err) {
			t.Errorf("session %s missing CLAUDE.md", s.UUID)
		}
	}

	// Verify session directories have .claude with symlinks
	for _, s := range []session.Metadata{s1, s2} {
		claudeDir := filepath.Join(base, s.UUID, config.ClaudeConfigDir)
		if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
			t.Errorf("session %s missing .claude directory", s.UUID)
		}
		// Check symlinks exist
		for _, name := range []string{"settings.json", "settings.local.json", ".credentials.json", "plugins"} {
			link := filepath.Join(claudeDir, name)
			if _, err := os.Lstat(link); os.IsNotExist(err) {
				t.Errorf("session %s missing symlink %s", s.UUID, name)
			}
		}
	}

	// Lock session 1
	unlock, err := mgr.Lock(s1.UUID)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	// Verify locked
	if !mgr.IsLocked(s1.UUID) {
		t.Error("session 1 should be locked")
	}

	// Try to delete locked session
	err = mgr.Delete(s1.UUID)
	if err == nil {
		t.Error("expected error deleting locked session")
	}

	// Unlock
	unlock()

	// Verify unlocked
	if mgr.IsLocked(s1.UUID) {
		t.Error("session 1 should be unlocked")
	}

	// Delete session 1
	if err := mgr.Delete(s1.UUID); err != nil {
		t.Fatalf("Delete session 1 failed: %v", err)
	}

	// Verify deleted
	sessions, err = mgr.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session after delete, got %d", len(sessions))
	}
	if sessions[0].UUID != s2.UUID {
		t.Error("remaining session should be session 2")
	}
}

func TestConcurrentSessionLocking(t *testing.T) {
	base := t.TempDir()
	skelDir := filepath.Join(base, config.SkelDir)
	if err := skel.Setup(skelDir); err != nil {
		t.Fatal(err)
	}

	mgr := session.NewManager(base, skelDir)

	s, err := mgr.Create("concurrent-test")
	if err != nil {
		t.Fatal(err)
	}

	// Lock from first instance
	unlock1, err := mgr.Lock(s.UUID)
	if err != nil {
		t.Fatalf("first lock failed: %v", err)
	}

	// Second lock should fail
	_, err = mgr.Lock(s.UUID)
	if err == nil {
		t.Error("expected error on concurrent lock")
	}

	// Release first lock
	unlock1()

	// Now second lock should succeed
	unlock2, err := mgr.Lock(s.UUID)
	if err != nil {
		t.Fatalf("lock after release failed: %v", err)
	}
	defer unlock2()
}

func TestSessionIsolation(t *testing.T) {
	base := t.TempDir()
	skelDir := filepath.Join(base, config.SkelDir)
	if err := skel.Setup(skelDir); err != nil {
		t.Fatal(err)
	}

	mgr := session.NewManager(base, skelDir)

	s1, err := mgr.Create("isolated-1")
	if err != nil {
		t.Fatal(err)
	}
	s2, err := mgr.Create("isolated-2")
	if err != nil {
		t.Fatal(err)
	}

	// Write a file in session 1
	testFile := filepath.Join(base, s1.UUID, "test-file.txt")
	if err := os.WriteFile(testFile, []byte("session 1 data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Verify file does NOT exist in session 2
	testFile2 := filepath.Join(base, s2.UUID, "test-file.txt")
	if _, err := os.Stat(testFile2); !os.IsNotExist(err) {
		t.Error("file from session 1 should not appear in session 2")
	}
}
