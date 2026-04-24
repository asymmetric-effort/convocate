package hostinstall

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

// UpdateOptions configures `claude-host update`.
type UpdateOptions struct {
	// ShellBinaryPath is the local path to the new claude-shell binary.
	// Empty = auto-discover (sibling of claude-host, then ./build/claude-shell).
	ShellBinaryPath string

	// AgentBinaryPath is the local path to the new claude-agent binary.
	// Empty = auto-discover (sibling of claude-host, then ./build/claude-agent).
	AgentBinaryPath string
}

// updateTarget describes one of the binaries we know how to update on a
// remote host. Omitting a target (by setting its LocalBinaryResolver to nil
// or the remote path to "") means "we don't manage this here".
type updateTarget struct {
	Name              string
	RemotePath        string // /usr/local/bin/claude-...
	LocalBinaryOverride string
	ServiceName       string // systemd unit to restart after install
	BinaryResolver    func(override string) (string, error)
}

// Update detects which claude-* binaries are installed on r, uploads fresh
// copies from the local filesystem, and re-runs each `<binary> install` so
// systemd units + entrypoint scripts refresh to match the new release.
// Services are restarted at the end — a staged update leaves the host
// serving the old binary until this point.
func Update(ctx context.Context, r Runner, sshCfg *SSHConfig, opts UpdateOptions, log io.Writer) error {
	_ = sshCfg
	if log == nil {
		log = io.Discard
	}
	fmt.Fprintf(log, "[claude-host] target: %s\n", r.Target())

	targets := []updateTarget{
		{
			Name:                "claude-shell",
			RemotePath:          "/usr/local/bin/claude-shell",
			LocalBinaryOverride: opts.ShellBinaryPath,
			ServiceName:         "claude-shell-status.service",
			BinaryResolver:      resolveBinaryPath,
		},
		{
			Name:                "claude-agent",
			RemotePath:          "/usr/local/bin/claude-agent",
			LocalBinaryOverride: opts.AgentBinaryPath,
			ServiceName:         "claude-agent.service",
			BinaryResolver:      resolveAgentBinaryPath,
		},
	}

	anyUpdated := false
	for _, t := range targets {
		installed, err := remoteFileExists(ctx, r, t.RemotePath, log)
		if err != nil {
			return fmt.Errorf("probe %s: %w", t.Name, err)
		}
		if !installed {
			fmt.Fprintf(log, "[claude-host] %s not present at %s — skipping\n", t.Name, t.RemotePath)
			continue
		}
		local, err := t.BinaryResolver(t.LocalBinaryOverride)
		if err != nil {
			return fmt.Errorf("locate %s binary: %w", t.Name, err)
		}
		fmt.Fprintf(log, "[claude-host] updating %s from %s\n", t.Name, local)
		if err := updateOne(ctx, r, log, t, local); err != nil {
			return err
		}
		anyUpdated = true
	}
	if !anyUpdated {
		return fmt.Errorf("no claude-* binaries found on %s; run init-shell or init-agent first", r.Target())
	}

	fmt.Fprintln(log, "")
	fmt.Fprintln(log, "[claude-host] update complete.")
	return nil
}

// updateOne does the upload + install + restart dance for a single target.
// Stopping early on error is intentional: a failed upload shouldn't leave
// us running the new install step against the old binary.
func updateOne(ctx context.Context, r Runner, log io.Writer, t updateTarget, localBinary string) error {
	steps := []step{
		{fmt.Sprintf("Upload %s binary", t.Name), func(ctx context.Context, r Runner, log io.Writer) error {
			return r.CopyFile(ctx, localBinary, t.RemotePath, 0755)
		}},
		{fmt.Sprintf("Re-run %s install", t.Name), func(ctx context.Context, r Runner, log io.Writer) error {
			return r.Run(ctx, fmt.Sprintf("%s install", t.RemotePath),
				RunOptions{Sudo: true, Stdout: log, Stderr: log})
		}},
		{fmt.Sprintf("Restart %s", t.ServiceName), func(ctx context.Context, r Runner, log io.Writer) error {
			// The install step usually restarts already, but doing it again
			// is cheap and guarantees the new binary is live even if a
			// future install decides to skip the restart.
			return r.Run(ctx,
				fmt.Sprintf("systemctl restart %s || true", t.ServiceName),
				RunOptions{Sudo: true, Stdout: log, Stderr: log})
		}},
	}
	for _, s := range steps {
		if err := runStep(ctx, r, log, s); err != nil {
			return err
		}
	}
	return nil
}

// remoteFileExists reports whether path exists on r. Implemented via
// `test -f && echo YES || echo NO` so the Runner's "non-zero = error"
// contract doesn't trip on a routine "not installed" answer.
func remoteFileExists(ctx context.Context, r Runner, path string, log io.Writer) (bool, error) {
	var buf bytes.Buffer
	cmd := fmt.Sprintf(`test -f %s && echo YES || echo NO`, shellQuoteArg(path))
	if err := r.Run(ctx, cmd, RunOptions{Stdout: &buf, Stderr: log}); err != nil {
		return false, err
	}
	return strings.Contains(buf.String(), "YES"), nil
}
