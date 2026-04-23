package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
)

// systemdUnit is the content installed at defaultSystemdUnit. Kept inline
// rather than as an embed asset because it's small and the substitution
// surface is zero.
const systemdUnit = `[Unit]
Description=claude-agent SSH API service
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
User=claude
Group=claude
ExecStart=/usr/local/bin/claude-agent serve
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`

// cmdInstall prepares the host to run claude-agent as a systemd service. It
// is idempotent — repeated invocations only update what's out of date.
//
// Requires root (EUID 0) because it writes to /etc/systemd, /etc/claude-agent,
// and fixes ownership on /home/claude directories.
func cmdInstall(_ []string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("%s install must be run as root (use sudo)", appName)
	}

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Ensure claude user", ensureClaudeUser},
		{"Create /etc/claude-agent directory", ensureEtcDir},
		{"Generate / assign agent ID", ensureAgentID},
		{"Ensure /home/claude/.ssh directory", ensureSSHDir},
		{"Ensure authorized_keys file", ensureAuthKeys},
		{"Install systemd unit", writeSystemdUnit},
		{"Reload systemd + enable claude-agent", enableService},
	}

	for _, s := range steps {
		fmt.Printf("[%s] %s...\n", appName, s.name)
		if err := s.fn(); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
		fmt.Printf("[%s] %s... done\n", appName, s.name)
	}
	fmt.Printf("\n[%s] install complete.\n", appName)
	fmt.Printf("[%s] host key: %s\n", appName, defaultHostKeyPath)
	fmt.Printf("[%s] agent-id: %s\n", appName, defaultAgentIDPath)
	fmt.Printf("[%s] authorized keys: %s (empty until init-agent populates)\n", appName, defaultAuthKeysPath)
	return nil
}

func ensureClaudeUser() error {
	if _, err := user.Lookup(defaultClaudeUsername); err == nil {
		// Already exists; make sure docker group membership is in place for
		// the container-lifecycle ops we'll run later.
		cmd := exec.Command("usermod", "-aG", "docker", defaultClaudeUsername)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		// ignore failure when docker group doesn't exist yet — docker may be
		// installed later by claude-host install.
		_ = cmd.Run()
		return nil
	}
	cmd := exec.Command("useradd", "-u", "1337", "-m", "-s", "/bin/bash", defaultClaudeUsername)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("useradd claude: %w", err)
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
	if err := os.WriteFile(defaultAuthKeysPath, []byte("# claude-agent authorized keys. Populated by 'claude-host init-agent'.\n"), 0600); err != nil {
		return err
	}
	return chownClaude(defaultAuthKeysPath)
}

func writeSystemdUnit() error {
	return os.WriteFile(defaultSystemdUnit, []byte(systemdUnit), 0644)
}

func enableService() error {
	for _, args := range [][]string{
		{"daemon-reload"},
		{"enable", "claude-agent.service"},
		{"restart", "claude-agent.service"},
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
	u, err := user.Lookup(defaultClaudeUsername)
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
