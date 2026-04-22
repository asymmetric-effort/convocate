package container

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asymmetric-effort/claude-shell/internal/config"
	"github.com/asymmetric-effort/claude-shell/internal/user"
)

func testUserInfo() user.Info {
	return user.Info{
		UID:      1337,
		GID:      1337,
		Username: "claude",
		HomeDir:  "/home/claude",
	}
}

func testPaths() config.Paths {
	return config.Paths{
		ClaudeHome:   "/home/claude",
		SessionsBase: "/home/claude",
		SkelDir:      "/home/claude/.skel",
		ClaudeConfig: "/home/claude/.claude",
		SSHDir:       "/home/claude/.ssh",
		GitConfig:    "/home/claude/.gitconfig",
	}
}

func TestNewRunner(t *testing.T) {
	r := NewRunner("test-uuid", "/tmp/session", testUserInfo(), testPaths())
	if r == nil {
		t.Fatal("NewRunner returned nil")
	}
	if r.sessionID != "test-uuid" {
		t.Errorf("sessionID = %q, want %q", r.sessionID, "test-uuid")
	}
	if r.sessionDir != "/tmp/session" {
		t.Errorf("sessionDir = %q, want %q", r.sessionDir, "/tmp/session")
	}
}

func TestNewRunnerWithExec(t *testing.T) {
	mockExec := func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "mock")
	}
	r := NewRunnerWithExec("test-uuid", "/tmp/session", testUserInfo(), testPaths(), mockExec)
	if r == nil {
		t.Fatal("NewRunnerWithExec returned nil")
	}
}

func TestBuildRunArgs(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	gitConfig := filepath.Join(tmpDir, ".gitconfig")
	claudeConfig := filepath.Join(tmpDir, ".claude")

	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gitConfig, []byte("[user]"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(claudeConfig, 0700); err != nil {
		t.Fatal(err)
	}

	paths := config.Paths{
		ClaudeHome:   tmpDir,
		SessionsBase: tmpDir,
		SkelDir:      filepath.Join(tmpDir, ".skel"),
		ClaudeConfig: claudeConfig,
		SSHDir:       sshDir,
		GitConfig:    gitConfig,
	}

	sessionDir := filepath.Join(tmpDir, "test-session")
	r := NewRunner("abcdef12-3456-7890-abcd-ef1234567890", sessionDir, testUserInfo(), paths)

	args := r.buildRunArgs("claude-session-test")

	argStr := strings.Join(args, " ")

	checks := []struct {
		name    string
		pattern string
	}{
		{"--rm", "--rm"},
		{"--detach", "--detach"},
		{"-w", "-w /home/claude"},
		{"--name", "--name claude-session-test"},
		{"--hostname", "--hostname claude-abcdef12"},
		{"session home", sessionDir + ":/home/claude"},
		{"docker socket", config.DockerSocket + ":" + config.DockerSocket},
		{"SSH mount", sshDir + ":/home/claude/.ssh:ro"},
		{"gitconfig mount", gitConfig + ":/home/claude/.gitconfig:ro"},
		{"claude binary", config.ClaudeBinaryPath + ":" + config.ClaudeBinaryPath + ":ro"},
		{"CLAUDE_UID", "CLAUDE_UID=1337"},
		{"CLAUDE_GID", "CLAUDE_GID=1337"},
		{"image", config.ContainerImage()},
		{"claude-shared", config.ClaudeSharedDir + ":ro"},
	}

	for _, c := range checks {
		if !strings.Contains(argStr, c.pattern) {
			t.Errorf("missing %s in args: %s", c.name, argStr)
		}
	}
}

func TestBuildRunArgs_NoSSH(t *testing.T) {
	paths := config.Paths{
		ClaudeHome:   "/nonexistent",
		SessionsBase: "/nonexistent",
		ClaudeConfig: "/nonexistent/.claude",
		SSHDir:       "/nonexistent/.ssh",
		GitConfig:    "/nonexistent/.gitconfig",
	}

	r := NewRunner("test-uuid", "/tmp/session", testUserInfo(), paths)
	args := r.buildRunArgs("test-container")
	argStr := strings.Join(args, " ")

	if strings.Contains(argStr, ".ssh:ro") {
		t.Error("SSH should not be mounted when dir doesn't exist")
	}
	if strings.Contains(argStr, ".gitconfig:ro") {
		t.Error("gitconfig should not be mounted when file doesn't exist")
	}
}

func TestBuildRunArgs_DifferentUIDs(t *testing.T) {
	info := user.Info{UID: 5000, GID: 5000, Username: "testuser", HomeDir: "/home/testuser"}
	r := NewRunner("test-uuid", "/tmp/session", info, testPaths())
	args := r.buildRunArgs("test-container")
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "CLAUDE_UID=5000") {
		t.Error("missing CLAUDE_UID=5000")
	}
	if !strings.Contains(argStr, "CLAUDE_GID=5000") {
		t.Error("missing CLAUDE_GID=5000")
	}
}

