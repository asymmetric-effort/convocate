package agentclient

import (
	"os"
	"path/filepath"
	"testing"
)

// writeAgentDir stages an agent-keys subdirectory with the files discovery
// checks for. Pass an empty host to omit the agent-host file (simulates a
// half-initialized dir).
func writeAgentDir(t *testing.T, root, id, host string, withKey bool) {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if withKey {
		if err := os.WriteFile(filepath.Join(dir, "shell_to_agent_ed25519_key"), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if host != "" {
		if err := os.WriteFile(filepath.Join(dir, "agent-host"), []byte(host+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDiscoverAgents_MissingDirReturnsEmpty(t *testing.T) {
	got, err := DiscoverAgents(filepath.Join(t.TempDir(), "not-there"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

func TestDiscoverAgents_Happy(t *testing.T) {
	root := t.TempDir()
	writeAgentDir(t, root, "abc123def456", "root@host1.example.com", true)
	writeAgentDir(t, root, "zzz999yyy888", "host2.example.com", true)

	got, err := DiscoverAgents(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2: %+v", len(got), got)
	}
	// Sorted lexically by ID: "abc..." before "zzz...".
	if got[0].ID != "abc123def456" {
		t.Errorf("got[0].ID = %q", got[0].ID)
	}
	// user@host prefix stripped.
	if got[0].Host != "host1.example.com" {
		t.Errorf("got[0].Host = %q, want host1.example.com", got[0].Host)
	}
	if got[1].Host != "host2.example.com" {
		t.Errorf("got[1].Host = %q", got[1].Host)
	}
	// PrivateKeyPath should point at the expected file inside the agent dir.
	wantKey := filepath.Join(root, "abc123def456", "shell_to_agent_ed25519_key")
	if got[0].PrivateKeyPath != wantKey {
		t.Errorf("got[0].PrivateKeyPath = %q, want %q", got[0].PrivateKeyPath, wantKey)
	}
}

func TestDiscoverAgents_SkipsIncompleteDirs(t *testing.T) {
	root := t.TempDir()
	// Missing key.
	writeAgentDir(t, root, "nokey12345", "host.example.com", false)
	// Missing host file.
	writeAgentDir(t, root, "nohost67890", "", true)
	// Empty host file.
	writeAgentDir(t, root, "emptyhost00", "   ", true)
	// A stray file (not a dir) at the top level.
	if err := os.WriteFile(filepath.Join(root, "scratch.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	// One complete record.
	writeAgentDir(t, root, "goodagent12", "host.example.com", true)

	got, err := DiscoverAgents(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "goodagent12" {
		t.Errorf("expected 1 record (goodagent12), got %+v", got)
	}
}

func TestDiscoverAgents_DefaultDir(t *testing.T) {
	// Empty dir arg should resolve to DefaultAgentKeysDir; on a CI host
	// that path doesn't exist, which is the "no agents" case.
	if _, err := os.Stat(DefaultAgentKeysDir); err == nil {
		t.Skip("default dir exists on this host, skipping")
	}
	got, err := DiscoverAgents("")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil slice for missing dir, got %+v", got)
	}
}
