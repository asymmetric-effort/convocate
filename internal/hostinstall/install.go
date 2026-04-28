package hostinstall

import (
	"context"
	"fmt"
	"io"
)

// Install runs the full convocate-host install workflow against r. Over SSH, it
// reboots the target after the apt upgrade and transparently swaps to a
// fresh SSH connection for the remaining steps; in local mode, the reboot
// is skipped with a warning because rebooting the machine running us would
// abort the install.
//
// For SSH targets, sshCfg must describe the same connection as r so the
// reconnect after reboot reuses the same auth and address. Pass nil for
// local targets.
func Install(ctx context.Context, r Runner, sshCfg *SSHConfig, log io.Writer) error {
	if log == nil {
		log = io.Discard
	}
	phase1 := []step{
		{"Check platform", stepCheckPlatform},
		{"apt update + dist-upgrade", stepAptUpgrade},
	}
	phase2 := []step{
		{"Install apt base packages", stepAptBasePackages},
		{"Install Docker", stepInstallDocker},
		{"Install dnsmasq", stepInstallDnsmasq},
		{"Disable systemd-resolved stub listener", stepDisableResolvedStub},
		{"Create claude user", stepCreateClaudeUser},
		{"Enable ufw (default deny-incoming)", stepEnableUFW},
		{"Set timezone UTC", stepSetTimezoneUTC},
		{"Ensure unattended-upgrades enabled", stepUnattendedUpgrades},
	}

	fmt.Fprintf(log, "[convocate-host] target: %s\n", r.Target())
	for _, s := range phase1 {
		if err := runStep(ctx, r, log, s); err != nil {
			return err
		}
	}

	r, err := rebootIfRemote(ctx, r, sshCfg, log)
	if err != nil {
		return err
	}

	for _, s := range phase2 {
		if err := runStep(ctx, r, log, s); err != nil {
			return err
		}
	}

	fmt.Fprintln(log, "")
	fmt.Fprintln(log, "[convocate-host] host is provisioned. Next:")
	fmt.Fprintln(log, "  convocate-host init-shell --host <this-host>   # deploy convocate + rsyslog CA")
	fmt.Fprintln(log, "  convocate-host init-agent --host <agent-host>  # deploy convocate-agent on another host")
	return nil
}

// step pairs a human-readable name with a function that executes against a
// runner. The orchestrator logs the name before calling fn so failures have
// context even when the command's own output is terse.
type step struct {
	name string
	fn   func(ctx context.Context, r Runner, log io.Writer) error
}

func runStep(ctx context.Context, r Runner, log io.Writer, s step) error {
	fmt.Fprintf(log, "\n[convocate-host] %s...\n", s.name)
	if err := s.fn(ctx, r, log); err != nil {
		return fmt.Errorf("%s: %w", s.name, err)
	}
	fmt.Fprintf(log, "[convocate-host] %s... done\n", s.name)
	return nil
}

// rebootIfRemote reboots the target and returns a fresh runner. In local
// mode it skips (with a clear warning) because rebooting would kill this
// process; the operator is expected to reboot and re-run after phase 1
// succeeds locally.
func rebootIfRemote(ctx context.Context, r Runner, sshCfg *SSHConfig, log io.Writer) (Runner, error) {
	ssh, ok := r.(*SSHRunner)
	if !ok || sshCfg == nil {
		fmt.Fprintln(log, "")
		fmt.Fprintln(log, "[convocate-host] local mode: skipping automatic reboot.")
		fmt.Fprintln(log, "[convocate-host] If the kernel was upgraded, reboot manually and re-run 'convocate-host install'.")
		return r, nil
	}
	newSSH, err := RebootAndReconnect(ctx, ssh, *sshCfg, RebootOptions{Progress: log})
	if err != nil {
		return nil, fmt.Errorf("reboot/reconnect: %w", err)
	}
	return newSSH, nil
}

// --- individual steps ------------------------------------------------------

func stepCheckPlatform(ctx context.Context, r Runner, log io.Writer) error {
	// Confirm Ubuntu (we're not strict about the minor version yet — the
	// core packages we install are available across 22.04 and 24.04).
	return r.Run(ctx,
		`. /etc/os-release && if [ "$ID" != "ubuntu" ]; then echo "only ubuntu is supported (found: $ID)" >&2; exit 1; fi; echo "ok: $PRETTY_NAME ($(uname -m))"`,
		RunOptions{Stdout: log, Stderr: log})
}

