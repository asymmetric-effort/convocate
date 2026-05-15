package wrapper

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// Wrapper is the Agent Container entrypoint. It manages credentials,
// git worktrees, and Claude Code background tasks.
type Wrapper struct {
	cmdRunner     CommandRunner
	logger        *log.Logger
	activeTasks   map[uuid.UUID]context.CancelFunc
	workspaceDir  string
	secretsSocket string
	dispatchURL   string
	hostID        string
	containerID   string
	mu            sync.RWMutex
}

// Config holds the Wrapper configuration.
type Config struct {
	CmdRunner     CommandRunner
	Logger        *log.Logger
	WorkspaceDir  string
	SecretsSocket string
	DispatchURL   string
	HostID        string
	ContainerID   string
}

// CommandRunner abstracts command execution for testing.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
	RunWithEnv(ctx context.Context, env []string, name string, args ...string) (string, error)
	RunWithStdin(ctx context.Context, stdin, name string, args ...string) (string, error)
}

// SecretsResponse mirrors the broker's JSON response.
type SecretsResponse struct {
	SSHPrivateKey string            `json:"ssh_private_key,omitempty"`
	GitHubPAT     string            `json:"github_pat,omitempty"`
	CustomSecrets map[string]string `json:"custom_secrets,omitempty"`
	Error         string            `json:"error,omitempty"`
}

// New creates a new Wrapper.
func New(config *Config) (*Wrapper, error) {
	if config.WorkspaceDir == "" {
		return nil, fmt.Errorf("wrapper: workspace directory is required")
	}
	if config.SecretsSocket == "" {
		return nil, fmt.Errorf("wrapper: secrets socket is required")
	}
	logger := config.Logger
	if logger == nil {
		logger = log.Default()
	}
	runner := config.CmdRunner
	if runner == nil {
		runner = &realCommandRunner{}
	}
	return &Wrapper{
		workspaceDir:  config.WorkspaceDir,
		secretsSocket: config.SecretsSocket,
		logger:        logger,
		activeTasks:   make(map[uuid.UUID]context.CancelFunc),
		dispatchURL:   config.DispatchURL,
		hostID:        config.HostID,
		containerID:   config.ContainerID,
		cmdRunner:     runner,
	}, nil
}

// FetchSecrets reads project secrets from the Secrets Broker socket.
func (w *Wrapper) FetchSecrets() (*SecretsResponse, error) {
	return w.FetchSecretsFrom(w.secretsSocket)
}

// FetchSecretsFrom reads secrets from a given Unix socket path.
// Separated for testability.
func (w *Wrapper) FetchSecretsFrom(socketPath string) (*SecretsResponse, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("wrapper: connect to secrets socket: %w", err)
	}
	defer conn.Close()
	return w.DecodeSecrets(conn)
}

// DecodeSecrets reads and validates a secrets response from a reader.
func (w *Wrapper) DecodeSecrets(reader io.Reader) (*SecretsResponse, error) {
	var resp SecretsResponse
	err := json.NewDecoder(reader).Decode(&resp)
	if err != nil {
		return nil, fmt.Errorf("wrapper: decode secrets: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("wrapper: secrets broker error: %s", resp.Error)
	}
	return &resp, nil
}

// SetupSSH writes the SSH private key and configures known_hosts for github.com.
func (w *Wrapper) SetupSSH(sshPrivateKey string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("wrapper: get home dir: %w", err)
	}
	sshDir := filepath.Join(home, ".ssh")
	err = os.MkdirAll(sshDir, 0o700)
	if err != nil {
		return fmt.Errorf("wrapper: create .ssh dir: %w", err)
	}

	keyPath := filepath.Join(sshDir, "id_ed25519")
	err = os.WriteFile(keyPath, []byte(sshPrivateKey), 0o600)
	if err != nil {
		return fmt.Errorf("wrapper: write SSH key: %w", err)
	}

	// Add github.com to known_hosts.
	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	githubKey := "github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl\n"
	err = appendToFile(knownHostsPath, githubKey)
	if err != nil {
		return fmt.Errorf("wrapper: write known_hosts: %w", err)
	}

	return nil
}

