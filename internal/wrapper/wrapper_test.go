package wrapper

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// mockCommandRunner records commands for testing.
type mockCommandRunner struct {
	results  map[string]mockResult
	commands []mockCommand
	mu       sync.Mutex
}

type mockCommand struct {
	Name  string
	Stdin string
	Args  []string
	Env   []string
}

type mockResult struct {
	Err    error
	Output string
}

func newMockCommandRunner() *mockCommandRunner {
	return &mockCommandRunner{
		results: make(map[string]mockResult),
	}
}

func (m *mockCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	return m.record(ctx, nil, "", name, args...)
}

func (m *mockCommandRunner) RunWithEnv(ctx context.Context, env []string, name string, args ...string) (string, error) {
	return m.record(ctx, env, "", name, args...)
}

func (m *mockCommandRunner) RunWithStdin(ctx context.Context, stdin, name string, args ...string) (string, error) {
	return m.record(ctx, nil, stdin, name, args...)
}

func (m *mockCommandRunner) record(ctx context.Context, env []string, stdin, name string, args ...string) (string, error) {
	m.mu.Lock()
	cmd := mockCommand{Name: name, Args: args, Env: env, Stdin: stdin}
	m.commands = append(m.commands, cmd)

	key := name + " " + strings.Join(args, " ")
	result, hasResult := m.results[key]
	m.mu.Unlock()

	if hasResult {
		return result.Output, result.Err
	}

	// For claude commands (background tasks), block until context is cancelled.
	if name == "claude" {
		<-ctx.Done()
		return "", ctx.Err()
	}

	// Default: return empty success for any command.
	return "", nil
}

func (m *mockCommandRunner) setResult(name, output string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results[name] = mockResult{Output: output, Err: err}
}

func (m *mockCommandRunner) getCommands() []mockCommand {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]mockCommand, len(m.commands))
	copy(result, m.commands)
	return result
}

func testWrapper(t *testing.T) (*Wrapper, *mockCommandRunner) {
	t.Helper()
	dir := t.TempDir()
	runner := newMockCommandRunner()
	w, err := New(&Config{
		WorkspaceDir:  dir,
		SecretsSocket: "/tmp/test-secrets.sock",
		Logger:        log.New(io.Discard, "", 0),
		CmdRunner:     runner,
		ContainerID:   "test-container",
		HostID:        "test-host",
	})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	return w, runner
}

func TestNewWrapperValidation(t *testing.T) {
	tests := []struct {
		config Config
		name   string
	}{
		{name: "missing workspace dir", config: Config{SecretsSocket: "/tmp/s.sock"}},
		{name: "missing secrets socket", config: Config{WorkspaceDir: "/tmp/ws"}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := New(&testCase.config)
			if err == nil {
				t.Error("expected error for invalid config")
			}
		})
	}
}

func TestSetupSSH(t *testing.T) {
	w, _ := testWrapper(t)

	// Override home to a temp dir.
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Unsetenv("HOME")

	err := w.SetupSSH("-----BEGIN OPENSSH PRIVATE KEY-----\nfake-key-data\n-----END OPENSSH PRIVATE KEY-----")
	if err != nil {
		t.Fatalf("SetupSSH error: %v", err)
	}

	// Check the key file.
	keyPath := filepath.Join(tmpHome, ".ssh", "id_ed25519")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if !strings.Contains(string(data), "fake-key-data") {
		t.Error("key file doesn't contain expected data")
	}

	// Check permissions.
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("key permissions: got %o, want 600", info.Mode().Perm())
	}

	// Check known_hosts.
	knownHostsPath := filepath.Join(tmpHome, ".ssh", "known_hosts")
	khData, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatalf("read known_hosts: %v", err)
	}
	if !strings.Contains(string(khData), "github.com") {
		t.Error("known_hosts doesn't contain github.com")
	}
}

func TestSetupWorkspaceClone(t *testing.T) {
	w, runner := testWrapper(t)
	ctx := context.Background()

	err := w.SetupWorkspace(ctx, "org/repo")
	if err != nil {
		t.Fatalf("SetupWorkspace error: %v", err)
	}

	// Check that git clone was called.
	cmds := runner.getCommands()
	if len(cmds) < 1 {
		t.Fatal("expected at least 1 command")
	}
	if cmds[0].Name != "git" {
		t.Errorf("expected git command, got %q", cmds[0].Name)
	}
}

func TestSetupWorkspaceFetch(t *testing.T) {
	w, runner := testWrapper(t)
	ctx := context.Background()

	// Create a .git dir to simulate existing clone.
	gitDir := filepath.Join(w.workspaceDir, ".git")
	os.MkdirAll(gitDir, 0o755)

	err := w.SetupWorkspace(ctx, "org/repo")
	if err != nil {
		t.Fatalf("SetupWorkspace error: %v", err)
	}

	cmds := runner.getCommands()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	// Should be git fetch, not clone.
	found := false
	for _, arg := range cmds[0].Args {
		if arg == "fetch" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected git fetch, got: %v", cmds[0].Args)
	}
}

