//go:build e2e

// Package e2e provides end-to-end tests for claude-shell using Docker containers.
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
	testImageName = "claude-shell-e2e-test"
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
RUN apt-get update && apt-get install -y --no-install-recommends sudo locales \
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
	return "/root/git/claude-shell"
}

func TestContainerStarts(t *testing.T) {
	containerName := fmt.Sprintf("e2e-start-%d", time.Now().UnixNano())

	cmd := exec.Command("docker", "run",
		"--rm",
		"-i",
		"--name", containerName,
		"-e", "CLAUDE_UID=1000",
		"-e", "CLAUDE_GID=1000",
		testImageName+":"+testImageTag,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = strings.NewReader("exit\n")

	if err := cmd.Run(); err != nil {
		t.Fatalf("container failed to start: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Claude CLI (mock) - Ready") {
		t.Errorf("expected mock claude greeting in output, got: %s", output)
	}
	if !strings.Contains(output, "Goodbye!") {
		t.Errorf("expected goodbye message in output, got: %s", output)
	}
}

func TestContainerUserSetup(t *testing.T) {
	containerName := fmt.Sprintf("e2e-user-%d", time.Now().UnixNano())

	cmd := exec.Command("docker", "run",
		"--rm",
		"-i",
		"--name", containerName,
		"-e", "CLAUDE_UID=1337",
		"-e", "CLAUDE_GID=1337",
		testImageName+":"+testImageTag,
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stdin = strings.NewReader("exit\n")

	if err := cmd.Run(); err != nil {
		t.Fatalf("container failed: %v", err)
	}

	output := stdout.String()

	// Check the mock claude reports the user as claude
	if !strings.Contains(output, "User: claude") {
		t.Errorf("expected User: claude, got: %s", output)
	}
	// Check the mock claude shows a home directory
	if !strings.Contains(output, "Home: /home/claude") {
		t.Errorf("expected Home: /home/claude, got: %s", output)
	}
}

func TestContainerEchoIO(t *testing.T) {
	containerName := fmt.Sprintf("e2e-io-%d", time.Now().UnixNano())

	input := "hello world\ntest message\nexit\n"

	cmd := exec.Command("docker", "run",
		"--rm",
		"-i",
		"--name", containerName,
		"-e", "CLAUDE_UID=1000",
		"-e", "CLAUDE_GID=1000",
		testImageName+":"+testImageTag,
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stdin = strings.NewReader(input)

	if err := cmd.Run(); err != nil {
		t.Fatalf("container failed: %v", err)
	}

	output := stdout.String()
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
		"-e", "CLAUDE_UID=1000",
		"-e", "CLAUDE_GID=1000",
		testImageName+":"+testImageTag,
		"-c", `
			# Run entrypoint setup manually (just user creation part)
			groupadd -g 1000 claude 2>/dev/null || groupadd claude
			useradd -u 1000 -g claude -d /home/claude -s /bin/bash -m claude 2>/dev/null || useradd -g claude -d /home/claude -s /bin/bash -m claude
			echo "claude ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/claude
			chmod 440 /etc/sudoers.d/claude
			mkdir -p /home/claude
			chown claude:claude /home/claude
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
		"-e", "CLAUDE_UID=1000",
		"-e", "CLAUDE_GID=1000",
		"-v", sessionDir+":/home/claude",
		testImageName+":"+testImageTag,
		"-c", "cat /home/claude/test.txt",
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

func TestContainerEphemeral(t *testing.T) {
	containerName := fmt.Sprintf("e2e-ephemeral-%d", time.Now().UnixNano())

	cmd := exec.Command("docker", "run",
		"--rm",
		"-i",
		"--name", containerName,
		"-e", "CLAUDE_UID=1000",
		"-e", "CLAUDE_GID=1000",
		testImageName+":"+testImageTag,
	)
	cmd.Stdin = strings.NewReader("exit\n")

	if err := cmd.Run(); err != nil {
		t.Fatalf("container failed: %v", err)
	}

	// Verify container no longer exists
	inspect := exec.Command("docker", "inspect", containerName)
	if err := inspect.Run(); err == nil {
		t.Error("container should have been removed (--rm)")
	}
}
