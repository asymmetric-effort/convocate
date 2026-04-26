package hostinstall

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/asymmetric-effort/convocate/internal/sshutil"
)

// InitAgentOptions configures the init-agent deploy flow.
type InitAgentOptions struct {
	// BinaryPath is the local path to the convocate-agent binary. Empty =
	// auto-discover (neighbor of convocate-host, then ./build/convocate-agent).
	BinaryPath string

	// ShellHost is the address agents should use to reach the convocate
	// status listener (tcp/222). Stamped into /etc/convocate-agent/shell-host
	// on the target. Required.
	ShellHost string

	// LocalShellEtcDir is where to install the shell-side peering material:
	//   <dir>/status_authorized_keys       (agent->shell pub keys appended)
	//   <dir>/agent-keys/<id>/...          (per-agent material)
	// Empty defaults to /etc/convocate.
	LocalShellEtcDir string

	// ImageTag is the full image reference (e.g. "convocate:v2.0.0")
	// init-agent will push to the new agent and stamp as
	// /etc/convocate-agent/current-image. Required — the agent has no
	// image until init-agent runs, and we don't want to ship :latest.
	ImageTag string

	// CACertPath / CAKeyPath override where we read the rsyslog CA
	// material from. Empty falls back to
	// <LocalShellEtcDir>/rsyslog-ca/{ca.crt,ca.key}. Useful when
	// init-agent runs from a workstation instead of the shell host
	// itself — the operator copies the CA files locally once, then
	// points every init-agent run at those paths.
	CACertPath string
	CAKeyPath  string
}

