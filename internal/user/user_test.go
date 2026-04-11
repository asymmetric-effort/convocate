package user

import (
	"fmt"
	"os/user"
	"testing"
)

func TestLookupWith_Success(t *testing.T) {
	mockLookup := func(username string) (*user.User, error) {
		return &user.User{
			Uid:      "1337",
			Gid:      "1337",
			Username: "claude",
			HomeDir:  "/home/claude",
		}, nil
	}

	info, err := LookupWith("claude", mockLookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.UID != 1337 {
		t.Errorf("UID = %d, want 1337", info.UID)
	}
	if info.GID != 1337 {
		t.Errorf("GID = %d, want 1337", info.GID)
	}
	if info.Username != "claude" {
		t.Errorf("Username = %q, want %q", info.Username, "claude")
	}
	if info.HomeDir != "/home/claude" {
		t.Errorf("HomeDir = %q, want %q", info.HomeDir, "/home/claude")
	}
}

func TestLookupWith_UserNotFound(t *testing.T) {
	mockLookup := func(username string) (*user.User, error) {
		return nil, fmt.Errorf("user not found")
	}

	_, err := LookupWith("nonexistent", mockLookup)
	if err == nil {
		t.Error("expected error for nonexistent user, got nil")
	}
}

func TestLookupWith_InvalidUID(t *testing.T) {
	mockLookup := func(username string) (*user.User, error) {
		return &user.User{
			Uid:      "notanumber",
			Gid:      "1337",
			Username: "claude",
			HomeDir:  "/home/claude",
		}, nil
	}

	_, err := LookupWith("claude", mockLookup)
	if err == nil {
		t.Error("expected error for invalid UID, got nil")
	}
}

func TestLookupWith_InvalidGID(t *testing.T) {
	mockLookup := func(username string) (*user.User, error) {
		return &user.User{
			Uid:      "1337",
			Gid:      "notanumber",
			Username: "claude",
			HomeDir:  "/home/claude",
		}, nil
	}

	_, err := LookupWith("claude", mockLookup)
	if err == nil {
		t.Error("expected error for invalid GID, got nil")
	}
}

func TestLookup_RealUser(t *testing.T) {
	// Test with a real user lookup - the current user should work
	info, err := Lookup("root")
	if err != nil {
		t.Skipf("root user not available: %v", err)
	}
	if info.UID != 0 {
		t.Errorf("root UID = %d, want 0", info.UID)
	}
}

func TestDefaultLookup(t *testing.T) {
	// Test DefaultLookup with a user that should exist
	u, err := DefaultLookup("root")
	if err != nil {
		t.Skipf("root user not available: %v", err)
	}
	if u.Username != "root" {
		t.Errorf("Username = %q, want %q", u.Username, "root")
	}
}

func TestDefaultLookup_NotFound(t *testing.T) {
	_, err := DefaultLookup("nonexistent_user_xyz_12345")
	if err == nil {
		t.Error("expected error for nonexistent user, got nil")
	}
}
