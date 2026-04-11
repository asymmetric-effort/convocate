// Package skel manages the skeleton directory for new claude-shell sessions.
package skel

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultClaudeMD = `# Claude Session

## Environment

This is an isolated Claude session running inside a Docker container.

## Available Tools

- **Languages**: Go, Python 3, Node.js
- **Build Tools**: build-essential, cmake, pkg-config
- **Utilities**: git, curl, wget, jq, ripgrep, tmux, vim, nano
- **Network**: SSH client, full network access

## Session Isolation

This session has its own home directory and filesystem namespace.
Changes made here will not affect other sessions.

## Shared Configuration

Claude settings, credentials, and plugins are shared read-only from the host
via the ` + "`~/.claude-shared/`" + ` directory.
`

// Setup creates the skeleton directory with default files.
func Setup(skelPath string) error {
	if err := os.MkdirAll(skelPath, 0750); err != nil {
		return fmt.Errorf("failed to create skel directory: %w", err)
	}

	claudeMDPath := filepath.Join(skelPath, "CLAUDE.md")
	if _, err := os.Stat(claudeMDPath); os.IsNotExist(err) {
		if err := os.WriteFile(claudeMDPath, []byte(defaultClaudeMD), 0644); err != nil {
			return fmt.Errorf("failed to write CLAUDE.md: %w", err)
		}
	}

	return nil
}

// Exists checks if the skeleton directory exists and has the required files.
func Exists(skelPath string) bool {
	info, err := os.Stat(skelPath)
	if err != nil || !info.IsDir() {
		return false
	}
	claudeMD := filepath.Join(skelPath, "CLAUDE.md")
	_, err = os.Stat(claudeMD)
	return err == nil
}