// InitAgent deploys convocate-agent to r and wires up the bi-directional
// peering between agent host and convocate host.
//
// Steps:
//  1. Upload convocate-agent binary to /usr/local/bin/convocate-agent
//  2. Run `convocate-agent install` remotely (creates user, systemd unit,
//     agent-id, etc.)
//  3. Read the freshly-minted agent-id back
//  4. Generate two ed25519 keypairs (shell->agent, agent->shell)
//  5. Push the agent-side files: authorized_keys (shell->agent pub),
//     agent_to_shell private key, shell-host address
//  6. Restart convocate-agent.service so it picks up the new shell-host +
//     private key and opens the status emitter
//  7. Locally, append the agent->shell pubkey to
//     <LocalShellEtcDir>/status_authorized_keys and stash the
//     shell->agent private key under
//     <LocalShellEtcDir>/agent-keys/<id>/ so convocate can SSH to the
//     agent for CRUD ops later
func InitAgent(ctx context.Context, r Runner, sshCfg *SSHConfig, opts InitAgentOptions, log io.Writer) error {
	_ = sshCfg
	if log == nil {
		log = io.Discard
	}
	if strings.TrimSpace(opts.ShellHost) == "" {
		return fmt.Errorf("init-agent: --shell-host is required")
	}
	if strings.TrimSpace(opts.ImageTag) == "" {
		return fmt.Errorf("init-agent: --image-tag is required (agent has no image until pushed)")
	}
	if opts.LocalShellEtcDir == "" {
		opts.LocalShellEtcDir = "/etc/convocate"
	}
	binary, err := resolveAgentBinaryPath(opts.BinaryPath)
	if err != nil {
		return fmt.Errorf("locate convocate-agent binary: %w", err)
	}
	fmt.Fprintf(log, "[convocate-host] target: %s\n", r.Target())
	fmt.Fprintf(log, "[convocate-host] local binary: %s\n", binary)
	fmt.Fprintf(log, "[convocate-host] shell host: %s\n", opts.ShellHost)
	fmt.Fprintf(log, "[convocate-host] local shell etc dir: %s\n", opts.LocalShellEtcDir)

	// Upload + install.
	if err := runStep(ctx, r, log, step{"Upload convocate-agent binary", func(ctx context.Context, r Runner, log io.Writer) error {
		return r.CopyFile(ctx, binary, "/usr/local/bin/convocate-agent", 0755)
	}}); err != nil {
		return err
	}
	if err := runStep(ctx, r, log, step{"Run convocate-agent install", func(ctx context.Context, r Runner, log io.Writer) error {
		return r.Run(ctx, "/usr/local/bin/convocate-agent install", RunOptions{Sudo: true, Stdout: log, Stderr: log})
	}}); err != nil {
		return err
	}

	// Read agent-id — used to label keys and to locate per-agent files on
	// the shell side.
	agentID, err := readRemoteAgentID(ctx, r, log)
	if err != nil {
		return err
	}
	fmt.Fprintf(log, "[convocate-host] agent-id: %s\n", agentID)

	// Generate the two keypairs. Tagging comments by agent-id makes it
	// possible to audit /etc/convocate/status_authorized_keys and
	// /home/claude/.ssh/authorized_keys entries later.
	shellToAgentKP, err := sshutil.GenerateKeypair(fmt.Sprintf("shell->agent=%s", agentID))
	if err != nil {
		return fmt.Errorf("generate shell->agent keypair: %w", err)
	}
	agentToShellKP, err := sshutil.GenerateKeypair(fmt.Sprintf("agent=%s", agentID))
	if err != nil {
		return fmt.Errorf("generate agent->shell keypair: %w", err)
	}

	// Push peering files onto the agent.
	agentPushes := []step{
		{"Install shell->agent authorized_keys", func(ctx context.Context, r Runner, log io.Writer) error {
			return writeRemoteContent(ctx, r, log,
				shellToAgentKP.AuthorizedKey,
				"/home/claude/.ssh/authorized_keys", 0600, "claude:claude")
		}},
		{"Install agent->shell private key", func(ctx context.Context, r Runner, log io.Writer) error {
			return writeRemoteContent(ctx, r, log,
				agentToShellKP.PrivatePEM,
				"/etc/convocate-agent/agent_to_shell_ed25519_key", 0600, "claude:claude")
		}},
		{"Write shell-host address", func(ctx context.Context, r Runner, log io.Writer) error {
			return writeRemoteContent(ctx, r, log,
				[]byte(strings.TrimSpace(opts.ShellHost)+"\n"),
				"/etc/convocate-agent/shell-host", 0644, "root:root")
		}},
		{"Restart convocate-agent.service", func(ctx context.Context, r Runner, log io.Writer) error {
			return r.Run(ctx, "systemctl restart convocate-agent.service",
				RunOptions{Sudo: true, Stdout: log, Stderr: log})
		}},
	}
	for _, s := range agentPushes {
		if err := runStep(ctx, r, log, s); err != nil {
			return err
		}
	}

	// Local: populate the shell-side peering dir.
	if err := writeShellSideFiles(opts.LocalShellEtcDir, agentID, shellToAgentKP, agentToShellKP, r.Target(), log); err != nil {
		return fmt.Errorf("write shell-side files: %w", err)
	}

	// Configure the agent's rsyslog TLS client — issues a client cert
	// under the shell's CA, uploads cert/key, drops /etc/rsyslog.d
	// forwarder config, restarts rsyslog. Runs after the peering files
	// so SSH is already provably working.
	if err := runStep(ctx, r, log, step{"Configure rsyslog TLS client", func(ctx context.Context, r Runner, log io.Writer) error {
		return configureAgentRsyslogClient(ctx, r, opts.LocalShellEtcDir, agentID, opts.ShellHost, opts.CACertPath, opts.CAKeyPath, log)
	}}); err != nil {
		return err
	}

	// Push the container image to the agent + stamp the pointer file
	// so subsequent docker runs on the agent use the versioned tag.
	// This must happen after install so /etc/convocate-agent exists, and
	// before the agent is expected to accept any Create op.
	if err := runStep(ctx, r, log, step{"Push container image", func(ctx context.Context, r Runner, log io.Writer) error {
		return TransferImage(ctx, r, opts.ImageTag, log)
	}}); err != nil {
		return err
	}
	if err := runStep(ctx, r, log, step{"Write /etc/convocate-agent/current-image", func(ctx context.Context, r Runner, log io.Writer) error {
		return writeRemoteContent(ctx, r, log,
			[]byte(opts.ImageTag+"\n"),
			"/etc/convocate-agent/current-image", 0644, "root:root")
	}}); err != nil {
		return err
	}
	if err := runStep(ctx, r, log, step{"Restart convocate-agent for current-image", func(ctx context.Context, r Runner, log io.Writer) error {
		return r.Run(ctx, "systemctl restart convocate-agent.service",
			RunOptions{Sudo: true, Stdout: log, Stderr: log})
	}}); err != nil {
		return err
	}

	fmt.Fprintln(log, "")
	fmt.Fprintln(log, "[convocate-host] init-agent complete.")
	fmt.Fprintf(log, "  agent-id: %s\n", agentID)
	fmt.Fprintf(log, "  shell->agent private key: %s\n", shellToAgentKeyPath(opts.LocalShellEtcDir, agentID))
	fmt.Fprintf(log, "  agent->shell pubkey appended to: %s\n", filepath.Join(opts.LocalShellEtcDir, "status_authorized_keys"))
	fmt.Fprintf(log, "  agent now forwarding logs to %s:514 via TLS\n", opts.ShellHost)
	return nil
}

