//go:build e2e

// Package e2e provides end-to-end tests for convocate using Docker containers.
package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testImageName = "convocate-e2e-test"
	testImageTag  = "latest"
)

func TestMain(m *testing.M) {
	// Build the mock claude binary
	if err := buildMockClaude(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build mock claude: %v\n", err)
		os.Exit(1)
	}

	// Build the test Docker image
	if err := buildTestImage(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build test image: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	// Cleanup
	_ = exec.Command("docker", "rmi", testImageName+":"+testImageTag).Run()

	os.Exit(code)
}

func buildMockClaude() error {
	cmd := exec.Command("go", "build", "-o", "test/e2e/mock_claude/mock-claude", "./test/e2e/mock_claude/")
	cmd.Dir = findProjectRoot()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildTestImage() error {
	projectRoot := findProjectRoot()
	dockerfileContent := `FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends sudo locales tmux \
    && locale-gen en_US.UTF-8 \
    && rm -rf /var/lib/apt/lists/*
ENV LANG=en_US.UTF-8
COPY test/e2e/mock_claude/mock-claude /usr/local/bin/claude
RUN chmod +x /usr/local/bin/claude
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
`
	tmpDockerfile := filepath.Join(projectRoot, "Dockerfile.e2e")
	if err := os.WriteFile(tmpDockerfile, []byte(dockerfileContent), 0644); err != nil {
		return err
	}
	defer os.Remove(tmpDockerfile)

	cmd := exec.Command("docker", "build",
		"-t", testImageName+":"+testImageTag,
		"-f", tmpDockerfile,
		projectRoot,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Fallback
	return "/root/git/convocate"
}

// startDetachedContainer starts a container in detached mode and returns its name.
// The container runs the entrypoint which starts Claude inside tmux.
func startDetachedContainer(t *testing.T, prefix string, extraArgs ...string) string {
	t.Helper()
	containerName := fmt.Sprintf("e2e-%s-%d", prefix, time.Now().UnixNano())

	args := []string{
		"run", "--detach", "--rm",
		"--name", containerName,
		"-e", "CONVOCATE_UID=1000",
		"-e", "CONVOCATE_GID=1000",
	}
	args = append(args, extraArgs...)
	args = append(args, testImageName+":"+testImageTag)

	cmd := exec.Command("docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if out, err := cmd.Output(); err != nil {
		t.Fatalf("failed to start container: %v\nstderr: %s\nout: %s", err, stderr.String(), string(out))
	}

	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})

	// Wait for tmux session to be ready
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		check := exec.Command("docker", "exec", containerName,
			"sudo", "-u", "convocate", "tmux", "has-session", "-t", "convocate")
		if check.Run() == nil {
			return containerName
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("tmux session did not start within timeout")
	return ""
}

// execInTmux sends keys to the tmux session and captures pane output.
func execInTmux(containerName, keys string) error {
	cmd := exec.Command("docker", "exec", containerName,
		"sudo", "-u", "convocate", "tmux", "send-keys", "-t", "convocate", keys, "Enter")
	return cmd.Run()
}

// captureTmuxPane captures the current tmux pane content.
func captureTmuxPane(containerName string) (string, error) {
	cmd := exec.Command("docker", "exec", containerName,
		"sudo", "-u", "convocate", "tmux", "capture-pane", "-t", "convocate", "-p")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func TestContainerStarts(t *testing.T) {
	containerName := startDetachedContainer(t, "start")

	// The mock claude should have printed its greeting in the tmux pane
	output, err := captureTmuxPane(containerName)
	if err != nil {
		t.Fatalf("failed to capture tmux pane: %v", err)
	}

	if !strings.Contains(output, "Claude CLI (mock) - Ready") {
		t.Errorf("expected mock claude greeting in output, got: %s", output)
	}
}

func TestContainerUserSetup(t *testing.T) {
	containerName := fmt.Sprintf("e2e-user-%d", time.Now().UnixNano())

	args := []string{
		"run", "--detach", "--rm",
		"--name", containerName,
		"-e", "CONVOCATE_UID=1337",
		"-e", "CONVOCATE_GID=1337",
		testImageName + ":" + testImageTag,
	}

	cmd := exec.Command("docker", args...)
	if _, err := cmd.Output(); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})

	// Wait for tmux session
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		check := exec.Command("docker", "exec", containerName,
			"sudo", "-u", "convocate", "tmux", "has-session", "-t", "convocate")
		if check.Run() == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	output, err := captureTmuxPane(containerName)
	if err != nil {
		t.Fatalf("failed to capture tmux pane: %v", err)
	}

	if !strings.Contains(output, "User: convocate") {
		t.Errorf("expected User: convocate, got: %s", output)
	}
	if !strings.Contains(output, "Home: /home/convocate") {
		t.Errorf("expected Home: /home/convocate, got: %s", output)
	}
}

func TestContainerEchoIO(t *testing.T) {
	containerName := startDetachedContainer(t, "io")

	// Send input via tmux
	if err := execInTmux(containerName, "hello world"); err != nil {
		t.Fatalf("failed to send keys: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	if err := execInTmux(containerName, "test message"); err != nil {
		t.Fatalf("failed to send keys: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	output, err := captureTmuxPane(containerName)
	if err != nil {
		t.Fatalf("failed to capture tmux pane: %v", err)
	}

	if !strings.Contains(output, "echo: hello world") {
		t.Errorf("expected echo of input, got: %s", output)
	}
	if !strings.Contains(output, "echo: test message") {
		t.Errorf("expected echo of second input, got: %s", output)
	}
}

func TestContainerSudoAccess(t *testing.T) {
	containerName := fmt.Sprintf("e2e-sudo-%d", time.Now().UnixNano())

	// Override entrypoint to test sudo directly
	cmd := exec.Command("docker", "run",
		"--rm",
		"--name", containerName,
		"--entrypoint", "/bin/bash",
		"-e", "CONVOCATE_UID=1000",
		"-e", "CONVOCATE_GID=1000",
		testImageName+":"+testImageTag,
		"-c", `
			# Run entrypoint setup manually (just user creation part)
			groupadd -g 1000 claude 2>/dev/null || groupadd claude
			useradd -u 1337 -m -s /bin/bash convocate
			echo "claude ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/claude
			chmod 440 /etc/sudoers.d/claude
			mkdir -p /home/convocate
			chown claude:claude /home/convocate
			# Test that sudo works without password
			su -l claude -c "sudo whoami"
		`,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("sudo test failed: %v\nstderr: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output != "root" {
		t.Errorf("expected sudo whoami to return 'root', got: %q", output)
	}
}

func TestContainerSessionMount(t *testing.T) {
	containerName := fmt.Sprintf("e2e-mount-%d", time.Now().UnixNano())
	sessionDir := t.TempDir()

	// Create a test file in the session directory
	testContent := "session data test"
	if err := os.WriteFile(filepath.Join(sessionDir, "test.txt"), []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("docker", "run",
		"--rm",
		"--name", containerName,
		"--entrypoint", "/bin/bash",
		"-e", "CONVOCATE_UID=1000",
		"-e", "CONVOCATE_GID=1000",
		"-v", sessionDir+":/home/convocate",
		testImageName+":"+testImageTag,
		"-c", "cat /home/convocate/test.txt",
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("mount test failed: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != testContent {
		t.Errorf("expected %q, got %q", testContent, stdout.String())
	}
}

func TestContainerTmuxSession(t *testing.T) {
	containerName := startDetachedContainer(t, "tmux")

	// Verify tmux session exists with correct name
	cmd := exec.Command("docker", "exec", containerName,
		"sudo", "-u", "convocate", "tmux", "list-sessions")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to list tmux sessions: %v", err)
	}
	if !strings.Contains(string(out), "claude:") {
		t.Errorf("expected tmux session named 'claude', got: %s", string(out))
	}
}

func TestContainerTmuxPersistsAfterDetach(t *testing.T) {
	containerName := startDetachedContainer(t, "persist")

	// Send some input to create state
	if err := execInTmux(containerName, "hello persistence"); err != nil {
		t.Fatalf("failed to send keys: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	// Verify the input was echoed (state exists)
	output, err := captureTmuxPane(containerName)
	if err != nil {
		t.Fatalf("failed to capture pane: %v", err)
	}
	if !strings.Contains(output, "echo: hello persistence") {
		t.Errorf("expected echoed input before detach, got: %s", output)
	}

	// Verify container is still running (tmux keeps it alive)
	inspect := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerName)
	inspectOut, err := inspect.Output()
	if err != nil {
		t.Fatalf("failed to inspect container: %v", err)
	}
	if strings.TrimSpace(string(inspectOut)) != "true" {
		t.Error("container should still be running after detach")
	}

	// Verify tmux session still exists
	check := exec.Command("docker", "exec", containerName,
		"sudo", "-u", "convocate", "tmux", "has-session", "-t", "convocate")
	if err := check.Run(); err != nil {
		t.Error("tmux session should still exist after detach")
	}
}

func TestContainerStopsWhenClaudeExits(t *testing.T) {
	containerName := startDetachedContainer(t, "exit")

	// Send "exit" to claude via tmux
	if err := execInTmux(containerName, "exit"); err != nil {
		t.Fatalf("failed to send exit: %v", err)
	}

	// Wait for container to stop (tmux session ends → entrypoint loop exits → container stops)
	deadline := time.Now().Add(10 * time.Second)
	stopped := false
	for time.Now().Before(deadline) {
		inspect := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerName)
		out, err := inspect.Output()
		if err != nil || strings.TrimSpace(string(out)) != "true" {
			stopped = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !stopped {
		t.Error("container should stop after claude exits")
	}
}

func TestContainerEphemeral(t *testing.T) {
	containerName := startDetachedContainer(t, "ephemeral")

	// Send "exit" to claude
	if err := execInTmux(containerName, "exit"); err != nil {
		t.Fatalf("failed to send exit: %v", err)
	}

	// Wait for container to be removed (--rm flag)
	deadline := time.Now().Add(10 * time.Second)
	removed := false
	for time.Now().Before(deadline) {
		inspect := exec.Command("docker", "inspect", containerName)
		if inspect.Run() != nil {
			removed = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !removed {
		t.Error("container should have been removed (--rm)")
	}
}
