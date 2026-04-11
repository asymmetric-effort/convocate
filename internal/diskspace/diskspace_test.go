package diskspace

import (
	"strings"
	"testing"
)

func TestCheckForFile_SufficientSpace(t *testing.T) {
	// A 1-byte file should always have enough space in /tmp
	if err := CheckForFile(t.TempDir(), 1); err != nil {
		t.Errorf("expected sufficient space for 1-byte file: %v", err)
	}
}

func TestCheckForFile_ExtremeSize(t *testing.T) {
	// Request an absurdly large file size that no filesystem can satisfy
	err := CheckForFile(t.TempDir(), 1<<62)
	if err == nil {
		t.Fatal("expected error for impossibly large file")
	}
	if !strings.Contains(err.Error(), "not enough disk space") {
		t.Errorf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "Please free up some space") {
		t.Errorf("error should include user-friendly guidance: %v", err)
	}
}

func TestCheckForFile_NonexistentDir(t *testing.T) {
	// Should walk up to an existing ancestor
	tmpDir := t.TempDir()
	deepPath := tmpDir + "/a/b/c/d"
	if err := CheckForFile(deepPath, 1); err != nil {
		t.Errorf("should resolve to existing ancestor: %v", err)
	}
}

func TestNearestExistingDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Existing dir returns itself
	got, err := nearestExistingDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != tmpDir {
		t.Errorf("got %q, want %q", got, tmpDir)
	}

	// Non-existent child returns parent
	got, err = nearestExistingDir(tmpDir + "/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != tmpDir {
		t.Errorf("got %q, want %q", got, tmpDir)
	}
}
