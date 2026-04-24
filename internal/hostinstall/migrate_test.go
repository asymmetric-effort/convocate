package hostinstall

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrate_RequiresAgent(t *testing.T) {
	err := MigrateSession(context.Background(), MigrateSessionOptions{SessionUUID: "x"}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "--agent") {
		t.Errorf("expected agent-required, got %v", err)
	}
}

func TestMigrate_RequiresSession(t *testing.T) {
	err := MigrateSession(context.Background(), MigrateSessionOptions{AgentID: "a"}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "--session") {
		t.Errorf("expected session-required, got %v", err)
	}
}

func TestMigrate_MissingSessionJSON(t *testing.T) {
	base := t.TempDir()
	// Directory exists but no session.json inside.
	if err := os.MkdirAll(filepath.Join(base, "uuid-1"), 0755); err != nil {
		t.Fatal(err)
	}
	err := MigrateSession(context.Background(), MigrateSessionOptions{
		AgentID:           "a",
		SessionUUID:       "uuid-1",
		ShellSessionsBase: base,
	}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "session.json") {
		t.Errorf("expected session.json error, got %v", err)
	}
}

func TestMigrate_UnregisteredAgent(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "uuid-1"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "uuid-1", "session.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	keysDir := t.TempDir() // empty — no agents registered
	err := MigrateSession(context.Background(), MigrateSessionOptions{
		AgentID:           "not-registered",
		SessionUUID:       "uuid-1",
		ShellSessionsBase: base,
		AgentKeysDir:      keysDir,
	}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Errorf("expected not-registered error, got %v", err)
	}
}

func TestMigrate_EmptyAgentHost(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "uuid-1"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "uuid-1", "session.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	keysDir := t.TempDir()
	agentDir := filepath.Join(keysDir, "agent-1")
	if err := os.MkdirAll(agentDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Key present but host file empty.
	if err := os.WriteFile(filepath.Join(agentDir, "shell_to_agent_ed25519_key"), []byte("key"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "agent-host"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	err := MigrateSession(context.Background(), MigrateSessionOptions{
		AgentID:           "agent-1",
		SessionUUID:       "uuid-1",
		ShellSessionsBase: base,
		AgentKeysDir:      keysDir,
	}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "agent-host") {
		t.Errorf("expected agent-host error, got %v", err)
	}
}

func TestMigrate_TarAndSSHStubbedEndToEnd(t *testing.T) {
	// Happy path with stub tar + ssh on PATH so we exercise the whole
	// function without touching the network or writing a real tarball.
	base := t.TempDir()
	uuid := "abc-def-ghi"
	sess := filepath.Join(base, uuid)
	if err := os.MkdirAll(sess, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess, "session.json"),
		[]byte(`{"uuid":"abc-def-ghi","name":"test"}`), 0644); err != nil {
		t.Fatal(err)
	}

	keysDir := t.TempDir()
	agentDir := filepath.Join(keysDir, "the-agent")
	if err := os.MkdirAll(agentDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "shell_to_agent_ed25519_key"), []byte("stub"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "agent-host"), []byte("agent.example.com\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Stub tar + ssh: both exit 0 with no output. Ensures MigrateSession
	// wires the pipe correctly without needing a real target.
	stubDir := t.TempDir()
	for _, name := range []string{"tar", "ssh"} {
		if err := os.WriteFile(filepath.Join(stubDir, name), []byte("#!/bin/sh\ncat >/dev/null 2>&1 || true\nexit 0\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Keep PATH so lookpath on `docker` (preflight) still works the same
	// way it would in prod. We prepend our stubs so tar/ssh resolve to
	// them.
	t.Setenv("PATH", stubDir+":"+os.Getenv("PATH"))

	var log bytes.Buffer
	err := MigrateSession(context.Background(), MigrateSessionOptions{
		AgentID:           "the-agent",
		SessionUUID:       uuid,
		ShellSessionsBase: base,
		AgentKeysDir:      keysDir,
	}, &log)
	if err != nil {
		t.Fatalf("MigrateSession: %v", err)
	}
	if !strings.Contains(log.String(), "transfer complete") {
		t.Errorf("log missing completion line: %s", log.String())
	}
}

func TestMigrate_DeleteSource(t *testing.T) {
	base := t.TempDir()
	uuid := "to-delete"
	sess := filepath.Join(base, uuid)
	if err := os.MkdirAll(sess, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess, "session.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	keysDir := t.TempDir()
	ad := filepath.Join(keysDir, "a")
	_ = os.MkdirAll(ad, 0700)
	_ = os.WriteFile(filepath.Join(ad, "shell_to_agent_ed25519_key"), []byte("k"), 0600)
	_ = os.WriteFile(filepath.Join(ad, "agent-host"), []byte("host\n"), 0644)

	stubDir := t.TempDir()
	for _, name := range []string{"tar", "ssh"} {
		_ = os.WriteFile(filepath.Join(stubDir, name), []byte("#!/bin/sh\ncat >/dev/null 2>&1 || true\n"), 0755)
	}
	t.Setenv("PATH", stubDir+":"+os.Getenv("PATH"))

	if err := MigrateSession(context.Background(), MigrateSessionOptions{
		AgentID: "a", SessionUUID: uuid, ShellSessionsBase: base, AgentKeysDir: keysDir,
		DeleteSource: true,
	}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sess); !os.IsNotExist(err) {
		t.Error("source dir should have been removed")
	}
}
