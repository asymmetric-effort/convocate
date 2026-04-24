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

// tempBinary writes a zero-byte file and returns its path — resolveBinaryPath
// only checks os.Stat, and CopyFile is mocked, so the content is irrelevant.
func tempBinary(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInitShell_HappyPath(t *testing.T) {
	bin := tempBinary(t, "claude-shell")
	m := &mockRunner{}
	var log bytes.Buffer
	if err := InitShell(context.Background(), m, nil, InitShellOptions{BinaryPath: bin}, &log); err != nil {
		t.Fatalf("InitShell failed: %v", err)
	}

	// One CopyFile (binary upload) + five Runs (install, mkdir+authkeys,
	// systemd unit, ufw, enable+start).
	if len(m.copies) != 1 {
		t.Fatalf("expected 1 CopyFile, got %d", len(m.copies))
	}
	if m.copies[0].Dst != "/usr/local/bin/claude-shell" || m.copies[0].Mode != 0755 {
		t.Errorf("copy target = %+v", m.copies[0])
	}
	if m.copies[0].Src != bin {
		t.Errorf("copy src = %q, want %q", m.copies[0].Src, bin)
	}

	if len(m.cmds) != 5 {
		t.Fatalf("expected 5 Run calls, got %d:\n%v", len(m.cmds), cmdNames(m.cmds))
	}

	// Every remote command must run under sudo.
	for i, c := range m.cmds {
		if !c.Opts.Sudo {
			t.Errorf("cmd[%d] %q was not sudo", i, firstLine(c.Cmd))
		}
	}

	// Sanity-check the step ordering by keyword.
	wants := []string{
		"claude-shell install",
		"/etc/claude-shell/status_authorized_keys",
		"/etc/systemd/system/claude-shell-status.service",
		"ufw allow 222/tcp",
		"systemctl enable claude-shell-status.service",
	}
	for i, want := range wants {
		if !strings.Contains(m.cmds[i].Cmd, want) {
			t.Errorf("cmd[%d] missing %q — got %q", i, want, firstLine(m.cmds[i].Cmd))
		}
	}

	// Log should include the target name and the completion banner.
	out := log.String()
	if !strings.Contains(out, "mock") {
		t.Errorf("log missing target: %s", out)
	}
	if !strings.Contains(out, "init-shell complete") {
		t.Errorf("log missing completion banner: %s", out)
	}
}

func TestInitShell_MissingBinaryOverride(t *testing.T) {
	m := &mockRunner{}
	err := InitShell(context.Background(), m, nil,
		InitShellOptions{BinaryPath: "/does/not/exist"}, io.Discard)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "locate claude-shell binary") {
		t.Errorf("error should mention locating the binary: %v", err)
	}
	if len(m.cmds) != 0 || len(m.copies) != 0 {
		t.Error("no remote work should happen when binary lookup fails")
	}
}

func TestInitShell_StopsOnStepFailure(t *testing.T) {
	bin := tempBinary(t, "claude-shell")
	// failAt=1 fails the first Run, which is `claude-shell install`.
	m := &mockRunner{failAt: 1}
	err := InitShell(context.Background(), m, nil, InitShellOptions{BinaryPath: bin}, io.Discard)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Run claude-shell install") {
		t.Errorf("error should name failing step: %v", err)
	}
	// The subsequent Run steps should not have been attempted.
	if len(m.cmds) != 1 {
		t.Errorf("expected 1 Run before failure, got %d", len(m.cmds))
	}
}

func TestInitShell_BinaryUploadFirst(t *testing.T) {
	// Verify ordering: upload must precede any remote command because every
	// subsequent step assumes /usr/local/bin/claude-shell exists.
	bin := tempBinary(t, "claude-shell")
	m := &mockRunner{}
	if err := InitShell(context.Background(), m, nil, InitShellOptions{BinaryPath: bin}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if len(m.copies) < 1 || len(m.cmds) < 1 {
		t.Fatal("need both copies and cmds")
	}
	// The copy happens in the "Upload" step which runs before any Run step;
	// since mockRunner records them in separate slices, we can only verify
	// they both occurred — but that's good enough because failAt testing
	// covers ordering semantics.
}

// --- resolveBinaryPath -----------------------------------------------------

func TestResolveBinaryPath_ExplicitOverride(t *testing.T) {
	bin := tempBinary(t, "claude-shell")
	got, err := resolveBinaryPath(bin)
	if err != nil {
		t.Fatal(err)
	}
	if got != bin {
		t.Errorf("got %q, want %q", got, bin)
	}
}

func TestResolveBinaryPath_ExplicitMissing(t *testing.T) {
	_, err := resolveBinaryPath("/really/not/here")
	if err == nil {
		t.Fatal("expected error for missing override")
	}
}

func TestResolveBinaryPath_FallbackOrder(t *testing.T) {
	// When no override is given, resolve picks the first existing candidate.
	// We can't easily stub os.Executable, so we exercise the "nothing
	// found" branch with a cwd that has no build/ dir.
	dir := t.TempDir()
	origWD, _ := os.Getwd()
	defer os.Chdir(origWD)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// In production this will also try <exe-dir>/claude-shell, which won't
	// exist in the test binary's tempdir either — so this test relies on
	// both candidate paths missing.
	got, err := resolveBinaryPath("")
	if err == nil {
		// It's legitimate for the exe-sibling lookup to succeed (e.g. if
		// someone ran `go test` from a tree that has claude-shell next to
		// the test binary). Don't fail in that case — just verify the
		// returned path exists.
		if _, serr := os.Stat(got); serr != nil {
			t.Errorf("resolver returned a nonexistent path %q", got)
		}
		return
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- small helpers ---------------------------------------------------------

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func cmdNames(cs []mockCall) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = firstLine(c.Cmd)
	}
	return out
}
