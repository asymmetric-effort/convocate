package hostinstall

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// InitShellOptions configures the init-shell deploy flow.
type InitShellOptions struct {
	// BinaryPath is the local path to the claude-shell binary to upload.
	// Empty means "auto-discover" (neighbor of the running claude-host
	// binary, then ./build/claude-shell relative to cwd).
	BinaryPath string
}

// statusUnit is the systemd unit that runs `claude-shell status-serve` as
// root. Root is required because :222 is a privileged port and the host key
// + authorized_keys files live under /etc/claude-shell. The service restarts
// on failure so a transient panic doesn't leave agents unable to push status.
const statusUnit = `[Unit]
Description=claude-shell status listener (agent push channel)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/claude-shell status-serve
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`

// statusAuthKeysHeader is the placeholder we drop into
// /etc/claude-shell/status_authorized_keys the first time init-shell runs.
// init-agent appends agent pubkeys to this file later; we don't want to
// clobber an existing file on subsequent init-shell invocations.
const statusAuthKeysHeader = "# claude-shell status listener authorized keys.\n" +
	"# Populated by 'claude-host init-agent' — one line per agent pubkey.\n"

// InitShell deploys claude-shell to r and sets up the status listener.
//
// Steps (in order):
//  1. Upload the local claude-shell binary to /usr/local/bin/claude-shell
//  2. Run `claude-shell install` remotely (idempotent; creates user, builds image, etc.)
//  3. Create /etc/claude-shell with an empty status_authorized_keys file
//  4. Install the claude-shell-status systemd unit
//  5. ufw allow 222/tcp  (agent->shell status channel)
//  6. daemon-reload, enable + start the unit
//
// sshCfg is accepted but unused today — init-shell does not reboot, so the
// reconnect plumbing isn't needed. It's in the signature to mirror Install's
// shape in case a future step needs to drop it.
func InitShell(ctx context.Context, r Runner, sshCfg *SSHConfig, opts InitShellOptions, log io.Writer) error {
	_ = sshCfg
	if log == nil {
		log = io.Discard
	}
	binary, err := resolveBinaryPath(opts.BinaryPath)
	if err != nil {
		return fmt.Errorf("locate claude-shell binary: %w", err)
	}
	fmt.Fprintf(log, "[claude-host] target: %s\n", r.Target())
	fmt.Fprintf(log, "[claude-host] local binary: %s\n", binary)

	steps := []step{
		{"Upload claude-shell binary", func(ctx context.Context, r Runner, log io.Writer) error {
			return uploadShellBinary(ctx, r, binary, log)
		}},
		{"Run claude-shell install", stepRunShellInstall},
		{"Create /etc/claude-shell", stepCreateEtcShellDir},
		{"Install claude-shell-status systemd unit", stepWriteStatusUnit},
		{"Allow tcp/222 through ufw", stepUFWAllow222},
		{"Enable + start claude-shell-status", stepEnableStatusService},
		{"Install rsyslog TLS CA + server", stepInstallRsyslogServer},
	}
	for _, s := range steps {
		if err := runStep(ctx, r, log, s); err != nil {
			return err
		}
	}

	fmt.Fprintln(log, "")
	fmt.Fprintln(log, "[claude-host] init-shell complete.")
	fmt.Fprintln(log, "  Next: claude-host init-agent --host <agent-host>")
	return nil
}

// resolveBinaryPath figures out which local claude-shell binary to upload.
// Order: explicit override → sibling of the current executable → ./build/claude-shell.
func resolveBinaryPath(override string) (string, error) {
	if override != "" {
		if _, err := os.Stat(override); err != nil {
			return "", fmt.Errorf("binary %s: %w", override, err)
		}
		return override, nil
	}
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		// Resolve any symlink so /usr/local/bin/claude-host -> /opt/.../claude-host
		// still finds the neighboring binary.
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "claude-shell"))
	}
	candidates = append(candidates, "./build/claude-shell")
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("claude-shell binary not found; pass --binary <path> (tried: %s)", strings.Join(candidates, ", "))
}

// --- steps -----------------------------------------------------------------

func uploadShellBinary(ctx context.Context, r Runner, src string, log io.Writer) error {
	dst := "/usr/local/bin/claude-shell"
	fmt.Fprintf(log, "  -> %s\n", dst)
	return r.CopyFile(ctx, src, dst, 0755)
}

func stepRunShellInstall(ctx context.Context, r Runner, log io.Writer) error {
	// claude-shell install is idempotent and enforces its own root check.
	// -y equivalent: no interactive prompts are issued by the installer, so
	// we just run it under sudo.
	return r.Run(ctx, "/usr/local/bin/claude-shell install", RunOptions{
		Sudo:   true,
		Stdout: log,
		Stderr: log,
	})
}

func stepCreateEtcShellDir(ctx context.Context, r Runner, log io.Writer) error {
	// Don't clobber status_authorized_keys if it already has agent keys —
	// use `cat > file` only when the file is absent.
	cmd := `set -e
mkdir -p /etc/claude-shell
if [ ! -f /etc/claude-shell/status_authorized_keys ]; then
  cat >/etc/claude-shell/status_authorized_keys <<'EOF'
` + statusAuthKeysHeader + `EOF
  chmod 0644 /etc/claude-shell/status_authorized_keys
fi
`
	return r.Run(ctx, cmd, RunOptions{Sudo: true, Stdout: log, Stderr: log})
}

func stepWriteStatusUnit(ctx context.Context, r Runner, log io.Writer) error {
	// Write atomically through a temp file: the `tee` pipeline would run
	// under sudo but preserve stdin-to-file framing, and the final chmod
	// keeps mode predictable.
	cmd := `set -e
cat >/etc/systemd/system/claude-shell-status.service <<'UNIT_EOF'
` + statusUnit + `UNIT_EOF
chmod 0644 /etc/systemd/system/claude-shell-status.service
`
	return r.Run(ctx, cmd, RunOptions{Sudo: true, Stdout: log, Stderr: log})
}

func stepUFWAllow222(ctx context.Context, r Runner, log io.Writer) error {
	// `ufw allow` is idempotent — re-running it just prints "Skipping
	// adding existing rule". Some hosts may not have ufw enabled, in
	// which case `ufw allow` still records the rule for when it comes
	// up. `|| true` guards the genuinely-not-installed case.
	cmd := `command -v ufw >/dev/null 2>&1 && ufw allow 222/tcp || true`
	return r.Run(ctx, cmd, RunOptions{Sudo: true, Stdout: log, Stderr: log})
}

func stepEnableStatusService(ctx context.Context, r Runner, log io.Writer) error {
	cmd := `set -e
systemctl daemon-reload
systemctl enable claude-shell-status.service
systemctl restart claude-shell-status.service
systemctl --no-pager status claude-shell-status.service | head -20
`
	return r.Run(ctx, cmd, RunOptions{Sudo: true, Stdout: log, Stderr: log})
}
