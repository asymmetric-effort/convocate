package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Well-known on-disk paths for claude-agent. Tests can override via package
// vars where useful, but the defaults reflect production layout.
const (
	defaultEtcDir         = "/etc/claude-agent"
	defaultHostKeyPath    = "/etc/claude-agent/ssh_host_ed25519_key"
	defaultAgentIDPath    = "/etc/claude-agent/agent-id"
	defaultBinaryPath     = "/usr/local/bin/claude-agent"
	defaultSystemdUnit    = "/etc/systemd/system/claude-agent.service"
	defaultAuthKeysPath   = "/home/claude/.ssh/authorized_keys"
	defaultAuthKeysDir    = "/home/claude/.ssh"
	defaultClaudeHomeDir  = "/home/claude"
	defaultClaudeUsername = "claude"
	defaultListen         = ":222"
)

// Agent ID format: 12 lowercase alphanumeric characters. Generated once and
// persisted so a rebooted agent keeps the same identity on the shell's log
// directory and status registry.
const (
	agentIDLength = 12
	// alphabet omits visually-ambiguous 0/o and 1/l/i to make IDs easier to
	// read in logs and CLIs — still alphanumeric per spec.
	agentIDAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"
)

// generateAgentID returns a fresh random 12-char lowercase alphanumeric ID.
var generateAgentID = func() (string, error) {
	buf := make([]byte, agentIDLength)
	for i := range buf {
		idx, err := randIndex(len(agentIDAlphabet))
		if err != nil {
			return "", err
		}
		buf[i] = agentIDAlphabet[idx]
	}
	return string(buf), nil
}

func randIndex(n int) (int, error) {
	var b [1]byte
	for {
		if _, err := rand.Read(b[:]); err != nil {
			return 0, fmt.Errorf("rand read: %w", err)
		}
		// Modulo bias is negligible at n=31 against a uniform byte; accept it.
		return int(b[0]) % n, nil
	}
}

// loadOrCreateAgentID reads the agent ID from path. If the file is missing,
// a fresh ID is generated and written atomically.
func loadOrCreateAgentID(path string) (string, error) {
	if data, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	id, err := generateAgentID()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(id+"\n"), 0644); err != nil {
		return "", fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return id, nil
}
