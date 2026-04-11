package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupTestDir(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	skelDir := filepath.Join(base, "skel")
	if err := os.MkdirAll(skelDir, 0750); err != nil {
		t.Fatal(err)
	}
	// Create a CLAUDE.md in skel
	if err := os.WriteFile(filepath.Join(skelDir, "CLAUDE.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}
	return base, skelDir
}

func TestNewManager(t *testing.T) {
	mgr := NewManager("/tmp/test", "/tmp/skel")
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.basePath != "/tmp/test" {
		t.Errorf("basePath = %q, want %q", mgr.basePath, "/tmp/test")
	}
	if mgr.skelPath != "/tmp/skel" {
		t.Errorf("skelPath = %q, want %q", mgr.skelPath, "/tmp/skel")
	}
}

func TestCreate_Success(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	meta, err := mgr.Create("test-session")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if meta.Name != "test-session" {
		t.Errorf("Name = %q, want %q", meta.Name, "test-session")
	}
	if meta.UUID == "" {
		t.Error("UUID is empty")
	}
	if meta.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
	if meta.LastAccessed.IsZero() {
		t.Error("LastAccessed is zero")
	}

	// Verify session directory exists
	sessionDir := filepath.Join(base, meta.UUID)
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		t.Errorf("session directory not created: %s", sessionDir)
	}

	// Verify CLAUDE.md was copied from skel
	claudeMD := filepath.Join(sessionDir, "CLAUDE.md")
	if _, err := os.Stat(claudeMD); os.IsNotExist(err) {
		t.Error("CLAUDE.md not copied from skel")
	}

	// Verify metadata file exists
	metaFile := filepath.Join(sessionDir, "session.json")
	if _, err := os.Stat(metaFile); os.IsNotExist(err) {
		t.Error("session.json not created")
	}
}

func TestCreateWithUUID(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "12345678-1234-1234-1234-123456789abc"
	meta, err := mgr.CreateWithUUID(testUUID, "named-session")
	if err != nil {
		t.Fatalf("CreateWithUUID failed: %v", err)
	}
	if meta.UUID != testUUID {
		t.Errorf("UUID = %q, want %q", meta.UUID, testUUID)
	}
	if meta.Name != "named-session" {
		t.Errorf("Name = %q, want %q", meta.Name, "named-session")
	}
}

func TestList_Empty(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "")

	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestList_NonExistentDir(t *testing.T) {
	mgr := NewManager("/nonexistent/path/12345", "")

	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List should not error for nonexistent dir: %v", err)
	}
	if sessions != nil {
		t.Errorf("expected nil sessions, got %v", sessions)
	}
}

func TestList_MultipleSessions(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	_, err := mgr.CreateWithUUID("aaaaaaaa-1111-1111-1111-111111111111", "first")
	if err != nil {
		t.Fatal(err)
	}

	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	_, err = mgr.CreateWithUUID("bbbbbbbb-2222-2222-2222-222222222222", "second")
	if err != nil {
		t.Fatal(err)
	}

	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// Should be sorted by last accessed (most recent first)
	if sessions[0].Name != "second" {
		t.Errorf("first session should be 'second' (most recent), got %q", sessions[0].Name)
	}
}

func TestList_IgnoresNonUUIDDirs(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	// Create a non-UUID directory
	if err := os.MkdirAll(filepath.Join(base, "not-a-uuid"), 0750); err != nil {
		t.Fatal(err)
	}

	_, err := mgr.CreateWithUUID("cccccccc-3333-3333-3333-333333333333", "valid")
	if err != nil {
		t.Fatal(err)
	}

	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session (ignoring non-UUID dirs), got %d", len(sessions))
	}
}

func TestGet_Success(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "dddddddd-4444-4444-4444-444444444444"
	_, err := mgr.CreateWithUUID(testUUID, "get-test")
	if err != nil {
		t.Fatal(err)
	}

	meta, err := mgr.Get(testUUID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if meta.Name != "get-test" {
		t.Errorf("Name = %q, want %q", meta.Name, "get-test")
	}
}

func TestGet_NotFound(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "")

	_, err := mgr.Get("nonexistent-uuid")
	if err == nil {
		t.Error("expected error for nonexistent session, got nil")
	}
}

func TestDelete_Success(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "eeeeeeee-5555-5555-5555-555555555555"
	_, err := mgr.CreateWithUUID(testUUID, "delete-test")
	if err != nil {
		t.Fatal(err)
	}

	if err := mgr.Delete(testUUID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	sessionDir := filepath.Join(base, testUUID)
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Error("session directory still exists after delete")
	}
}

func TestDelete_NotFound(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "")

	err := mgr.Delete("nonexistent-uuid")
	if err == nil {
		t.Error("expected error for nonexistent session, got nil")
	}
}