func TestIsRunning_NotRunning(t *testing.T) {
	mockExec := func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	r := NewRunnerWithExec("test-uuid", "/tmp/session", testUserInfo(), testPaths(), mockExec)
	running, err := r.IsRunning()
	if err != nil {
		t.Fatalf("IsRunning failed: %v", err)
	}
	if running {
		t.Error("expected not running")
	}
}

func TestIsRunning_Running(t *testing.T) {
	mockExec := func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "true")
	}

	r := NewRunnerWithExec("test-uuid", "/tmp/session", testUserInfo(), testPaths(), mockExec)
	running, err := r.IsRunning()
	if err != nil {
		t.Fatalf("IsRunning failed: %v", err)
	}
	if !running {
		t.Error("expected running")
	}
}

func TestIsRunning_FalseOutput(t *testing.T) {
	mockExec := func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo", "false")
	}

	r := NewRunnerWithExec("test-uuid", "/tmp/session", testUserInfo(), testPaths(), mockExec)
	running, err := r.IsRunning()
	if err != nil {
		t.Fatalf("IsRunning failed: %v", err)
	}
	if running {
		t.Error("expected not running when output is 'false'")
	}
}

func TestImageExists_NotExists(t *testing.T) {
	mockExec := func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	exists, err := ImageExists(mockExec)
	if err != nil {
		t.Fatalf("ImageExists failed: %v", err)
	}
	if exists {
		t.Error("expected image not to exist")
	}
}

func TestImageExists_Exists(t *testing.T) {
	mockExec := func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	exists, err := ImageExists(mockExec)
	if err != nil {
		t.Fatalf("ImageExists failed: %v", err)
	}
	if !exists {
		t.Error("expected image to exist")
	}
}

func TestImageExists_NilExec(t *testing.T) {
	_, err := ImageExists(nil)
	if err != nil {
		t.Fatalf("ImageExists with nil exec should not error: %v", err)
	}
}

