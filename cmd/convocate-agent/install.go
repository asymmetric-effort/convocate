package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/asymmetric-effort/convocate/internal/config"
	"github.com/asymmetric-effort/convocate/internal/skel"
)

// systemdUnit is the content installed at defaultSystemdUnit. Kept inline
// rather than as an embed asset because it's small and the substitution
// surface is zero.
const systemdUnit = `[Unit]
Description=convocate-agent SSH API service
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
User=convocate
Group=convocate
ExecStart=/usr/local/bin/convocate-agent serve
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`

// cmdInstall prepares the host to run convocate-agent as a systemd service. It
// is idempotent — repeated invocations only update what's out of date.
//
// Requires root (EUID 0) because it writes to /etc/systemd, /etc/convocate-agent,
// and fixes ownership on /home/convocate directories.
func cmdInstall(_ []string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("%s install must be run as root (use sudo)", appName)
	}

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Ensure claude user", ensureClaudeUser},
		{"Create /etc/convocate-agent directory", ensureEtcDir},
		{"Generate / assign agent ID", ensureAgentID},
		{"Ensure /home/convocate/.ssh directory", ensureSSHDir},
		{"Ensure authorized_keys file", ensureAuthKeys},
		{"Set up session skeleton directory", ensureSessionSkel},
		{"Check claude CLI is installed", checkClaudeCLIPresent},
		{"Install convocate-sessions.slice (90% cgroup cap)", writeSessionsSlice},
		{"Install daily image-prune cron", writeImagePruneCron},
		{"Install systemd unit", writeSystemdUnit},
		{"Reload systemd + enable convocate-agent", enableService},
	}

	for _, s := range steps {
		fmt.Printf("[%s] %s...\n", appName, s.name)
		if err := s.fn(); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
		fmt.Printf("[%s] %s... done\n", appName, s.name)
	}

	// If convocate was previously installed on this host, its session
	// directory is co-owned with us (same uid) — our serve process will
	// pick up any existing session.json files automatically. Surface the
	// count so operators know what's been adopted without having to go
	// find the directory themselves.
	if n, err := countAdoptedSessions(); err == nil && n > 0 {
		fmt.Printf("\n[%s] adopted %d pre-existing session(s) from this host\n", appName, n)
		fmt.Printf("[%s] any containers still running under docker continue to run and will be\n", appName)
		fmt.Printf("[%s] reported by 'convocate-agent list' once the service is up.\n", appName)
	}

	fmt.Printf("\n[%s] install complete.\n", appName)
	fmt.Printf("[%s] host key: %s\n", appName, defaultHostKeyPath)
	fmt.Printf("[%s] agent-id: %s\n", appName, defaultAgentIDPath)
	fmt.Printf("[%s] authorized keys: %s (empty until init-agent populates)\n", appName, defaultAuthKeysPath)
	return nil
}

// countAdoptedSessions returns the number of session.json files under the
// claude user's home dir — these are pre-existing convocate sessions
// that this agent now manages because both services run as the same uid
// and read/write the same directory layout.
//
// Session dirs live directly under /home/convocate/ with UUID names. We use
// "has a session.json inside" as the detection heuristic rather than
// parsing the directory name, since that's the file session.Manager
// itself treats as authoritative.
func countAdoptedSessions() (int, error) {
	u, err := user.Lookup(defaultConvocateUsername)
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(u.HomeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(u.HomeDir, e.Name(), "session.json")); err == nil {
			count++
		}
	}
	return count, nil
}

func ensureClaudeUser() error {
	if _, err := user.Lookup(defaultConvocateUsername); err == nil {
		// Already exists; make sure docker group membership is in place for
		// the container-lifecycle ops we'll run later.
		cmd := exec.Command("usermod", "-aG", "docker", defaultConvocateUsername)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		// ignore failure when docker group doesn't exist yet — docker may be
		// installed later by convocate-host install.
		_ = cmd.Run()
		return nil
	}
	cmd := exec.Command("useradd", "-u", "1337", "-m", "-s", "/bin/bash", defaultConvocateUsername)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("useradd -u 1337 -m -s /bin/bash convocate: %w", err)
	}
	return nil
}

func ensureEtcDir() error {
	return os.MkdirAll(defaultEtcDir, 0755)
}

func ensureAgentID() error {
	_, err := loadOrCreateAgentID(defaultAgentIDPath)
	return err
}

func ensureSSHDir() error {
	if err := os.MkdirAll(defaultAuthKeysDir, 0700); err != nil {
		return err
	}
	return chownClaude(defaultAuthKeysDir)
}

func ensureAuthKeys() error {
	if _, err := os.Stat(defaultAuthKeysPath); err == nil {
		return chownClaude(defaultAuthKeysPath)
	}
	if err := os.WriteFile(defaultAuthKeysPath, []byte("# convocate-agent authorized keys. Populated by 'convocate-host init-agent'.\n"), 0600); err != nil {
		return err
	}
	return chownClaude(defaultAuthKeysPath)
}

func writeSystemdUnit() error {
	return os.WriteFile(defaultSystemdUnit, []byte(systemdUnit), 0644)
}

// ensureSessionSkel provisions /home/convocate/.skel/ with the embedded
// starter files (CLAUDE.md etc.) that session.Manager.CreateWithOptions
// copies into every new session dir. Pre-v2 this was a convocate
// install step; now that sessions only spawn on agents, the agent owns
// it. Idempotent: skel.Setup only writes missing files.
func ensureSessionSkel() error {
	u, err := user.Lookup(defaultConvocateUsername)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", defaultConvocateUsername, err)
	}
	skelPath := filepath.Join(u.HomeDir, config.SkelDir)
	if err := skel.Setup(skelPath); err != nil {
		return err
	}
	return chownClaude(skelPath)
}

