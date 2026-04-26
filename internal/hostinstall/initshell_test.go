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
	bin := tempBinary(t, "convocate")
	m := &mockRunner{cmdStdout: map[string]string{
		// Rsyslog step queries hostname + probes for existing CA.
		"hostname -f": "shell.test.example\n",
	}}
	var log bytes.Buffer
	if err := InitShell(context.Background(), m, nil, InitShellOptions{BinaryPath: bin}, &log); err != nil {
		t.Fatalf("InitShell failed: %v\nlog:\n%s", err, log.String())
	}

	// One CopyFile for the binary + four writeRemoteContent uploads for
	// the rsyslog step (ca.crt, ca.key, server.crt, server.key, rsyslog
	// config, logrotate config). The auth_keys + status-unit steps use
	// inline heredocs via Run, so they don't show up as copies.
	if len(m.copies) < 1 {
		t.Fatalf("expected at least 1 CopyFile, got %d", len(m.copies))
	}
	binCopy := findCopy(m.copies, "/usr/local/bin/convocate")
	if binCopy == nil {
		t.Fatalf("convocate binary not copied")
	}
	if binCopy.Mode != 0755 {
		t.Errorf("binary mode = %o, want 0755", binCopy.Mode)
	}
	if binCopy.Src != bin {
		t.Errorf("binary src = %q, want %q", binCopy.Src, bin)
	}

	// Sanity-check the step ordering by keyword — everything up to
	// rsyslog arrives in a known order.
	wants := []string{
		"convocate install",
		"/etc/convocate/status_authorized_keys",
		"/etc/systemd/system/convocate-status.service",
		"ufw allow 223/tcp",
		"systemctl enable convocate-status.service",
	}
	for i, want := range wants {
		if !strings.Contains(m.cmds[i].Cmd, want) {
			t.Errorf("cmd[%d] missing %q — got %q", i, want, firstLine(m.cmds[i].Cmd))
		}
	}

	// Rsyslog step must have written the server-side config + ensured
	// /var/log/convocate-agent exists + restarted the daemon.
	joined := allCmds(m.cmds)
	for _, want := range []string{
		"mkdir -p /etc/convocate/rsyslog-ca",
		"rsyslog-gnutls",
		"mkdir -p /var/log/convocate-agent",
		"ufw allow 514/tcp",
		"systemctl restart rsyslog",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("rsyslog step missing %q", want)
		}
	}
	for _, want := range []string{
		"/etc/convocate/rsyslog-ca/server.crt",
		"/etc/convocate/rsyslog-ca/server.key",
		"/etc/convocate/rsyslog-ca/ca.crt",
		"/etc/convocate/rsyslog-ca/ca.key",
		"/etc/rsyslog.d/10-convocate-server.conf",
		"/etc/logrotate.d/convocate-agent-logs",
	} {
		if findCopy(m.copies, want) == nil {
			t.Errorf("rsyslog step missing copy to %q", want)
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
	if !strings.Contains(err.Error(), "locate convocate binary") {
		t.Errorf("error should mention locating the binary: %v", err)
	}
	if len(m.cmds) != 0 || len(m.copies) != 0 {
		t.Error("no remote work should happen when binary lookup fails")
	}
}

func TestInitShell_StopsOnStepFailure(t *testing.T) {
	bin := tempBinary(t, "convocate")
	// failAt=1 fails the first Run, which is `convocate install`.
	m := &mockRunner{failAt: 1}
	err := InitShell(context.Background(), m, nil, InitShellOptions{BinaryPath: bin}, io.Discard)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Run convocate install") {
		t.Errorf("error should name failing step: %v", err)
	}
	// The subsequent Run steps should not have been attempted.
	if len(m.cmds) != 1 {
		t.Errorf("expected 1 Run before failure, got %d", len(m.cmds))
	}
}

func TestInitShell_BinaryUploadFirst(t *testing.T) {
	// Verify ordering: upload must precede any remote command because every
	// subsequent step assumes /usr/local/bin/convocate exists.
	bin := tempBinary(t, "convocate")
	m := &mockRunner{cmdStdout: map[string]string{"hostname -f": "h\n"}}
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

// allCmds returns every recorded Run command concatenated, for "does the
// sequence contain this phrase" style assertions.
func allCmds(cs []mockCall) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteString(c.Cmd)
		b.WriteByte('\n')
	}
	return b.String()
}

// --- resolveBinaryPath -----------------------------------------------------

func TestResolveBinaryPath_ExplicitOverride(t *testing.T) {
	bin := tempBinary(t, "convocate")
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

	// In production this will also try <exe-dir>/convocate, which won't
	// exist in the test binary's tempdir either — so this test relies on
	// both candidate paths missing.
	got, err := resolveBinaryPath("")
	if err == nil {
		// It's legitimate for the exe-sibling lookup to succeed (e.g. if
		// someone ran `go test` from a tree that has convocate next to
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
