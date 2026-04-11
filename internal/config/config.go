// Package config provides configuration constants and paths for claude-shell.
package config

import (
	"fmt"
	"os/user"
	"path/filepath"
)

const (
	// AppName is the application name.
	AppName = "claude-shell"

	// ContainerImageName is the Docker image name for claude-shell sessions.
	ContainerImageName = "claude-shell"

	// ContainerImageTag is the Docker image tag.
	ContainerImageTag = "latest"

	// ContainerPrefix is the prefix for container names.
	ContainerPrefix = "claude-session-"

	// ClaudeBinaryPath is the path to the claude CLI binary on the host.
	ClaudeBinaryPath = "/usr/local/bin/claude"

	// ClaudeUser is the username for the claude user.
	ClaudeUser = "claude"

	// SessionMetadataFile is the filename for session metadata.
	SessionMetadataFile = "session.json"

	// SkelDir is the skeleton directory name.
	SkelDir = ".skel"

	// ClaudeConfigDir is the claude configuration directory name.
	ClaudeConfigDir = ".claude"

	// ClaudeSharedDir is the mount point for shared claude config inside the container.
	ClaudeSharedDir = ".claude-shared"

	// ClaudeShellBinaryPath is the installed path for the claude-shell binary.
	ClaudeShellBinaryPath = "/usr/local/bin/claude-shell"

	// DockerSocket is the path to the Docker socket.
	DockerSocket = "/var/run/docker.sock"

	// LockFileExtension is the extension for session lock files.
	LockFileExtension = ".lock"
)

// Paths holds resolved filesystem paths for claude-shell.
type Paths struct {
	ClaudeHome   string
	SessionsBase string
	SkelDir      string
	ClaudeConfig string
	SSHDir       string
	GitConfig    string
}

// ResolvePaths resolcts all paths based on the claude user's home directory.
func ResolvePaths() (Paths, error) {
	u, err := user.Lookup(ClaudeUser)
	if err != nil {
		return Paths{}, fmt.Errorf("failed to lookup user %q: %w", ClaudeUser, err)
	}
	return PathsFromHome(u.HomeDir), nil
}

// PathsFromHome creates Paths from a given home directory.
func PathsFromHome(home string) Paths {
	return Paths{
		ClaudeHome:   home,
		SessionsBase: home,
		SkelDir:      filepath.Join(home, SkelDir),
		ClaudeConfig: filepath.Join(home, ClaudeConfigDir),
		SSHDir:       filepath.Join(home, ".ssh"),
		GitConfig:    filepath.Join(home, ".gitconfig"),
	}
}

// ContainerImage returns the full image reference.
func ContainerImage() string {
	return fmt.Sprintf("%s:%s", ContainerImageName, ContainerImageTag)
}

// ContainerName returns the container name for a given session UUID.
func ContainerName(uuid string) string {
	return ContainerPrefix + uuid
}
