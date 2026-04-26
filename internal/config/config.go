// Package config provides configuration constants and paths for convocate.
package config

import (
	"fmt"
	"os/user"
	"path/filepath"
)

const (
	// AppName is the application name.
	AppName = "convocate"

	// ContainerImageName is the Docker image name for convocate sessions.
	ContainerImageName = "convocate"

	// ContainerImageTag is the Docker image tag.
	ContainerImageTag = "latest"

	// ContainerPrefix is the prefix for container names.
	ContainerPrefix = "convocate-session-"

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

	// ConvocateBinaryPath is the installed path for the convocate binary.
	ConvocateBinaryPath = "/usr/local/bin/convocate"

	// DockerSocket is the path to the Docker socket.
	DockerSocket = "/var/run/docker.sock"

	// LockFileExtension is the extension for session lock files.
	LockFileExtension = ".lock"

	// TmuxSessionName is the name of the tmux session inside the container.
	TmuxSessionName = "claude"
)

// Paths holds resolved filesystem paths for convocate.
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

// ContainerImage returns the full image reference with the legacy
// :latest tag. Kept for tests and for callers that have no version
// context; production code on both shell and agent should prefer
// ContainerImageWithTag with the concrete semver.
func ContainerImage() string {
	return ContainerImageWithTag(ContainerImageTag)
}

// ContainerImageWithTag returns "convocate:<tag>". When tag is empty
// it defaults to ContainerImageTag so callers can pass a user-supplied
// version without null-checking.
func ContainerImageWithTag(tag string) string {
	if tag == "" {
		tag = ContainerImageTag
	}
	return fmt.Sprintf("%s:%s", ContainerImageName, tag)
}

// ContainerName returns the container name for a given session UUID.
func ContainerName(uuid string) string {
	return ContainerPrefix + uuid
}
