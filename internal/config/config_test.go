package config

import (
	"testing"
)

func TestContainerImage(t *testing.T) {
	expected := ContainerImageName + ":" + ContainerImageTag
	got := ContainerImage()
	if got != expected {
		t.Errorf("ContainerImage() = %q, want %q", got, expected)
	}
}

func TestContainerImageWithTag(t *testing.T) {
	if got := ContainerImageWithTag("v9.9.9"); got != ContainerImageName+":v9.9.9" {
		t.Errorf("explicit tag = %q", got)
	}
	// Empty tag falls back to the compile-time default.
	if got := ContainerImageWithTag(""); got != ContainerImageName+":"+ContainerImageTag {
		t.Errorf("empty tag = %q", got)
	}
}

func TestContainerName(t *testing.T) {
	tests := []struct {
		name     string
		uuid     string
		expected string
	}{
		{
			name:     "valid uuid",
			uuid:     "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			expected: ContainerPrefix + "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		},
		{
			name:     "short string",
			uuid:     "test",
			expected: ContainerPrefix + "test",
		},
		{
			name:     "empty string",
			uuid:     "",
			expected: ContainerPrefix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainerName(tt.uuid)
			if got != expected(tt) {
				t.Errorf("ContainerName(%q) = %q, want %q", tt.uuid, got, tt.expected)
			}
		})
	}
}

func expected(tt struct {
	name     string
	uuid     string
	expected string
}) string {
	return tt.expected
}

func TestPathsFromHome(t *testing.T) {
	home := "/home/testuser"
	paths := PathsFromHome(home)

	if paths.ClaudeHome != home {
		t.Errorf("ClaudeHome = %q, want %q", paths.ClaudeHome, home)
	}
	if paths.SessionsBase != home {
		t.Errorf("SessionsBase = %q, want %q", paths.SessionsBase, home)
	}
	if paths.SkelDir != home+"/"+SkelDir {
		t.Errorf("SkelDir = %q, want %q", paths.SkelDir, home+"/"+SkelDir)
	}
	if paths.ClaudeConfig != home+"/"+ClaudeConfigDir {
		t.Errorf("ClaudeConfig = %q, want %q", paths.ClaudeConfig, home+"/"+ClaudeConfigDir)
	}
	if paths.SSHDir != home+"/.ssh" {
		t.Errorf("SSHDir = %q, want %q", paths.SSHDir, home+"/.ssh")
	}
	if paths.GitConfig != home+"/.gitconfig" {
		t.Errorf("GitConfig = %q, want %q", paths.GitConfig, home+"/.gitconfig")
	}
}

func TestResolvePaths_UserNotFound(t *testing.T) {
	// ResolvePaths looks up the "claude" user. In test environments this may
	// or may not exist, so we just verify it returns without panicking.
	_, err := ResolvePaths()
	if err != nil {
		// Expected in environments without the claude user
		t.Logf("ResolvePaths returned expected error: %v", err)
	}
}

func TestConstants(t *testing.T) {
	// Verify constants have sensible values
	if AppName == "" {
		t.Error("AppName is empty")
	}
	if ContainerImageName == "" {
		t.Error("ContainerImageName is empty")
	}
	if ContainerImageTag == "" {
		t.Error("ContainerImageTag is empty")
	}
	if ContainerPrefix == "" {
		t.Error("ContainerPrefix is empty")
	}
	if ClaudeBinaryPath == "" {
		t.Error("ClaudeBinaryPath is empty")
	}
	if ClaudeUser == "" {
		t.Error("ClaudeUser is empty")
	}
	if SessionMetadataFile == "" {
		t.Error("SessionMetadataFile is empty")
	}
	if SkelDir == "" {
		t.Error("SkelDir is empty")
	}
	if ClaudeConfigDir == "" {
		t.Error("ClaudeConfigDir is empty")
	}
	if ClaudeSharedDir == "" {
		t.Error("ClaudeSharedDir is empty")
	}
	if DockerSocket == "" {
		t.Error("DockerSocket is empty")
	}
	if LockFileExtension == "" {
		t.Error("LockFileExtension is empty")
	}
}
