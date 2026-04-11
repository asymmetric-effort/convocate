package skel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetup_CreatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	skelPath := filepath.Join(tmpDir, "newskel")

	if err := Setup(skelPath); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	info, err := os.Stat(skelPath)
	if err != nil {
		t.Fatalf("skel dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("skel path is not a directory")
	}
}

func TestSetup_CreatesCLAUDEMD(t *testing.T) {
	tmpDir := t.TempDir()
	skelPath := filepath.Join(tmpDir, "skel")

	if err := Setup(skelPath); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	claudeMD := filepath.Join(skelPath, "CLAUDE.md")
	data, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("CLAUDE.md not created: %v", err)
	}
	if len(data) == 0 {
		t.Error("CLAUDE.md is empty")
	}
}

func TestSetup_DoesNotOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	skelPath := filepath.Join(tmpDir, "skel")
	if err := os.MkdirAll(skelPath, 0750); err != nil {
		t.Fatal(err)
	}

	customContent := "# Custom CLAUDE.md"
	claudeMD := filepath.Join(skelPath, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte(customContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Setup(skelPath); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	data, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != customContent {
		t.Errorf("CLAUDE.md was overwritten; got %q, want %q", string(data), customContent)
	}
}

func TestSetup_InvalidPath(t *testing.T) {
	err := Setup("/proc/nonexistent/invalid/path")
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

func TestExists_True(t *testing.T) {
	tmpDir := t.TempDir()
	skelPath := filepath.Join(tmpDir, "skel")

	if err := Setup(skelPath); err != nil {
		t.Fatal(err)
	}

	if !Exists(skelPath) {
		t.Error("Exists returned false for valid skel dir")
	}
}

func TestExists_NoDir(t *testing.T) {
	if Exists("/nonexistent/path/12345") {
		t.Error("Exists returned true for nonexistent path")
	}
}

func TestExists_NoCLAUDEMD(t *testing.T) {
	tmpDir := t.TempDir()
	skelPath := filepath.Join(tmpDir, "skel")
	if err := os.MkdirAll(skelPath, 0750); err != nil {
		t.Fatal(err)
	}

	if Exists(skelPath) {
		t.Error("Exists returned true for skel dir without CLAUDE.md")
	}
}

func TestExists_NotADir(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "notadir")
	if err := os.WriteFile(filePath, []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}

	if Exists(filePath) {
		t.Error("Exists returned true for a file (not a directory)")
	}
}
