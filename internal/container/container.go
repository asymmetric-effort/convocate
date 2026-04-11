// Package container manages Docker container lifecycle for claude-shell sessions.
package container

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/asymmetric-effort/claude-shell/internal/config"
	"github.com/asymmetric-effort/claude-shell/internal/user"
)

// Runner executes Docker commands for session containers.
type Runner struct {
	sessionID  string
	sessionDir string
	userInfo   user.Info
	paths      config.Paths
	execFn     ExecFunc
}

// ExecFunc abstracts command execution for testing.
type ExecFunc func(name string, args ...string) *exec.Cmd

// DefaultExecFunc is the default command executor.
func DefaultExecFunc(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// NewRunner creates a new container Runner.
func NewRunner(sessionID, sessionDir string, userInfo user.Info, paths config.Paths) *Runner {
	return &Runner{
		sessionID:  sessionID,
		sessionDir: sessionDir,
		userInfo:   userInfo,
		paths:      paths,
		execFn:     DefaultExecFunc,
	}
}

// NewRunnerWithExec creates a Runner with a custom exec function (for testing).
func NewRunnerWithExec(sessionID, sessionDir string, userInfo user.Info, paths config.Paths, execFn ExecFunc) *Runner {
	return &Runner{
		sessionID:  sessionID,
		sessionDir: sessionDir,
		userInfo:   userInfo,
		paths:      paths,
		execFn:     execFn,
	}
}

// Start launches the container in detached mode and attaches to the tmux session.
func (r *Runner) Start() error {
	containerName := config.ContainerName(r.sessionID)

	args := r.buildRunArgs(containerName)

	cmd := r.execFn("docker", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start container: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return r.attachTmux()
}

// Stop gracefully stops the container.
func (r *Runner) Stop() error {
	containerName := config.ContainerName(r.sessionID)
	cmd := r.execFn("docker", "stop", "-t", "10", containerName)
	return cmd.Run()
}

// IsRunning checks if the session container is currently running.
func (r *Runner) IsRunning() (bool, error) {
	containerName := config.ContainerName(r.sessionID)
	cmd := r.execFn("docker", "inspect", "-f", "{{.State.Running}}", containerName)
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

// Attach attaches to the tmux session in a running container.
func (r *Runner) Attach() error {
	return r.attachTmux()
}

// attachTmux connects to the tmux session inside the container.
func (r *Runner) attachTmux() error {
	containerName := config.ContainerName(r.sessionID)
	cmd := r.execFn("docker", "exec", "-it",
		containerName,
		"sudo", "-E", "-u", "claude", "-H", "--",
		"tmux", "attach-session", "-t", config.TmuxSessionName,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ImageExists checks if the claude-shell Docker image exists.
func ImageExists(execFn ExecFunc) (bool, error) {
	if execFn == nil {
		execFn = DefaultExecFunc
	}
	cmd := execFn("docker", "image", "inspect", config.ContainerImage())
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func (r *Runner) buildRunArgs(containerName string) []string {
	args := []string{
		"run",
		"--rm",
		"--detach",
		"--name", containerName,
		"--hostname", fmt.Sprintf("claude-%s", r.sessionID[:8]),
	}

	// Session home directory
	args = append(args, "-v", fmt.Sprintf("%s:/home/claude", r.sessionDir))

	// Shared claude config (read-only)
	args = append(args, "-v", fmt.Sprintf("%s:/home/claude/%s:ro", r.paths.ClaudeConfig, config.ClaudeSharedDir))

	// Docker socket
	args = append(args, "-v", fmt.Sprintf("%s:%s", config.DockerSocket, config.DockerSocket))

	// SSH keys (read-only)
	if _, err := os.Stat(r.paths.SSHDir); err == nil {
		args = append(args, "-v", fmt.Sprintf("%s:/home/claude/.ssh:ro", r.paths.SSHDir))
	}

	// Git config (read-only)
	if _, err := os.Stat(r.paths.GitConfig); err == nil {
		args = append(args, "-v", fmt.Sprintf("%s:/home/claude/.gitconfig:ro", r.paths.GitConfig))
	}

	// Claude binary (read-only)
	args = append(args, "-v", fmt.Sprintf("%s:%s:ro", config.ClaudeBinaryPath, config.ClaudeBinaryPath))

	// Environment variables for user setup
	args = append(args, "-e", fmt.Sprintf("CLAUDE_UID=%d", r.userInfo.UID))
	args = append(args, "-e", fmt.Sprintf("CLAUDE_GID=%d", r.userInfo.GID))

	// Image and entrypoint
	args = append(args, config.ContainerImage())

	return args
}
