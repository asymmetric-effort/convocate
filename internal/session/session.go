// Package session manages claude-shell sessions including creation, listing, deletion, and locking.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/asymmetric-effort/claude-shell/internal/config"
	"github.com/google/uuid"
)

// Metadata holds session metadata persisted as session.json.
type Metadata struct {
	UUID         string    `json:"uuid"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
	LastAccessed time.Time `json:"last_accessed"`
}

// Manager handles session lifecycle operations.
type Manager struct {
	basePath string
	skelPath string
}

// NewManager creates a new session Manager.
func NewManager(basePath, skelPath string) *Manager {
	return &Manager{
		basePath: basePath,
		skelPath: skelPath,
	}
}

// Create creates a new session with the given name and returns its metadata.
func (m *Manager) Create(name string) (Metadata, error) {
	id := uuid.New().String()
	return m.CreateWithUUID(id, name)
}

// CreateWithUUID creates a new session with a specific UUID (used for testing).
func (m *Manager) CreateWithUUID(id, name string) (Metadata, error) {
	sessionDir := filepath.Join(m.basePath, id)

	if err := m.copySkel(sessionDir); err != nil {
		return Metadata{}, fmt.Errorf("failed to initialize session directory: %w", err)
	}

	if err := m.setupClaudeSymlinks(sessionDir); err != nil {
		return Metadata{}, fmt.Errorf("failed to setup claude symlinks: %w", err)
	}

	now := time.Now().UTC()
	meta := Metadata{
		UUID:         id,
		Name:         name,
		CreatedAt:    now,
		LastAccessed: now,
	}

	if err := m.writeMetadata(sessionDir, meta); err != nil {
		_ = os.RemoveAll(sessionDir)
		return Metadata{}, fmt.Errorf("failed to write session metadata: %w", err)
	}

	return meta, nil
}

// List returns all existing sessions sorted by last accessed time (most recent first).
func (m *Manager) List() ([]Metadata, error) {
	entries, err := os.ReadDir(m.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []Metadata
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, err := uuid.Parse(name); err != nil {
			continue
		}
		meta, err := m.readMetadata(filepath.Join(m.basePath, name))
		if err != nil {
			continue
		}
		sessions = append(sessions, meta)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastAccessed.After(sessions[j].LastAccessed)
	})

	return sessions, nil
}

// Get retrieves metadata for a specific session.
func (m *Manager) Get(id string) (Metadata, error) {
	sessionDir := filepath.Join(m.basePath, id)
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		return Metadata{}, fmt.Errorf("session %q does not exist", id)
	}
	return m.readMetadata(sessionDir)
}

// Delete removes a session directory and its contents.
func (m *Manager) Delete(id string) error {
	sessionDir := filepath.Join(m.basePath, id)
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		return fmt.Errorf("session %q does not exist", id)
	}

	lockPath := filepath.Join(m.basePath, id+config.LockFileExtension)
	if _, err := os.Stat(lockPath); err == nil {
		return fmt.Errorf("session %q is currently locked (in use)", id)
	}

	return os.RemoveAll(sessionDir)
}

// Touch updates the last accessed time for a session.
func (m *Manager) Touch(id string) error {
	sessionDir := filepath.Join(m.basePath, id)
	meta, err := m.readMetadata(sessionDir)
	if err != nil {
		return err
	}
	meta.LastAccessed = time.Now().UTC()
	return m.writeMetadata(sessionDir, meta)
}

// Lock acquires an exclusive lock for a session. Returns an unlock function.
func (m *Manager) Lock(id string) (func(), error) {
	lockPath := filepath.Join(m.basePath, id+config.LockFileExtension)

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("session %q is already locked (another instance may be running)", id)
		}
		return nil, fmt.Errorf("failed to acquire lock for session %q: %w", id, err)
	}

	pid := os.Getpid()
	_, _ = fmt.Fprintf(f, "%d", pid)
	_ = f.Close()

	unlock := func() {
		_ = os.Remove(lockPath)
	}

	return unlock, nil
}

// IsLocked checks if a session is currently locked.
func (m *Manager) IsLocked(id string) bool {
	lockPath := filepath.Join(m.basePath, id+config.LockFileExtension)
	info, err := os.Stat(lockPath)
	if err != nil {
		return false
	}
	// Check if the lock is stale (older than 24 hours)
	if time.Since(info.ModTime()) > 24*time.Hour {
		_ = os.Remove(lockPath)
		return false
	}
	// Check if the PID in the lock file is still running
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return false
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		_ = os.Remove(lockPath)
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(lockPath)
		return false
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		_ = os.Remove(lockPath)
		return false
	}
	return true
}

// SessionDir returns the directory path for a session.
func (m *Manager) SessionDir(id string) string {
	return filepath.Join(m.basePath, id)
}

func (m *Manager) copySkel(dest string) error {
	if _, err := os.Stat(m.skelPath); os.IsNotExist(err) {
		return os.MkdirAll(dest, 0750)
	}

	return copyDir(m.skelPath, dest)
}

func (m *Manager) setupClaudeSymlinks(sessionDir string) error {
	claudeDir := filepath.Join(sessionDir, config.ClaudeConfigDir)
	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		return err
	}

	sharedBase := filepath.Join("/home/claude", config.ClaudeSharedDir)

	symlinks := []string{
		"settings.json",
		"settings.local.json",
		".credentials.json",
		"plugins",
	}

	for _, name := range symlinks {
		src := filepath.Join(sharedBase, name)
		dst := filepath.Join(claudeDir, name)
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		if err := os.Symlink(src, dst); err != nil {
			return fmt.Errorf("failed to symlink %s: %w", name, err)
		}
	}

	return nil
}

func (m *Manager) writeMetadata(sessionDir string, meta Metadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(sessionDir, config.SessionMetadataFile), data, 0644)
}

func (m *Manager) readMetadata(sessionDir string) (Metadata, error) {
	data, err := os.ReadFile(filepath.Join(sessionDir, config.SessionMetadataFile))
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to read session metadata: %w", err)
	}
	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return Metadata{}, fmt.Errorf("failed to parse session metadata: %w", err)
	}
	return meta, nil
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err := os.Symlink(link, dstPath); err != nil {
				return err
			}
			continue
		}

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, srcInfo.Mode())
}

// ValidateName checks if a session name is valid.
// Names may contain letters, digits, spaces, underscores, hyphens, and periods.
func ValidateName(name string) error {
	if len(strings.TrimSpace(name)) == 0 {
		return fmt.Errorf("session name cannot be empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("session name cannot exceed 64 characters")
	}
	for _, c := range name {
		if !isValidNameRune(c) {
			return fmt.Errorf("session name contains invalid character: %q (only letters, digits, spaces, _, -, . allowed)", c)
		}
	}
	return nil
}

// isValidNameRune returns true if the rune is allowed in a session name.
func isValidNameRune(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == ' ' || c == '_' || c == '-' || c == '.'
}
