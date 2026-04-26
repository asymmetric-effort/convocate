package hostinstall

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// syncWriter serializes Write calls against a wrapped writer so multiple
// exec.Cmd instances can share the same log destination safely. Needed
// because exec.Cmd spawns an internal goroutine per non-*os.File stdio
// and those goroutines Write concurrently to whatever we hand them.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// MigrateSessionOptions drives `convocate-host migrate-session`.
type MigrateSessionOptions struct {
	// AgentID identifies the target agent — must be present as a
	// subdirectory of AgentKeysDir with shell_to_agent_ed25519_key and
	// agent-host inside (the layout init-agent writes).
	AgentID string

	// SessionUUID is the directory name under ShellSessionsBase that
	// holds session.json.
	SessionUUID string

	// ShellSessionsBase defaults to /home/claude when empty. Orphan
	// session dirs live directly under this path.
	ShellSessionsBase string

	// AgentKeysDir defaults to /etc/convocate/agent-keys when empty.
	// Used to locate the shell→agent key + agent's hostname.
	AgentKeysDir string

	// DeleteSource removes the local session directory after a
	// successful transfer. Off by default — the operator is expected
	// to run Delete via the TUI once they've verified the session is
	// live on the agent.
	DeleteSource bool
}

// MigrateSession ships a local orphan session directory to a registered
// agent by piping a tar through SSH. Container state is not migrated
// — only on-disk session metadata + home contents. The agent's docker
// daemon will docker run a fresh container on next attach/restart using
// the same UUID; new container, same files.
//
// Preflight: a convocate-session-<uuid> container still running on the
// shell host will have live writes into the source dir. The function
// does NOT stop it for you — the shell no longer manages containers
// post-v2, so the operator is expected to `docker stop` the leftover
// container before migrating. If the container is still running we
// refuse to proceed.
func MigrateSession(ctx context.Context, opts MigrateSessionOptions, log io.Writer) error {
	if log == nil {
		log = io.Discard
	}
	if opts.AgentID == "" {
		return fmt.Errorf("migrate-session: --agent is required")
	}
	if opts.SessionUUID == "" {
		return fmt.Errorf("migrate-session: --session is required")
	}
	if opts.ShellSessionsBase == "" {
		opts.ShellSessionsBase = "/home/claude"
	}
	if opts.AgentKeysDir == "" {
		opts.AgentKeysDir = "/etc/convocate/agent-keys"
	}

	srcDir := filepath.Join(opts.ShellSessionsBase, opts.SessionUUID)
	if _, err := os.Stat(filepath.Join(srcDir, "session.json")); err != nil {
		return fmt.Errorf("no session.json at %s: %w", srcDir, err)
	}

	// Preflight: refuse if a stray container is still running for this
	// UUID. Silently-OK if docker isn't installed locally (common for a
	// pure v2 shell host where F removed that requirement).
	if err := refuseIfContainerRunning(ctx, opts.SessionUUID); err != nil {
		return err
	}

	agentDir := filepath.Join(opts.AgentKeysDir, opts.AgentID)
	keyPath := filepath.Join(agentDir, "shell_to_agent_ed25519_key")
	if _, err := os.Stat(keyPath); err != nil {
		return fmt.Errorf("agent %q not registered (missing %s): %w",
			opts.AgentID, keyPath, err)
	}
	hostBytes, err := os.ReadFile(filepath.Join(agentDir, "agent-host"))
	if err != nil {
		return fmt.Errorf("read agent-host: %w", err)
	}
	host := strings.TrimSpace(string(hostBytes))
	if i := strings.Index(host, "@"); i >= 0 {
		host = host[i+1:]
	}
	if host == "" {
		return fmt.Errorf("agent-host file is empty")
	}

	fmt.Fprintf(log, "[migrate] %s → %s (%s)\n", opts.SessionUUID, opts.AgentID, host)

	// tar cf - -C <base> <uuid> | ssh -i <key> claude@<host>
	//     'mkdir -p /home/claude && tar xf - -C /home/claude'
	//
	// tar from <base> with <uuid> as the archive entry so the unpacked
	// tree lands at /home/claude/<uuid>/. Owner/mode are preserved by
	// tar's defaults, which matches the invariant that files are
	// claude-owned on both ends (same uid).
	//
	// We wire an explicit io.Pipe between the two commands rather than
	// exec.StdoutPipe — StdoutPipe has Wait()-ordering rules that the
	// race detector flags when a second Cmd consumes the reader.
	tarCmd := exec.CommandContext(ctx, "tar",
		"cf", "-",
		"-C", opts.ShellSessionsBase,
		opts.SessionUUID,
	)
	sshCmd := exec.CommandContext(ctx, "ssh",
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "BatchMode=yes",
		"claude@"+host,
		"mkdir -p /home/claude && tar xf - -C /home/claude",
	)
	// os.Pipe (not io.Pipe) so exec can dup2 these fds directly into
	// the child processes — no goroutines copying bytes through a
	// user-space pipe, no race-detector warnings about concurrent
	// accesses to the pipe ends from exec's writerDescriptor and
	// readerDescriptor paths.
	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("os.Pipe: %w", err)
	}
	// Serialize log writes — exec.Cmd's own goroutines will write
	// concurrently through the stderr/stdout wrappers.
	shared := &syncWriter{w: log}
	tarCmd.Stdout = pw
	tarCmd.Stderr = shared
	sshCmd.Stdin = pr
	sshCmd.Stdout = shared
	sshCmd.Stderr = shared

	if err := sshCmd.Start(); err != nil {
		_ = pw.Close()
		_ = pr.Close()
		return fmt.Errorf("start ssh: %w", err)
	}
	// Parent no longer needs the reader end — ssh dup2'd it already.
	_ = pr.Close()

	tarErr := tarCmd.Run()
	// Closing the write end delivers EOF to ssh's stdin so its child
	// tar can finish unpacking and exit.
	_ = pw.Close()

	sshErr := sshCmd.Wait()
	if tarErr != nil {
		return fmt.Errorf("tar cf: %w", tarErr)
	}
	if sshErr != nil {
		return fmt.Errorf("remote tar/ssh: %w", sshErr)
	}

	fmt.Fprintln(log, "[migrate] transfer complete.")

	if opts.DeleteSource {
		if err := os.RemoveAll(srcDir); err != nil {
			return fmt.Errorf("remove local source: %w", err)
		}
		fmt.Fprintf(log, "[migrate] removed local %s\n", srcDir)
	}
	return nil
}

// refuseIfContainerRunning returns an error if docker is available on
// the local host AND a container named convocate-session-<uuid> is running.
// We can't safely tar the home dir while the container is writing to it.
func refuseIfContainerRunning(ctx context.Context, uuid string) error {
	if _, err := exec.LookPath("docker"); err != nil {
		// No docker → no stale container to worry about on a post-v2 shell.
		return nil
	}
	name := "convocate-session-" + uuid
	out, err := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{.State.Running}}",
		name,
	).Output()
	if err != nil {
		// Most likely "No such object" — container doesn't exist, fine.
		return nil
	}
	if strings.TrimSpace(string(out)) == "true" {
		return fmt.Errorf("container %s is still running locally — `docker stop %s` before migrating", name, name)
	}
	return nil
}
