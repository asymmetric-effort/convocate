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
	port       int
	protocol   string
	dnsServer  string
	execFn     ExecFunc
}

// SetPort configures the port to publish from the container. A value of 0
// (the default) publishes no port.
func (r *Runner) SetPort(port int) {
	r.port = port
}

// SetProtocol configures the protocol used when publishing the port. An empty
// string is treated as "tcp".
func (r *Runner) SetProtocol(proto string) {
	r.protocol = proto
}

// SetDNSServer configures the DNS server the container should use for name
// resolution. When set to a non-empty IP, docker run is invoked with
// --dns <ip>, bypassing the default-bridge's auto-generated resolver config.
// This makes every container query the host's local dnsmasq (so session
// DNS names, cached lookups, and authoritative zones all resolve the same
// way a process on the host would).
//
// Empty (the default) leaves docker's usual DNS behavior in place.
func (r *Runner) SetDNSServer(ip string) {
	r.dnsServer = ip
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
	if err := r.StartDetached(); err != nil {
		return err
	}
	return r.attachTmux()
}

// StartDetached launches the container in detached mode without attaching any
// user terminal. The tmux session inside the container runs autonomously; a
// later call to Attach (or pressing Enter in the TUI) will connect to it.
func (r *Runner) StartDetached() error {
	containerName := config.ContainerName(r.sessionID)

	args := r.buildRunArgs(containerName)

	cmd := r.execFn("docker", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start container: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
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

// pkgExecFn is the package-level command executor used by standalone helpers
// (IsContainerRunning, StopContainer, DetachClients). Tests override it.
var pkgExecFn ExecFunc = DefaultExecFunc

// IsContainerRunning checks if the container for a given session ID is running.
func IsContainerRunning(sessionID string) bool {
	containerName := config.ContainerName(sessionID)
	cmd := pkgExecFn("docker", "inspect", "-f", "{{.State.Running}}", containerName)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// StopContainer stops the container for a given session ID.
func StopContainer(sessionID string) error {
	containerName := config.ContainerName(sessionID)
	cmd := pkgExecFn("docker", "stop", "-t", "10", containerName)
	return cmd.Run()
}

// DetachClients detaches all tmux clients attached to the session's tmux
// server inside the container. The container and tmux session keep running;
// only the user-facing terminal connections are dropped.
func DetachClients(sessionID string) error {
	containerName := config.ContainerName(sessionID)
	cmd := pkgExecFn("docker", "exec", containerName,
		"sudo", "-u", config.ClaudeUser, "--",
		"tmux", "detach-client", "-s", config.TmuxSessionName,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to detach tmux clients: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
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
		"-w", "/home/claude",
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

	// Published port (if any). Format: "-p HOST:CONTAINER/PROTO" where PROTO
	// is tcp or udp. Empty protocol is treated as tcp.
	if r.port > 0 {
		proto := r.protocol
		if proto == "" {
			proto = "tcp"
		}
		args = append(args, "-p", fmt.Sprintf("%d:%d/%s", r.port, r.port, proto))
	}

	// Point DNS at the host's local dnsmasq (when a resolver is configured).
	// Containers on the default bridge otherwise fall back to the daemon's
	// default resolvers, skipping the session DNS names we register.
	if r.dnsServer != "" {
		args = append(args, "--dns", r.dnsServer)
	}

	// Environment variables for user setup
	args = append(args, "-e", fmt.Sprintf("CLAUDE_UID=%d", r.userInfo.UID))
	args = append(args, "-e", fmt.Sprintf("CLAUDE_GID=%d", r.userInfo.GID))

	// Image and entrypoint
	args = append(args, config.ContainerImage())

	return args
}
