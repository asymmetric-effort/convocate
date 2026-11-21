package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadToken_FileVarSet_ReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token")
	if err := os.WriteFile(path, []byte("file-token\n"), 0400); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_SECRET_FILE", path)
	t.Setenv("TEST_SECRET", "env-token")

	got := ReadToken("TEST_SECRET_FILE", "TEST_SECRET")
	if got != "file-token" {
		t.Errorf("ReadToken = %q, want %q", got, "file-token")
	}
}

func TestReadToken_FileVarMissing_FallsBackToEnv(t *testing.T) {
	t.Setenv("TEST_SECRET_FILE", "")
	t.Setenv("TEST_SECRET", "env-token")

	got := ReadToken("TEST_SECRET_FILE", "TEST_SECRET")
	if got != "env-token" {
		t.Errorf("ReadToken = %q, want %q", got, "env-token")
	}
}

func TestReadToken_FileVarSetButFileUnreadable_FallsBackToEnv(t *testing.T) {
	t.Setenv("TEST_SECRET_FILE", "/nonexistent/path/token")
	t.Setenv("TEST_SECRET", "env-token")

	got := ReadToken("TEST_SECRET_FILE", "TEST_SECRET")
	if got != "env-token" {
		t.Errorf("ReadToken = %q, want %q", got, "env-token")
	}
}

func TestReadToken_BothEmpty_ReturnsEmpty(t *testing.T) {
	t.Setenv("TEST_SECRET_FILE", "")
	t.Setenv("TEST_SECRET", "")

	got := ReadToken("TEST_SECRET_FILE", "TEST_SECRET")
	if got != "" {
		t.Errorf("ReadToken = %q, want %q", got, "")
	}
}

func TestReadToken_FileTrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token")
	if err := os.WriteFile(path, []byte("  spaced-token  \n"), 0400); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_SECRET_FILE", path)
	t.Setenv("TEST_SECRET", "")

	got := ReadToken("TEST_SECRET_FILE", "TEST_SECRET")
	if got != "spaced-token" {
		t.Errorf("ReadToken = %q, want %q", got, "spaced-token")
	}
}
