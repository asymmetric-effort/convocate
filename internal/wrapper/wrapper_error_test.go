package wrapper

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// TestSetupWorkspaceCloneError tests SetupWorkspace when git clone fails.
func TestSetupWorkspaceCloneError(t *testing.T) {
	w, runner := testWrapper(t)
	ctx := context.Background()

	runner.setResult("git clone git@github.com:org/fail-repo.git "+w.workspaceDir,
		"", fmt.Errorf("clone failed"))

	err := w.SetupWorkspace(ctx, "org/fail-repo")
	if err == nil {
		t.Error("expected error when clone fails")
	}
}

// TestSetupWorkspaceFetchError tests SetupWorkspace when git fetch fails.
func TestSetupWorkspaceFetchError(t *testing.T) {
	w, runner := testWrapper(t)
	ctx := context.Background()

	// Create .git dir to trigger fetch path.
	createGitDir(t, w.workspaceDir)

	runner.setResult("git -C "+w.workspaceDir+" fetch origin",
		"", fmt.Errorf("fetch failed"))

	err := w.SetupWorkspace(ctx, "org/repo")
	if err == nil {
		t.Error("expected error when fetch fails")
	}
}

// TestCreateWorktreeError tests CreateWorktree when git worktree add fails.
func TestCreateWorktreeError(t *testing.T) {
	w, runner := testWrapper(t)
	ctx := context.Background()
	jobID := uuid.MustNew()

	worktreeDir := w.workspaceDir + "/jobs/" + jobID.String()
	runner.setResult("git -C "+w.workspaceDir+" worktree add -b feature/issue-1 "+worktreeDir+" origin/main",
		"", fmt.Errorf("worktree failed"))

	_, _, err := w.CreateWorktree(ctx, jobID, 1)
	if err == nil {
		t.Error("expected error when worktree creation fails")
	}
}

// TestPruneWorktreeError tests PruneWorktree when git worktree remove fails.
func TestPruneWorktreeError(t *testing.T) {
	w, runner := testWrapper(t)
	ctx := context.Background()
	jobID := uuid.MustNew()

	worktreeDir := w.workspaceDir + "/jobs/" + jobID.String()
	runner.setResult("git -C "+w.workspaceDir+" worktree remove "+worktreeDir,
		"", fmt.Errorf("prune failed"))

	err := w.PruneWorktree(ctx, jobID)
	if err == nil {
		t.Error("expected error when prune fails")
	}
}

// TestDeleteBranchLocalError tests DeleteBranch when local delete fails
// but remote delete succeeds.
func TestDeleteBranchLocalError(t *testing.T) {
	w, runner := testWrapper(t)
	ctx := context.Background()

	runner.setResult("git -C "+w.workspaceDir+" branch -D feature/broken",
		"", fmt.Errorf("local delete failed"))

	// Remote delete succeeds (default mock behavior).
	err := w.DeleteBranch(ctx, "feature/broken")
	// Local failure is logged but remote failure returns error.
	if err != nil {
		t.Errorf("DeleteBranch should succeed if remote delete succeeds: %v", err)
	}
}

// TestDeleteBranchRemoteError tests DeleteBranch when remote delete fails.
func TestDeleteBranchRemoteError(t *testing.T) {
	w, runner := testWrapper(t)
	ctx := context.Background()

	runner.setResult("git -C "+w.workspaceDir+" push origin --delete feature/fail",
		"", fmt.Errorf("remote delete failed"))

	err := w.DeleteBranch(ctx, "feature/fail")
	if err == nil {
		t.Error("expected error when remote delete fails")
	}
}

// TestCreatePullRequestError tests CreatePullRequest when gh fails.
func TestCreatePullRequestError(t *testing.T) {
	w, runner := testWrapper(t)
	ctx := context.Background()

	runner.setResult("gh pr create --repo org/repo --head feature/err --title T --body B",
		"", fmt.Errorf("pr create failed"))

	_, err := w.CreatePullRequest(ctx, "ghp_test", "org/repo", "feature/err", "T", "B")
	if err == nil {
		t.Error("expected error when PR creation fails")
	}
}

