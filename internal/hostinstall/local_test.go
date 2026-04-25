package hostinstall

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalRunner_TargetCloseTrivial(t *testing.T) {
	r := NewLocalRunner()
	if got := r.Target(); got != "local" {
		t.Errorf("Target = %q, want 'local'", got)
	}
	if err := r.Close(); err != nil {
		t.Errorf("Close = %v, want nil", err)
	}
}

func TestLocalRunner_Run_HappyPath(t *testing.T) {
	r := NewLocalRunner()
	var out bytes.Buffer
	if err := r.Run(context.Background(), "echo hi", RunOptions{Stdout: &out}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := out.String(); got != "hi\n" {
		t.Errorf("stdout = %q, want %q", got, "hi\n")
	}
}

func TestLocalRunner_Run_PropagatesNonzeroExit(t *testing.T) {
	r := NewLocalRunner()
	err := r.Run(context.Background(), "false", RunOptions{})
	if err == nil {
		t.Error("expected non-nil error from `false`")
	}
}

func TestLocalRunner_Run_RespectsCtxCancel(t *testing.T) {
	r := NewLocalRunner()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Run(ctx, "sleep 5", RunOptions{}); err == nil {
		t.Error("expected canceled context to abort Run")
	}
}

func TestLocalRunner_CopyFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "nested", "dst") // intermediate dir doesn't exist yet
	// We don't auto-create intermediate dirs in LocalRunner.CopyFile,
	// so put dst alongside src to keep the test honest about the API.
	dst = filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	r := NewLocalRunner()
	if err := r.CopyFile(context.Background(), src, dst, 0640); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload" {
		t.Errorf("dst content = %q", got)
	}
	st, _ := os.Stat(dst)
	if st.Mode().Perm() != 0640 {
		t.Errorf("dst mode = %o, want 0640", st.Mode().Perm())
	}
}

func TestLocalRunner_CopyFile_MissingSrcErrors(t *testing.T) {
	r := NewLocalRunner()
	err := r.CopyFile(context.Background(), "/does/not/exist", "/tmp/whatever", 0644)
	if err == nil {
		t.Error("expected error for missing src")
	}
}