func TestCreateWorktreeIssue(t *testing.T) {
	w, _ := testWrapper(t)
	ctx := context.Background()
	jobID := uuid.MustNew()

	dir, branch, err := w.CreateWorktree(ctx, jobID, 42)
	if err != nil {
		t.Fatalf("CreateWorktree error: %v", err)
	}

	if dir == "" {
		t.Error("worktree dir is empty")
	}
	if branch != "feature/issue-42" {
		t.Errorf("branch: got %q, want %q", branch, "feature/issue-42")
	}
	if !strings.Contains(dir, jobID.String()) {
		t.Errorf("dir should contain job ID: %q", dir)
	}
}

func TestCreateWorktreeAdHoc(t *testing.T) {
	w, _ := testWrapper(t)
	ctx := context.Background()
	jobID := uuid.MustNew()

	_, branch, err := w.CreateWorktree(ctx, jobID, 0)
	if err != nil {
		t.Fatalf("CreateWorktree error: %v", err)
	}

	if !strings.HasPrefix(branch, "feature/adhoc-") {
		t.Errorf("branch: got %q, want feature/adhoc-*", branch)
	}
}

func TestRunBackgroundTaskAndCancel(t *testing.T) {
	w, _ := testWrapper(t)
	jobID := uuid.MustNew()

	w.RunBackgroundTask(jobID, "test prompt")

	// Wait briefly for task to start.
	time.Sleep(10 * time.Millisecond)

	if w.ActiveTaskCount() != 1 {
		t.Errorf("ActiveTaskCount: got %d, want 1", w.ActiveTaskCount())
	}

	cancelled := w.CancelTask(jobID)
	if !cancelled {
		t.Error("CancelTask returned false")
	}

	// Wait for cleanup.
	time.Sleep(10 * time.Millisecond)

	if w.ActiveTaskCount() != 0 {
		t.Errorf("ActiveTaskCount after cancel: got %d, want 0", w.ActiveTaskCount())
	}
}

func TestCancelNonexistentTask(t *testing.T) {
	w, _ := testWrapper(t)
	cancelled := w.CancelTask(uuid.MustNew())
	if cancelled {
		t.Error("CancelTask should return false for nonexistent task")
	}
}

func TestPostIssueComment(t *testing.T) {
	w, runner := testWrapper(t)
	ctx := context.Background()

	err := w.PostIssueComment(ctx, "ghp_test", "org/repo", 42, "Starting work on this issue.")
	if err != nil {
		t.Fatalf("PostIssueComment error: %v", err)
	}

	cmds := runner.getCommands()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Name != "gh" {
		t.Errorf("expected gh, got %q", cmds[0].Name)
	}
	// Verify GH_TOKEN is set.
	foundToken := false
	for _, env := range cmds[0].Env {
		if strings.HasPrefix(env, "GH_TOKEN=") {
			foundToken = true
			break
		}
	}
	if !foundToken {
		t.Error("GH_TOKEN not set in environment")
	}
}

func TestCreatePullRequest(t *testing.T) {
	w, runner := testWrapper(t)
	ctx := context.Background()

	prURL := "https://github.com/org/repo/pull/1"
	key := "gh pr create --repo org/repo --head feature/issue-42 --title Fix bug --body Closes #42"
	runner.setResult(key, prURL+"\n", nil)

	url, err := w.CreatePullRequest(ctx, "ghp_test", "org/repo", "feature/issue-42", "Fix bug", "Closes #42")
	if err != nil {
		t.Fatalf("CreatePullRequest error: %v", err)
	}
	if url != prURL {
		t.Errorf("PR URL: got %q, want %q", url, prURL)
	}
}

func TestRemoveLabel(t *testing.T) {
	w, runner := testWrapper(t)
	ctx := context.Background()

	err := w.RemoveLabel(ctx, "ghp_test", "org/repo", 42, "automated-development")
	if err != nil {
		t.Fatalf("RemoveLabel error: %v", err)
	}

	cmds := runner.getCommands()
	found := false
	for _, cmd := range cmds {
		for _, arg := range cmd.Args {
			if arg == "--remove-label" {
				found = true
			}
		}
	}
	if !found {
		t.Error("--remove-label not found in commands")
	}
}

func TestAssignIssue(t *testing.T) {
	w, runner := testWrapper(t)
	ctx := context.Background()

	err := w.AssignIssue(ctx, "ghp_test", "org/repo", 42, "alice")
	if err != nil {
		t.Fatalf("AssignIssue error: %v", err)
	}

	cmds := runner.getCommands()
	found := false
	for _, cmd := range cmds {
		for _, arg := range cmd.Args {
			if arg == "--add-assignee" {
				found = true
			}
		}
	}
	if !found {
		t.Error("--add-assignee not found in commands")
	}
}