// SetupWorkspace initializes or reuses the workspace git clone.
func (w *Wrapper) SetupWorkspace(ctx context.Context, repository string) error {
	gitDir := filepath.Join(w.workspaceDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		// Clone the repository.
		repoURL := fmt.Sprintf("git@github.com:%s.git", repository)
		_, err := w.cmdRunner.Run(ctx, "git", "clone", repoURL, w.workspaceDir)
		if err != nil {
			return fmt.Errorf("wrapper: git clone: %w", err)
		}
	} else {
		// Fetch latest.
		_, err := w.cmdRunner.Run(ctx, "git", "-C", w.workspaceDir, "fetch", "origin")
		if err != nil {
			return fmt.Errorf("wrapper: git fetch: %w", err)
		}
	}
	return nil
}

// CreateWorktree creates a per-job git worktree and feature branch.
func (w *Wrapper) CreateWorktree(ctx context.Context, jobID uuid.UUID, issueNumber int) (worktreeDir, branchName string, err error) {
	worktreeDir = filepath.Join(w.workspaceDir, "jobs", jobID.String())

	if issueNumber > 0 {
		branchName = fmt.Sprintf("feature/issue-%d", issueNumber)
	} else {
		branchName = fmt.Sprintf("feature/adhoc-%s", jobID.String()[:8])
	}

	_, err = w.cmdRunner.Run(ctx, "git", "-C", w.workspaceDir,
		"worktree", "add", "-b", branchName, worktreeDir, "origin/main")
	if err != nil {
		return "", "", fmt.Errorf("wrapper: create worktree: %w", err)
	}

	return worktreeDir, branchName, nil
}

// PruneWorktree removes a job's worktree.
func (w *Wrapper) PruneWorktree(ctx context.Context, jobID uuid.UUID) error {
	worktreeDir := filepath.Join(w.workspaceDir, "jobs", jobID.String())
	_, err := w.cmdRunner.Run(ctx, "git", "-C", w.workspaceDir,
		"worktree", "remove", worktreeDir)
	if err != nil {
		return fmt.Errorf("wrapper: prune worktree: %w", err)
	}
	return nil
}

// DeleteBranch deletes a branch locally and remotely.
func (w *Wrapper) DeleteBranch(ctx context.Context, branchName string) error {
	_, err := w.cmdRunner.Run(ctx, "git", "-C", w.workspaceDir,
		"branch", "-D", branchName)
	if err != nil {
		w.logger.Printf("wrapper: delete local branch %s: %v", branchName, err)
	}

	_, err = w.cmdRunner.Run(ctx, "git", "-C", w.workspaceDir,
		"push", "origin", "--delete", branchName)
	if err != nil {
		return fmt.Errorf("wrapper: delete remote branch: %w", err)
	}
	return nil
}

// RunBackgroundTask accepts a prompt and runs it as a background task
// within the wrapper. Each task gets its own context for cancellation.
func (w *Wrapper) RunBackgroundTask(jobID uuid.UUID, prompt string) {
	ctx, cancel := context.WithCancel(context.Background())

	w.mu.Lock()
	w.activeTasks[jobID] = cancel
	w.mu.Unlock()

	go func() {
		defer func() {
			w.mu.Lock()
			delete(w.activeTasks, jobID)
			w.mu.Unlock()
		}()

		w.logger.Printf("wrapper: starting background task %s", jobID)
		_, err := w.cmdRunner.RunWithStdin(ctx, prompt, "claude", "--print")
		if err != nil {
			if ctx.Err() != nil {
				w.logger.Printf("wrapper: task %s cancelled", jobID)
				return
			}
			w.logger.Printf("wrapper: task %s failed: %v", jobID, err)
			return
		}
		w.logger.Printf("wrapper: task %s completed", jobID)
	}()
}

