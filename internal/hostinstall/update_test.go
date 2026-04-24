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

// newUpdateMockRunner returns a mockRunner whose `test -f ...` responses
// match installed (return "YES") for paths in the installed set, and
// "NO" otherwise.
func newUpdateMockRunner(installed []string) *mockRunner {
	stdout := map[string]string{}
	for _, p := range installed {
		// The exact command is `test -f '<path>' && echo YES || echo NO` —
		// we match by prefix using just the path fragment.
		key := "test -f '" + p + "'"
		stdout[key] = "YES\n"
	}
	return &mockRunner{cmdStdout: stdout, returnTxt: "NO\n"}
}

func tempBin(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestUpdate_ShellOnly(t *testing.T) {
	m := newUpdateMockRunner([]string{"/usr/local/bin/claude-shell"})
	shellBin := tempBin(t, "claude-shell")
	agentBin := tempBin(t, "claude-agent") // resolver needs this to exist
	var log bytes.Buffer
	err := Update(context.Background(), m, nil, UpdateOptions{
		ShellBinaryPath: shellBin,
		AgentBinaryPath: agentBin,
	}, &log)
	if err != nil {
		t.Fatalf("Update: %v\nlog:\n%s", err, log.String())
	}
	// Expect: probe shell (YES), upload shell, install shell, restart shell,
	// probe agent (NO). = 5 Run calls + 1 CopyFile.
	if len(m.copies) != 1 || m.copies[0].Dst != "/usr/local/bin/claude-shell" {
		t.Errorf("copies = %+v", m.copies)
	}
	wantCmds := []string{
		"test -f '/usr/local/bin/claude-shell'",
		"/usr/local/bin/claude-shell install",
		"systemctl restart claude-shell-status.service",
		"test -f '/usr/local/bin/claude-agent'",
	}
	if len(m.cmds) != len(wantCmds) {
		t.Fatalf("cmd count = %d, want %d: %v", len(m.cmds), len(wantCmds), cmdNames(m.cmds))
	}
	for i, want := range wantCmds {
		if !strings.Contains(m.cmds[i].Cmd, want) {
			t.Errorf("cmd[%d] = %q, want substring %q", i, firstLine(m.cmds[i].Cmd), want)
		}
	}
}

func TestUpdate_AgentOnly(t *testing.T) {
	m := newUpdateMockRunner([]string{"/usr/local/bin/claude-agent"})
	shellBin := tempBin(t, "claude-shell")
	agentBin := tempBin(t, "claude-agent")
	err := Update(context.Background(), m, nil, UpdateOptions{
		ShellBinaryPath: shellBin,
		AgentBinaryPath: agentBin,
	}, io.Discard)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(m.copies) != 1 || m.copies[0].Dst != "/usr/local/bin/claude-agent" {
		t.Errorf("expected single agent copy, got %+v", m.copies)
	}
}

func TestUpdate_Both(t *testing.T) {
	m := newUpdateMockRunner([]string{"/usr/local/bin/claude-shell", "/usr/local/bin/claude-agent"})
	shellBin := tempBin(t, "claude-shell")
	agentBin := tempBin(t, "claude-agent")
	err := Update(context.Background(), m, nil, UpdateOptions{
		ShellBinaryPath: shellBin,
		AgentBinaryPath: agentBin,
	}, io.Discard)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(m.copies) != 2 {
		t.Errorf("expected 2 copies, got %d", len(m.copies))
	}
	dests := map[string]bool{}
	for _, c := range m.copies {
		dests[c.Dst] = true
	}
	if !dests["/usr/local/bin/claude-shell"] || !dests["/usr/local/bin/claude-agent"] {
		t.Errorf("missing expected copy dest; got %v", dests)
	}
}

func TestUpdate_NoneInstalled_Errors(t *testing.T) {
	m := newUpdateMockRunner(nil) // nothing installed
	shellBin := tempBin(t, "claude-shell")
	agentBin := tempBin(t, "claude-agent")
	err := Update(context.Background(), m, nil, UpdateOptions{
		ShellBinaryPath: shellBin,
		AgentBinaryPath: agentBin,
	}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "no claude-*") {
		t.Errorf("expected 'no claude-*' error, got %v", err)
	}
}

func TestUpdate_MissingLocalBinary_Errors(t *testing.T) {
	m := newUpdateMockRunner([]string{"/usr/local/bin/claude-shell"})
	// Provide an override that does not exist.
	err := Update(context.Background(), m, nil, UpdateOptions{
		ShellBinaryPath: "/does/not/exist",
	}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "locate claude-shell") {
		t.Errorf("expected locate error, got %v", err)
	}
}

func TestRemoteFileExists(t *testing.T) {
	yes := newUpdateMockRunner([]string{"/some/path"})
	ok, err := remoteFileExists(context.Background(), yes, "/some/path", io.Discard)
	if err != nil || !ok {
		t.Errorf("expected exists, got ok=%v err=%v", ok, err)
	}
	no := newUpdateMockRunner(nil)
	ok, err = remoteFileExists(context.Background(), no, "/missing", io.Discard)
	if err != nil || ok {
		t.Errorf("expected missing, got ok=%v err=%v", ok, err)
	}
}