// checkClaudeCLIPresent fails the install if /usr/local/bin/claude is
// missing. Containers mount that binary read-only into each session
// (via container.Runner.buildRunArgs), so its absence is a genuine
// blocker for the agent — better to catch it at install than at
// docker-run time with an unhelpful mount error.
func checkClaudeCLIPresent() error {
	if _, err := os.Stat(config.ClaudeBinaryPath); os.IsNotExist(err) {
		return fmt.Errorf("claude CLI not found at %s; install it before provisioning sessions", config.ClaudeBinaryPath)
	}
	fmt.Printf("[%s]   found claude CLI at %s\n", appName, config.ClaudeBinaryPath)
	return nil
}

// writeSessionsSlice renders /etc/systemd/system/convocate-sessions.slice
// with CPUQuota and MemoryMax values computed from the host's own
// resource totals. Containers enroll under this slice via docker run
// --cgroup-parent so the kernel caps their aggregate usage at ~90% of
// the box — operator retains 10% headroom to intervene.
//
// CPUQuota is expressed as "quota%" where 100% == one full core. We
// multiply nproc by 90 to cap total CPU at 90% of available cores.
// MemoryMax is an absolute byte count — 90% of MemTotal from
// /proc/meminfo. Re-running install recomputes against current host
// specs (rare unless the host was resized).
func writeSessionsSlice() error {
	cores, err := detectHostCores()
	if err != nil {
		return fmt.Errorf("detect host cores: %w", err)
	}
	memBytes, err := detectHostMemoryBytes()
	if err != nil {
		return fmt.Errorf("detect host memory: %w", err)
	}
	cpuQuota := cores * 90
	memMax := memBytes * 90 / 100

	unit := fmt.Sprintf(`[Unit]
Description=convocate-agent session containers (aggregate 90%% cap)
Documentation=https://github.com/asymmetric-effort/convocate
Before=convocate-agent.service

[Slice]
CPUAccounting=yes
CPUQuota=%d%%
MemoryAccounting=yes
MemoryMax=%d
`, cpuQuota, memMax)

	if err := os.WriteFile(defaultSessionsSlicePath, []byte(unit), 0644); err != nil {
		return err
	}
	// daemon-reload is done once in enableService, which runs after this
	// step, so we don't duplicate the reload here.
	return nil
}

// imagePruneScript is what convocate-agent install drops at
// /etc/cron.daily/convocate-image-prune. Retention policy (per
// project decision): keep every image tag currently referenced by any
// container (running OR stopped) plus whatever /etc/convocate-agent/
// current-image points at. Everything else tagged convocate:* is
// removed so disk use doesn't creep up over many releases.
//
// docker rmi errors are tolerated (|| true) because a concurrent
// container start could grab an image between our decision to prune
// and the rmi call.
const imagePruneScript = `#!/bin/sh
# Managed by convocate-agent install. Do not edit by hand.
set -e

# Every image referenced by any container (running + stopped).
in_use=$(docker ps -a --format '{{.Image}}' | sort -u || true)

# Plus whatever is flagged as current.
if [ -f /etc/convocate-agent/current-image ]; then
    current=$(tr -d '[:space:]' </etc/convocate-agent/current-image)
else
    current=""
fi

# Enumerate all local convocate images (skip dangling <none>).
all=$(docker images convocate --format '{{.Repository}}:{{.Tag}}' 2>/dev/null | grep -v '<none>' || true)

for img in $all; do
    if echo "$in_use" | grep -qF -x "$img"; then
        continue
    fi
    if [ "$img" = "$current" ]; then
        continue
    fi
    echo "convocate-image-prune: removing $img"
    docker rmi "$img" || true
done
`

func writeImagePruneCron() error {
	// cron.daily requires the script be executable AND that the name
	// contain no dots (run-parts semantics). The chosen path
	// /etc/cron.daily/convocate-image-prune satisfies both.
	return os.WriteFile(defaultImagePruneScript, []byte(imagePruneScript), 0755)
}

// detectHostCores returns the number of CPU cores the kernel exposes.
func detectHostCores() (int, error) {
	cmd := exec.Command("nproc")
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parse nproc output %q: %w", string(out), err)
	}
	if n < 1 {
		return 0, fmt.Errorf("nproc returned %d", n)
	}
	return n, nil
}

// detectHostMemoryBytes returns total RAM in bytes from /proc/meminfo.
func detectHostMemoryBytes() (int64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		// "MemTotal:  16292456 kB"
		if len(fields) < 3 {
			return 0, fmt.Errorf("unexpected MemTotal line: %q", line)
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse MemTotal kB %q: %w", fields[1], err)
		}
		return kb * 1024, nil
	}
	return 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
}

func enableService() error {
	for _, args := range [][]string{
		{"daemon-reload"},
		{"enable", "convocate-agent.service"},
		{"restart", "convocate-agent.service"},
	} {
		cmd := exec.Command("systemctl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("systemctl %v: %w", args, err)
		}
	}
	return nil
}

func chownClaude(path string) error {
	u, err := user.Lookup(defaultConvocateUsername)
	if err != nil {
		return err
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, uid, gid)
	})
}