func TestDelete_Locked(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "ffffffff-6666-6666-6666-666666666666"
	_, err := mgr.CreateWithUUID(testUUID, "locked-test")
	if err != nil {
		t.Fatal(err)
	}

	// Create a lock file
	lockPath := filepath.Join(base, testUUID+".lock")
	if err := os.WriteFile(lockPath, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}

	err = mgr.Delete(testUUID)
	if err == nil {
		t.Error("expected error when deleting locked session, got nil")
	}
}

func TestTouch(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "11111111-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	_, err := mgr.CreateWithUUID(testUUID, "touch-test")
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(10 * time.Millisecond)

	if err := mgr.Touch(testUUID); err != nil {
		t.Fatalf("Touch failed: %v", err)
	}

	meta, err := mgr.Get(testUUID)
	if err != nil {
		t.Fatal(err)
	}

	if !meta.LastAccessed.After(meta.CreatedAt) {
		t.Error("LastAccessed should be after CreatedAt after Touch")
	}
}

func TestLock_Success(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "22222222-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	_, err := mgr.CreateWithUUID(testUUID, "lock-test")
	if err != nil {
		t.Fatal(err)
	}

	unlock, err := mgr.Lock(testUUID)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	// Verify lock file exists
	lockPath := filepath.Join(base, testUUID+".lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file not created")
	}

	// Unlock
	unlock()

	// Verify lock file removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file not removed after unlock")
	}
}

func TestLock_AlreadyLocked(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "33333333-cccc-cccc-cccc-cccccccccccc"
	_, err := mgr.CreateWithUUID(testUUID, "double-lock-test")
	if err != nil {
		t.Fatal(err)
	}

	unlock, err := mgr.Lock(testUUID)
	if err != nil {
		t.Fatalf("first Lock failed: %v", err)
	}
	defer unlock()

	_, err = mgr.Lock(testUUID)
	if err == nil {
		t.Error("expected error for double lock, got nil")
	}
}

func TestIsLocked_NotLocked(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "44444444-dddd-dddd-dddd-dddddddddddd"
	_, err := mgr.CreateWithUUID(testUUID, "islocked-test")
	if err != nil {
		t.Fatal(err)
	}

	if mgr.IsLocked(testUUID) {
		t.Error("session should not be locked")
	}
}

func TestIsLocked_Locked(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "55555555-eeee-eeee-eeee-eeeeeeeeeeee"
	_, err := mgr.CreateWithUUID(testUUID, "islocked-locked-test")
	if err != nil {
		t.Fatal(err)
	}

	unlock, err := mgr.Lock(testUUID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	if !mgr.IsLocked(testUUID) {
		t.Error("session should be locked")
	}
}

func TestIsLocked_StaleLock(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "66666666-ffff-ffff-ffff-ffffffffffff"
	_, err := mgr.CreateWithUUID(testUUID, "stale-lock-test")
	if err != nil {
		t.Fatal(err)
	}

	// Create a lock file with invalid PID
	lockPath := filepath.Join(base, testUUID+".lock")
	if err := os.WriteFile(lockPath, []byte("99999999"), 0600); err != nil {
		t.Fatal(err)
	}

	// Lock should be considered stale (PID doesn't exist)
	if mgr.IsLocked(testUUID) {
		t.Error("stale lock should be cleaned up")
	}

	// Lock file should have been removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("stale lock file should have been removed")
	}
}

func TestSessionDir(t *testing.T) {
	mgr := NewManager("/base/path", "/skel/path")
	expected := "/base/path/test-uuid"
	got := mgr.SessionDir("test-uuid")
	if got != expected {
		t.Errorf("SessionDir = %q, want %q", got, expected)
	}
}

func TestValidateName_Valid(t *testing.T) {
	validNames := []string{
		"test",
		"my-session",
		"Session 1",
		"bug_fix_123",
		"a",
	}

	for _, name := range validNames {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) returned unexpected error: %v", name, err)
		}
	}
}

func TestValidateName_Empty(t *testing.T) {
	err := ValidateName("")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestValidateName_Whitespace(t *testing.T) {
	err := ValidateName("   ")
	if err == nil {
		t.Error("expected error for whitespace-only name")
	}
}

func TestValidateName_TooLong(t *testing.T) {
	longName := ""
	for i := 0; i < 65; i++ {
		longName += "a"
	}
	err := ValidateName(longName)
	if err == nil {
		t.Error("expected error for name exceeding 64 characters")
	}
}

func TestValidateName_ControlChars(t *testing.T) {
	err := ValidateName("test\x00name")
	if err == nil {
		t.Error("expected error for name with control characters")
	}
}

func TestCreate_NoSkel(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "/nonexistent/skel")

	meta, err := mgr.Create("no-skel")
	if err != nil {
		t.Fatalf("Create should succeed without skel dir: %v", err)
	}

	sessionDir := filepath.Join(base, meta.UUID)
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		t.Error("session directory not created")
	}
}