func resolveAgentBinaryPath(override string) (string, error) {
	if override != "" {
		if _, err := os.Stat(override); err != nil {
			return "", fmt.Errorf("binary %s: %w", override, err)
		}
		return override, nil
	}
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "convocate-agent"))
	}
	candidates = append(candidates, "./build/convocate-agent")
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("convocate-agent binary not found; pass --binary <path> (tried: %s)",
		strings.Join(candidates, ", "))
}

// readRemoteAgentID runs `cat /etc/convocate-agent/agent-id` on the target and
// returns the trimmed content. Fails if the file is missing (which would
// mean `convocate-agent install` didn't complete).
func readRemoteAgentID(ctx context.Context, r Runner, log io.Writer) (string, error) {
	var buf bytes.Buffer
	if err := r.Run(ctx, "cat /etc/convocate-agent/agent-id", RunOptions{
		Sudo:   true,
		Stdout: &buf,
		Stderr: log,
	}); err != nil {
		return "", fmt.Errorf("read agent-id: %w", err)
	}
	id := strings.TrimSpace(buf.String())
	if id == "" {
		return "", fmt.Errorf("agent-id file was empty")
	}
	return id, nil
}

// writeRemoteContent stages content in a local temp file and uploads it via
// the runner's CopyFile primitive, then (optionally) chowns it on the
// target. Keeps callers free of per-destination plumbing.
func writeRemoteContent(ctx context.Context, r Runner, log io.Writer,
	content []byte, destPath string, mode os.FileMode, chownSpec string) error {
	tmpDir, err := os.MkdirTemp("", "convocate-host-")
	if err != nil {
		return fmt.Errorf("mktemp: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	local := filepath.Join(tmpDir, filepath.Base(destPath))
	if err := os.WriteFile(local, content, mode); err != nil {
		return fmt.Errorf("stage content: %w", err)
	}
	if err := r.CopyFile(ctx, local, destPath, mode); err != nil {
		return fmt.Errorf("upload to %s: %w", destPath, err)
	}
	if chownSpec != "" {
		if err := r.Run(ctx, fmt.Sprintf("chown %s %s", chownSpec, shellQuoteArg(destPath)),
			RunOptions{Sudo: true, Stdout: log, Stderr: log}); err != nil {
			return fmt.Errorf("chown %s: %w", destPath, err)
		}
	}
	return nil
}

// writeShellSideFiles populates the shell-side peering directory with the
// agent's public half + the shell's private half. This is the only place
// init-agent touches the local filesystem; everything else runs through r.
func writeShellSideFiles(etcDir, agentID string, shellToAgent, agentToShell *sshutil.Keypair, target string, log io.Writer) error {
	fmt.Fprintf(log, "\n[convocate-host] Writing shell-side peering files to %s...\n", etcDir)
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", etcDir, err)
	}
	authPath := filepath.Join(etcDir, "status_authorized_keys")
	if err := appendFile(authPath, agentToShell.AuthorizedKey, 0644); err != nil {
		return fmt.Errorf("append %s: %w", authPath, err)
	}
	fmt.Fprintf(log, "  appended agent->shell pubkey to %s\n", authPath)

	agentDir := filepath.Join(etcDir, "agent-keys", agentID)
	if err := os.MkdirAll(agentDir, 0700); err != nil {
		return fmt.Errorf("mkdir %s: %w", agentDir, err)
	}
	keyPath := shellToAgentKeyPath(etcDir, agentID)
	if err := os.WriteFile(keyPath, shellToAgent.PrivatePEM, 0600); err != nil {
		return fmt.Errorf("write %s: %w", keyPath, err)
	}
	fmt.Fprintf(log, "  wrote shell->agent privkey to %s\n", keyPath)

	// Record the agent's network address alongside the key so convocate
	// knows how to reach it. Future dial code reads this file by agent-id.
	hostPath := filepath.Join(agentDir, "agent-host")
	if err := os.WriteFile(hostPath, []byte(target+"\n"), 0644); err != nil {
		return fmt.Errorf("write %s: %w", hostPath, err)
	}
	fmt.Fprintf(log, "  recorded agent host (%s) at %s\n", target, hostPath)
	return nil
}

func shellToAgentKeyPath(etcDir, agentID string) string {
	return filepath.Join(etcDir, "agent-keys", agentID, "shell_to_agent_ed25519_key")
}

// appendFile writes content to path, creating the file with mode if absent
// or appending otherwise. authorized_keys files accumulate entries across
// init-agent runs — we must not clobber existing entries.
func appendFile(path string, content []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(content)
	return err
}

// shellQuoteArg wraps s in single quotes so a path with spaces survives
// bash -c "chown $spec $path".
func shellQuoteArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