// TestNewWrapperDefaultLogger tests that nil logger gets replaced.
func TestNewWrapperDefaultLogger(t *testing.T) {
	w, err := New(&Config{
		WorkspaceDir:  t.TempDir(),
		SecretsSocket: "/tmp/test.sock",
		// Logger intentionally nil.
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if w.logger == nil {
		t.Error("logger should not be nil")
	}
	if w.cmdRunner == nil {
		t.Error("cmdRunner should not be nil")
	}
}

// TestRunBackgroundTaskError tests RunBackgroundTask when the command fails.
func TestRunBackgroundTaskError(t *testing.T) {
	dir := t.TempDir()
	runner := newMockCommandRunner()
	w, err := New(&Config{
		WorkspaceDir:  dir,
		SecretsSocket: "/tmp/test.sock",
		Logger:        log.New(io.Discard, "", 0),
		CmdRunner:     runner,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Configure claude to return error immediately.
	runner.setResult("claude --print", "", fmt.Errorf("claude failed"))

	jobID := uuid.MustNew()
	w.RunBackgroundTask(jobID, "test prompt")

	// Wait for task to fail.
	time.Sleep(50 * time.Millisecond)

	// Task should be removed from active tasks.
	if w.ActiveTaskCount() != 0 {
		t.Errorf("expected 0 active tasks after failure, got %d", w.ActiveTaskCount())
	}
}

// TestSetupSSHMkdirError tests SetupSSH when .ssh dir can't be created.
func TestSetupSSHMkdirError(t *testing.T) {
	w, _ := testWrapper(t)
	tmpHome := t.TempDir()
	// Create a file where .ssh should be, preventing mkdir.
	os.WriteFile(tmpHome+"/.ssh", []byte("file"), 0o600)
	os.Setenv("HOME", tmpHome)
	defer os.Unsetenv("HOME")

	err := w.SetupSSH("key-data")
	if err == nil {
		t.Error("expected error when .ssh dir can't be created")
	}
}

// TestSetupSSHWriteKeyError tests SetupSSH when key file can't be written.
func TestSetupSSHWriteKeyError(t *testing.T) {
	w, _ := testWrapper(t)
	tmpHome := t.TempDir()
	// Create .ssh as a directory, then make id_ed25519 a directory to prevent write.
	os.MkdirAll(tmpHome+"/.ssh/id_ed25519", 0o700)
	os.Setenv("HOME", tmpHome)
	defer os.Unsetenv("HOME")

	err := w.SetupSSH("key-data")
	if err == nil {
		t.Error("expected error when key file can't be written")
	}
}

// TestSetupSSHWriteKnownHostsError tests SetupSSH when known_hosts can't be written.
func TestSetupSSHWriteKnownHostsError(t *testing.T) {
	w, _ := testWrapper(t)
	tmpHome := t.TempDir()
	os.MkdirAll(tmpHome+"/.ssh", 0o700)
	// Make known_hosts a directory to prevent write.
	os.MkdirAll(tmpHome+"/.ssh/known_hosts", 0o700)
	os.Setenv("HOME", tmpHome)
	defer os.Unsetenv("HOME")

	err := w.SetupSSH("key-data")
	if err == nil {
		t.Error("expected error when known_hosts can't be written")
	}
}

// TestAppendToFileError tests appendToFile when path is invalid.
func TestAppendToFileError(t *testing.T) {
	err := appendToFile("/proc/nonexistent/file.txt", "data")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

// TestSetupSSHError tests SetupSSH when HOME dir resolution would fail
// or the .ssh dir can't be created (difficult to test without changing HOME).
// Instead, test the known_hosts append.
func TestSetupSSHAppend(t *testing.T) {
	w, _ := testWrapper(t)
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Unsetenv("HOME")

	// Call SetupSSH twice to test append behavior.
	err := w.SetupSSH("key1")
	if err != nil {
		t.Fatalf("SetupSSH first: %v", err)
	}
	err = w.SetupSSH("key2")
	if err != nil {
		t.Fatalf("SetupSSH second: %v", err)
	}
}

func createGitDir(t *testing.T, dir string) {
	t.Helper()
	err := os.MkdirAll(dir+"/.git", 0o755)
	if err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
}
