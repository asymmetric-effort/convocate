// Package skel manages the skeleton directory for new claude-shell sessions.
package skel

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/asymmetric-effort/claude-shell/internal/assets"
	"github.com/asymmetric-effort/claude-shell/internal/diskspace"
)

// Setup creates the skeleton directory with default files.
func Setup(skelPath string) error {
	if err := os.MkdirAll(skelPath, 0750); err != nil {
		return fmt.Errorf("failed to create skel directory: %w", err)
	}

	claudeMDPath := filepath.Join(skelPath, "CLAUDE.md")
	if _, err := os.Stat(claudeMDPath); os.IsNotExist(err) {
		content, err := assets.ClaudeMD()
		if err != nil {
			return fmt.Errorf("failed to extract embedded CLAUDE.md: %w", err)
		}
		if err := diskspace.CheckForFile(skelPath, int64(len(content))); err != nil {
			return err
		}
		if err := os.WriteFile(claudeMDPath, content, 0644); err != nil {
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