func TestStop_Success(t *testing.T) {
	mockExec := func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	r := NewRunnerWithExec("test-uuid", "/tmp/session", testUserInfo(), testPaths(), mockExec)
	err := r.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestStop_Failure(t *testing.T) {
	mockExec := func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	r := NewRunnerWithExec("test-uuid", "/tmp/session", testUserInfo(), testPaths(), mockExec)
	err := r.Stop()
	if err == nil {
		t.Error("expected error from failed stop")
	}
}

func TestDefaultExecFunc(t *testing.T) {
	cmd := DefaultExecFunc("echo", "test")
	if cmd == nil {
		t.Fatal("DefaultExecFunc returned nil")
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if strings.TrimSpace(string(out)) != "test" {
		t.Errorf("output = %q, want %q", string(out), "test")
	}
}

func TestStart_DockerRunFailure(t *testing.T) {
	mockExec := func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	r := NewRunnerWithExec("test-uuid-1234567890", "/tmp/session", testUserInfo(), testPaths(), mockExec)
	err := r.Start()
	if err == nil {
		t.Error("expected error from failed docker run")
	}
	if !strings.Contains(err.Error(), "failed to start container") {
		t.Errorf("expected 'failed to start container' error, got: %v", err)
	}
}

func TestStart_AttachTmuxArgs(t *testing.T) {
	var capturedCalls [][]string
	callCount := 0
	mockExec := func(name string, args ...string) *exec.Cmd {
		capturedCalls = append(capturedCalls, append([]string{name}, args...))
		callCount++
		if callCount == 1 {
			// docker run --detach succeeds
			return exec.Command("echo", "container-id")
		}
		// docker exec (attachTmux) - use "true" to succeed
		return exec.Command("true")
	}

	r := NewRunnerWithExec("test-uuid-1234567890", "/tmp/session", testUserInfo(), testPaths(), mockExec)
	err := r.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if len(capturedCalls) < 2 {
		t.Fatalf("expected at least 2 calls, got %d", len(capturedCalls))
	}

	// Verify first call is docker run --detach
	runArgs := capturedCalls[0]
	runStr := strings.Join(runArgs, " ")
	if !strings.Contains(runStr, "--detach") {
		t.Errorf("docker run should include --detach, got: %s", runStr)
	}
	if strings.Contains(runStr, "--interactive") {
		t.Errorf("docker run should not include --interactive, got: %s", runStr)
	}

	// Verify second call is docker exec with tmux attach
	execArgs := capturedCalls[1]
	execStr := strings.Join(execArgs, " ")
	if !strings.Contains(execStr, "exec") {
		t.Errorf("second call should be docker exec, got: %s", execStr)
	}
	if !strings.Contains(execStr, "tmux") {
		t.Errorf("exec should include tmux, got: %s", execStr)
	}
	if !strings.Contains(execStr, "attach-session") {
		t.Errorf("exec should include attach-session, got: %s", execStr)
	}
	if !strings.Contains(execStr, "-t claude") {
		t.Errorf("exec should target tmux session 'claude', got: %s", execStr)
	}
}

func TestAttach_UsesTmux(t *testing.T) {
	var capturedArgs []string
	mockExec := func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return exec.Command("true")
	}

	r := NewRunnerWithExec("test-uuid-1234567890", "/tmp/session", testUserInfo(), testPaths(), mockExec)
	err := r.Attach()
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	argStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(argStr, "docker exec -it") {
		t.Errorf("Attach should use 'docker exec -it', got: %s", argStr)
	}
	if !strings.Contains(argStr, "tmux attach-session -t claude") {
		t.Errorf("Attach should use 'tmux attach-session -t claude', got: %s", argStr)
	}
}

func TestAttach_Failure(t *testing.T) {
	mockExec := func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	r := NewRunnerWithExec("test-uuid-1234567890", "/tmp/session", testUserInfo(), testPaths(), mockExec)
	err := r.Attach()
	if err == nil {
		t.Error("expected error from failed attach")
	}
}

func TestBuildRunArgs_PortPublished(t *testing.T) {
	r := NewRunner("abcdef12-3456-7890-abcd-ef1234567890", "/tmp/session", testUserInfo(), testPaths())
	r.SetPort(8080)
	args := r.buildRunArgs("test-container")
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "-p 8080:8080") {
		t.Errorf("expected '-p 8080:8080' in args, got: %s", argStr)
	}
}

func TestBuildRunArgs_NoPortByDefault(t *testing.T) {
	r := NewRunner("abcdef12-3456-7890-abcd-ef1234567890", "/tmp/session", testUserInfo(), testPaths())
	args := r.buildRunArgs("test-container")
	argStr := strings.Join(args, " ")

	if strings.Contains(argStr, " -p ") {
		t.Errorf("expected no '-p' flag when port not set, got: %s", argStr)
	}
}

func TestBuildRunArgs_NoInteractiveTty(t *testing.T) {
	r := NewRunner("abcdef12-3456-7890-abcd-ef1234567890", "/tmp/session", testUserInfo(), testPaths())
	args := r.buildRunArgs("test-container")
	argStr := strings.Join(args, " ")

	if strings.Contains(argStr, "--interactive") {
		t.Error("buildRunArgs should not include --interactive")
	}
	if strings.Contains(argStr, "--tty") {
		t.Error("buildRunArgs should not include --tty")
	}
	if !strings.Contains(argStr, "--detach") {
		t.Error("buildRunArgs should include --detach")
	}
}
