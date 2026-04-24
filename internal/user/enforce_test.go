package user

import (
	"strings"
	"testing"
)

func TestEnforceRunningAs_MatchingUID(t *testing.T) {
	// Stash the real Getuid and replace with a function that returns the
	// lookup result so the check passes regardless of the test runner's
	// actual uid.
	info, err := Lookup("root")
	if err != nil {
		t.Skip("need the 'root' user to exist for this test")
	}
	orig := GetuidFn
	defer func() { GetuidFn = orig }()
	GetuidFn = func() int { return info.UID }

	if err := EnforceRunningAs("root"); err != nil {
		t.Errorf("matching uid should succeed, got: %v", err)
	}
}

func TestEnforceRunningAs_Mismatch(t *testing.T) {
	info, err := Lookup("root")
	if err != nil {
		t.Skip("need 'root' for this test")
	}
	orig := GetuidFn
	defer func() { GetuidFn = orig }()
	GetuidFn = func() int { return info.UID + 99999 }

	err = EnforceRunningAs("root")
	if err == nil {
		t.Fatal("expected error for uid mismatch")
	}
	// Error must name the expected user and the sudo recipe.
	msg := err.Error()
	for _, want := range []string{"root", "sudo -u root", "uid"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing %q: %v", want, err)
		}
	}
}

func TestEnforceRunningAs_UnknownUser(t *testing.T) {
	if err := EnforceRunningAs("user-that-cannot-possibly-exist-xyz"); err == nil {
		t.Error("expected error looking up nonexistent user")
	}
}