func TestCopySkel_WithSymlink(t *testing.T) {
	base := t.TempDir()
	skelDir := filepath.Join(base, "skel")
	if err := os.MkdirAll(skelDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Create a regular file
	if err := os.WriteFile(filepath.Join(skelDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink
	targetFile := filepath.Join(base, "target.txt")
	if err := os.WriteFile(targetFile, []byte("target"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetFile, filepath.Join(skelDir, "link.txt")); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory with a file
	subDir := filepath.Join(skelDir, "subdir")
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "sub.txt"), []byte("sub"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(base, skelDir)
	destDir := filepath.Join(base, "dest")

	if err := mgr.copySkel(destDir); err != nil {
		t.Fatalf("copySkel failed: %v", err)
	}

	// Verify file copied
	if _, err := os.Stat(filepath.Join(destDir, "file.txt")); os.IsNotExist(err) {
		t.Error("file.txt not copied")
	}

	// Verify symlink copied
	link, err := os.Readlink(filepath.Join(destDir, "link.txt"))
	if err != nil {
		t.Errorf("link.txt not copied as symlink: %v", err)
	}
	if link != targetFile {
		t.Errorf("symlink target = %q, want %q", link, targetFile)
	}

	// Verify subdirectory and file copied
	if _, err := os.Stat(filepath.Join(destDir, "subdir", "sub.txt")); os.IsNotExist(err) {
		t.Error("subdir/sub.txt not copied")
	}
}

func TestSetupClaudeSymlinks(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "")
	sessionDir := filepath.Join(base, "test-session")
	if err := os.MkdirAll(sessionDir, 0750); err != nil {
		t.Fatal(err)
	}

	if err := mgr.setupClaudeSymlinks(sessionDir); err != nil {
		t.Fatalf("setupClaudeSymlinks failed: %v", err)
	}

	claudeDir := filepath.Join(sessionDir, ".claude")
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		t.Error(".claude directory not created")
	}

	// Check symlinks exist
	for _, name := range []string{"settings.json", "settings.local.json", ".credentials.json", "plugins"} {
		link := filepath.Join(claudeDir, name)
		target, err := os.Readlink(link)
		if err != nil {
			t.Errorf("symlink %s not created: %v", name, err)
			continue
		}
		expectedPrefix := "/home/claude/.claude-shared/"
		if !strings.HasPrefix(target, expectedPrefix) {
			t.Errorf("symlink %s target = %q, expected prefix %q", name, target, expectedPrefix)
		}
	}
}

func TestSetupClaudeSymlinks_Idempotent(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "")
	sessionDir := filepath.Join(base, "test-session")
	if err := os.MkdirAll(sessionDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Run twice - second run should not fail
	if err := mgr.setupClaudeSymlinks(sessionDir); err != nil {
		t.Fatal(err)
	}
	if err := mgr.setupClaudeSymlinks(sessionDir); err != nil {
		t.Fatalf("second setupClaudeSymlinks should be idempotent: %v", err)
	}
}

func TestReadMetadata_InvalidJSON(t *testing.T) {
	base := t.TempDir()
	sessionDir := filepath.Join(base, "bad-session")
	if err := os.MkdirAll(sessionDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "session.json"), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(base, "")
	_, err := mgr.Get("bad-session")
	if err == nil {
		t.Error("expected error for invalid JSON metadata")
	}
}

func TestList_SkipsBadMetadata(t *testing.T) {
	base := t.TempDir()

	// Create a UUID-named dir with invalid metadata
	badUUID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	badDir := filepath.Join(base, badUUID)
	if err := os.MkdirAll(badDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "session.json"), []byte("invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(base, "")
	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	// Should skip the bad entry
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (bad metadata skipped), got %d", len(sessions))
	}
}

func TestIsLocked_InvalidLockContent(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "77777777-aaaa-bbbb-cccc-dddddddddddd"
	_, err := mgr.CreateWithUUID(testUUID, "bad-lock-test")
	if err != nil {
		t.Fatal(err)
	}

	// Create lock file with non-numeric content
	lockPath := filepath.Join(base, testUUID+".lock")
	if err := os.WriteFile(lockPath, []byte("not-a-pid"), 0600); err != nil {
		t.Fatal(err)
	}

	if mgr.IsLocked(testUUID) {
		t.Error("invalid lock content should result in not locked (cleaned up)")
	}
}

func TestTouch_NonexistentSession(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "")

	err := mgr.Touch("nonexistent")
	if err == nil {
		t.Error("expected error when touching nonexistent session")
	}
}