// CancelTask cancels a running background task.
func (w *Wrapper) CancelTask(jobID uuid.UUID) bool {
	w.mu.Lock()
	cancel, exists := w.activeTasks[jobID]
	if exists {
		cancel()
		delete(w.activeTasks, jobID)
	}
	w.mu.Unlock()
	return exists
}

// ActiveTaskCount returns the number of running tasks.
func (w *Wrapper) ActiveTaskCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.activeTasks)
}

// PostIssueComment posts a comment on a GitHub issue using the gh CLI.
func (w *Wrapper) PostIssueComment(ctx context.Context, pat, repository string, issueNumber int, body string) error {
	env := []string{fmt.Sprintf("GH_TOKEN=%s", pat)}
	_, err := w.cmdRunner.RunWithEnv(ctx, env, "gh", "issue", "comment",
		fmt.Sprintf("%d", issueNumber),
		"--repo", repository,
		"--body", body)
	return err
}

// CreatePullRequest creates a PR using the gh CLI.
func (w *Wrapper) CreatePullRequest(ctx context.Context, pat, repository, branchName, title, body string) (string, error) {
	env := []string{fmt.Sprintf("GH_TOKEN=%s", pat)}
	output, err := w.cmdRunner.RunWithEnv(ctx, env, "gh", "pr", "create",
		"--repo", repository,
		"--head", branchName,
		"--title", title,
		"--body", body)
	if err != nil {
		return "", fmt.Errorf("wrapper: create PR: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// RemoveLabel removes a label from a GitHub issue.
func (w *Wrapper) RemoveLabel(ctx context.Context, pat, repository string, issueNumber int, label string) error {
	env := []string{fmt.Sprintf("GH_TOKEN=%s", pat)}
	_, err := w.cmdRunner.RunWithEnv(ctx, env, "gh", "issue", "edit",
		fmt.Sprintf("%d", issueNumber),
		"--repo", repository,
		"--remove-label", label)
	return err
}

// AssignIssue assigns an issue to a user.
func (w *Wrapper) AssignIssue(ctx context.Context, pat, repository string, issueNumber int, assignee string) error {
	env := []string{fmt.Sprintf("GH_TOKEN=%s", pat)}
	_, err := w.cmdRunner.RunWithEnv(ctx, env, "gh", "issue", "edit",
		fmt.Sprintf("%d", issueNumber),
		"--repo", repository,
		"--add-assignee", assignee)
	return err
}

// --- CommandRunner implementations ---

// NewRealCommandRunner returns a CommandRunner that executes real system commands.
// Used by the cmd/convocate-agent-wrapper entrypoint. Separated from the wrapper
// package's testable logic to keep coverage clean.
func NewRealCommandRunner() CommandRunner {
	return &realCommandRunner{}
}

type realCommandRunner struct{}

func (r *realCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (r *realCommandRunner) RunWithEnv(ctx context.Context, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (r *realCommandRunner) RunWithStdin(ctx context.Context, stdin, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// appendToFile appends data to a file, creating it if necessary.
func appendToFile(path, data string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(data)
	return err
}

// ReadPrompts reads prompts from the given reader line by line and sends
// them to the handler. This is how the Dispatch Service delivers prompts.
func (w *Wrapper) ReadPrompts(ctx context.Context, reader io.Reader, handler func(prompt string)) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			line := scanner.Text()
			if line != "" {
				handler(line)
			}
		}
	}
}

// SendHeartbeat sends a liveness heartbeat to the local Dispatch Service.
func (w *Wrapper) SendHeartbeat(writer io.Writer) error {
	_, err := fmt.Fprintln(writer, "heartbeat")
	return err
}

// StartHeartbeatLoop sends a heartbeat every 30 seconds.
func (w *Wrapper) StartHeartbeatLoop(ctx context.Context, writer io.Writer) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = w.SendHeartbeat(writer)
		case <-ctx.Done():
			return
		}
	}
}