func TestSendHeartbeat(t *testing.T) {
	w, _ := testWrapper(t)
	var buf bytes.Buffer
	err := w.SendHeartbeat(&buf)
	if err != nil {
		t.Fatalf("SendHeartbeat error: %v", err)
	}
	if !strings.Contains(buf.String(), "heartbeat") {
		t.Errorf("heartbeat output: got %q", buf.String())
	}
}

func TestDeleteBranch(t *testing.T) {
	w, runner := testWrapper(t)
	ctx := context.Background()

	err := w.DeleteBranch(ctx, "feature/issue-42")
	if err != nil {
		t.Fatalf("DeleteBranch error: %v", err)
	}

	cmds := runner.getCommands()
	if len(cmds) < 2 {
		t.Fatalf("expected at least 2 commands (local + remote delete), got %d", len(cmds))
	}
}

func TestPruneWorktree(t *testing.T) {
	w, _ := testWrapper(t)
	ctx := context.Background()
	jobID := uuid.MustNew()

	err := w.PruneWorktree(ctx, jobID)
	if err != nil {
		t.Fatalf("PruneWorktree error: %v", err)
	}
}

func TestAppendToFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")

	err := appendToFile(path, "line 1\n")
	if err != nil {
		t.Fatalf("appendToFile error: %v", err)
	}
	err = appendToFile(path, "line 2\n")
	if err != nil {
		t.Fatalf("appendToFile error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != "line 1\nline 2\n" {
		t.Errorf("got %q", string(data))
	}
}

func TestMultipleBackgroundTasks(t *testing.T) {
	w, _ := testWrapper(t)

	jobIDs := make([]uuid.UUID, 0, 5)
	for range 5 {
		id := uuid.MustNew()
		jobIDs = append(jobIDs, id)
		w.RunBackgroundTask(id, "task prompt")
	}

	time.Sleep(20 * time.Millisecond)
	count := w.ActiveTaskCount()
	if count != 5 {
		t.Errorf("ActiveTaskCount: got %d, want 5", count)
	}

	// Clean up — cancel all tasks.
	for _, id := range jobIDs {
		w.CancelTask(id)
	}
	time.Sleep(20 * time.Millisecond)
}

func TestDecodeSecrets(t *testing.T) {
	w, _ := testWrapper(t)

	t.Run("valid response", func(t *testing.T) {
		data := `{"ssh_private_key":"key","github_pat":"pat","custom_secrets":{"X":"Y"}}`
		resp, err := w.DecodeSecrets(strings.NewReader(data))
		if err != nil {
			t.Fatalf("DecodeSecrets error: %v", err)
		}
		if resp.SSHPrivateKey != "key" {
			t.Errorf("SSHPrivateKey: got %q", resp.SSHPrivateKey)
		}
		if resp.GitHubPAT != "pat" {
			t.Errorf("GitHubPAT: got %q", resp.GitHubPAT)
		}
		if resp.CustomSecrets["X"] != "Y" {
			t.Errorf("CustomSecrets: got %v", resp.CustomSecrets)
		}
	})

	t.Run("error response", func(t *testing.T) {
		data := `{"error":"not bound"}`
		_, err := w.DecodeSecrets(strings.NewReader(data))
		if err == nil {
			t.Error("expected error for error response")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := w.DecodeSecrets(strings.NewReader("not json"))
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestReadPrompts(t *testing.T) {
	w, _ := testWrapper(t)
	ctx := context.Background()

	input := "first prompt\nsecond prompt\n\nthird prompt\n"
	var received []string
	w.ReadPrompts(ctx, strings.NewReader(input), func(prompt string) {
		received = append(received, prompt)
	})

	if len(received) != 3 {
		t.Fatalf("expected 3 prompts, got %d: %v", len(received), received)
	}
	if received[0] != "first prompt" {
		t.Errorf("prompt[0]: got %q", received[0])
	}
	if received[1] != "second prompt" {
		t.Errorf("prompt[1]: got %q", received[1])
	}
	if received[2] != "third prompt" {
		t.Errorf("prompt[2]: got %q", received[2])
	}
}

func TestReadPromptsWithCancellation(t *testing.T) {
	w, _ := testWrapper(t)
	ctx, cancel := context.WithCancel(context.Background())

	// Use a finite input that ends, so scanner.Scan returns false.
	input := "prompt 1\nprompt 2\n"
	var received []string
	w.ReadPrompts(ctx, strings.NewReader(input), func(prompt string) {
		received = append(received, prompt)
		if len(received) == 1 {
			cancel() // Cancel after first prompt.
		}
	})

	// Should have stopped after 1 or 2 prompts depending on timing.
	if len(received) < 1 {
		t.Errorf("expected at least 1 prompt, got %d", len(received))
	}
}

func TestStartHeartbeatLoopWrapper(t *testing.T) {
	w, _ := testWrapper(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var buf bytes.Buffer
	w.StartHeartbeatLoop(ctx, &buf)

	// Should have exited cleanly (no panic).
}

func TestNewRealCommandRunner(t *testing.T) {
	runner := NewRealCommandRunner()
	if runner == nil {
		t.Error("NewRealCommandRunner returned nil")
	}
}