func stepAptUpgrade(ctx context.Context, r Runner, log io.Writer) error {
	// -y for non-interactive; -o Dpkg::Options to silently keep any already-
	// present conffiles (sidesteps interactive "do you want to keep or
	// replace" prompts during dist-upgrade).
	return r.Run(ctx,
		`DEBIAN_FRONTEND=noninteractive apt-get update -y && `+
			`DEBIAN_FRONTEND=noninteractive apt-get -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold -y dist-upgrade`,
		RunOptions{Sudo: true, Stdout: log, Stderr: log})
}

func stepAptBasePackages(ctx context.Context, r Runner, log io.Writer) error {
	pkgs := "ca-certificates gnupg curl make git tmux build-essential ufw"
	return r.Run(ctx,
		"DEBIAN_FRONTEND=noninteractive apt-get install -y "+pkgs,
		RunOptions{Sudo: true, Stdout: log, Stderr: log})
}

func stepInstallDocker(ctx context.Context, r Runner, log io.Writer) error {
	// Using Ubuntu's packaged docker.io (per TO-DO.md §5) rather than docker-ce,
	// so we don't have to manage an extra apt source. Also pull docker-compose-plugin
	// when available so `docker compose` works.
	return r.Run(ctx,
		`DEBIAN_FRONTEND=noninteractive apt-get install -y docker.io; `+
			`systemctl enable --now docker`,
		RunOptions{Sudo: true, Stdout: log, Stderr: log})
}

func stepInstallDnsmasq(ctx context.Context, r Runner, log io.Writer) error {
	return r.Run(ctx,
		"DEBIAN_FRONTEND=noninteractive apt-get install -y dnsmasq",
		RunOptions{Sudo: true, Stdout: log, Stderr: log})
}

func stepDisableResolvedStub(ctx context.Context, r Runner, log io.Writer) error {
	// Frees port 53 for dnsmasq. Uses a drop-in so we don't rewrite the stock
	// /etc/systemd/resolved.conf. Restarting the service applies the change
	// without needing a reboot.
	cmd := `set -e
mkdir -p /etc/systemd/resolved.conf.d
cat >/etc/systemd/resolved.conf.d/00-convocate-dnsmasq.conf <<'EOF'
# Managed by convocate-host. Frees port 53 for dnsmasq.
[Resolve]
DNSStubListener=no
EOF
systemctl restart systemd-resolved || true
`
	return r.Run(ctx, cmd, RunOptions{Sudo: true, Stdout: log, Stderr: log})
}

func stepCreateClaudeUser(ctx context.Context, r Runner, log io.Writer) error {
	// Idempotent: if the user already exists we just ensure docker group
	// membership. uid 1337 matches what convocate install expects (see
	// internal/config.ConvocateUser and the skel ownership in session dirs).
	cmd := `set -e
if ! id convocate >/dev/null 2>&1; then
  useradd -u 1337 -m -s /bin/bash convocate
fi
usermod -aG docker convocate
`
	return r.Run(ctx, cmd, RunOptions{Sudo: true, Stdout: log, Stderr: log})
}

func stepEnableUFW(ctx context.Context, r Runner, log io.Writer) error {
	// Default deny-incoming / allow-outgoing with no explicit allow rules —
	// init-shell / init-agent will open the specific ports they need (22,
	// 222, 514). --force skips the confirmation prompt when enabling.
	cmd := `set -e
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw --force enable
ufw status verbose
`
	return r.Run(ctx, cmd, RunOptions{Sudo: true, Stdout: log, Stderr: log})
}

func stepSetTimezoneUTC(ctx context.Context, r Runner, log io.Writer) error {
	return r.Run(ctx, "timedatectl set-timezone Etc/UTC", RunOptions{Sudo: true, Stdout: log, Stderr: log})
}

func stepUnattendedUpgrades(ctx context.Context, r Runner, log io.Writer) error {
	// The package is usually already installed on Ubuntu Server; install in
	// case it was removed. The drop-in writes unattended-upgrades defaults.
	cmd := `DEBIAN_FRONTEND=noninteractive apt-get install -y unattended-upgrades && ` +
		`systemctl enable --now unattended-upgrades.service`
	return r.Run(ctx, cmd, RunOptions{Sudo: true, Stdout: log, Stderr: log})
}
