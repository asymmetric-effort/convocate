package hypervisor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockRunner records every call and lets tests pre-program return
// values. Used across every test in this package; keep field names
// stable so other test files can grep for them.
type mockRunner struct {
	cmds       []mockCall
	copies     []mockCopy
	reads      []string
	readBody   map[string][]byte // path → bytes returned from ReadFile
	cmdStdout  map[string]string // command-prefix → stdout fed to opts.Stdout
	failOn     map[string]error  // command-prefix → error to return from Run
	copyFailOn map[string]error  // dst-substring → error to return from CopyFile
	closed     bool
	target     string
}

type mockCall struct {
	Cmd  string
	Opts RunOptions
}

type mockCopy struct {
	Src, Dst string
	Mode     os.FileMode
	Content  []byte // best-effort: read at copy time
}

func (m *mockRunner) Run(_ context.Context, cmd string, opts RunOptions) error {
	m.cmds = append(m.cmds, mockCall{Cmd: cmd, Opts: opts})
	if opts.Stdout != nil && m.cmdStdout != nil {
		for prefix, out := range m.cmdStdout {
			if strings.HasPrefix(cmd, prefix) {
				_, _ = opts.Stdout.Write([]byte(out))
			}
		}
	}
	if m.failOn != nil {
		for prefix, err := range m.failOn {
			if strings.HasPrefix(cmd, prefix) {
				return err
			}
		}
	}
	return nil
}

func (m *mockRunner) CopyFile(_ context.Context, src, dst string, mode os.FileMode) error {
	content, _ := os.ReadFile(src)
	m.copies = append(m.copies, mockCopy{Src: src, Dst: dst, Mode: mode, Content: content})
	if m.copyFailOn != nil {
		for substr, err := range m.copyFailOn {
			if strings.Contains(dst, substr) {
				return err
			}
		}
	}
	return nil
}

func (m *mockRunner) ReadFile(_ context.Context, path string) ([]byte, error) {
	m.reads = append(m.reads, path)
	if m.readBody != nil {
		if b, ok := m.readBody[path]; ok {
			return b, nil
		}
	}
	return nil, nil
}

func (m *mockRunner) Target() string {
	if m.target != "" {
		return m.target
	}
	return "mock"
}

func (m *mockRunner) Close() error { m.closed = true; return nil }

// --- runner.go tests ---------------------------------------------------------

func TestSshQuoteArg_RoundTrip(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "'plain'"},
		{"with space", "'with space'"},
		{"with'quote", `'with'"'"'quote'`},
	}
	for _, tc := range cases {
		if got := shellQuoteArg(tc.in); got != tc.want {
			t.Errorf("shellQuoteArg(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestReplaceAll_Edge(t *testing.T) {
	// Empty old or empty s short-circuits — covers both branches.
	if got := replaceAll("anything", "", "x"); got != "anything" {
		t.Errorf("empty old should no-op, got %q", got)
	}
	if got := replaceAll("", "a", "b"); got != "" {
		t.Errorf("empty s should no-op, got %q", got)
	}
	if got := replaceAll("aaaa", "a", "bb"); got != "bbbbbbbb" {
		t.Errorf("got %q", got)
	}
	if got := replaceAll("hello world", "x", "y"); got != "hello world" {
		t.Errorf("missing substring should no-op, got %q", got)
	}
}

func TestIndexOf(t *testing.T) {
	if i := indexOf("abcdef", "cd"); i != 2 {
		t.Errorf("indexOf cd in abcdef = %d, want 2", i)
	}
	if i := indexOf("abc", "xyz"); i != -1 {
		t.Errorf("indexOf missing = %d, want -1", i)
	}
}

func TestTrimPubKey_StripsTrailingNewlines(t *testing.T) {
	got := trimPubKey([]byte("ssh-ed25519 AAAA... user\n\r\n"))
	if string(got) != "ssh-ed25519 AAAA... user" {
		t.Errorf("got %q", got)
	}
}

// --- InstallOperatorKey ------------------------------------------------------

func TestInstallOperatorKey_HappyPath(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id.pub")
	if err := os.WriteFile(keyPath, []byte("ssh-ed25519 AAAA fingerprint user@laptop\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := &mockRunner{}
	if err := InstallOperatorKey(context.Background(), m, keyPath); err != nil {
		t.Fatalf("InstallOperatorKey: %v", err)
	}
	if len(m.cmds) != 1 {
		t.Fatalf("expected 1 Run, got %d", len(m.cmds))
	}
	body := m.cmds[0].Cmd
	for _, want := range []string{
		"mkdir -p ~/.ssh",
		"chmod 700 ~/.ssh",
		"chmod 600 ~/.ssh/authorized_keys",
		"ssh-ed25519 AAAA fingerprint user@laptop",
		"grep -F -x",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("install script missing %q\n%s", want, body)
		}
	}
	if m.cmds[0].Opts.Sudo {
		t.Error("key install should NOT use sudo (writes to ~)")
	}
}

func TestInstallOperatorKey_MissingFile(t *testing.T) {
	m := &mockRunner{}
	err := InstallOperatorKey(context.Background(), m, "/does/not/exist")
	if err == nil || !strings.Contains(err.Error(), "read pubkey") {
		t.Errorf("expected read-pubkey error, got %v", err)
	}
	if len(m.cmds) != 0 {
		t.Error("no remote work should run when pubkey missing")
	}
}

func TestInstallOperatorKey_DefaultPath(t *testing.T) {
	// Override readOperatorPubKey to simulate a successful default
	// lookup — the production fallback walks ~/.ssh which we can't
	// reliably control in CI.
	orig := readOperatorPubKey
	defer func() { readOperatorPubKey = orig }()
	readOperatorPubKey = func(p string) ([]byte, string, error) {
		if p != "" {
			t.Errorf("expected empty path, got %q", p)
		}
		return []byte("ssh-ed25519 AAAA host"), "/home/op/.ssh/id_ed25519.pub", nil
	}
	m := &mockRunner{}
	if err := InstallOperatorKey(context.Background(), m, ""); err != nil {
		t.Fatal(err)
	}
	if len(m.cmds) != 1 || !strings.Contains(m.cmds[0].Cmd, "ssh-ed25519 AAAA host") {
		t.Errorf("install command shape wrong: %v", m.cmds)
	}
}

func TestInstallOperatorKey_NoCandidate(t *testing.T) {
	orig := readOperatorPubKey
	defer func() { readOperatorPubKey = orig }()
	readOperatorPubKey = func(p string) ([]byte, string, error) {
		return nil, "", errors.New("no operator pubkey found in ~/.ssh")
	}
	err := InstallOperatorKey(context.Background(), &mockRunner{}, "")
	if err == nil || !strings.Contains(err.Error(), "no operator pubkey") {
		t.Errorf("expected no-pubkey error, got %v", err)
	}
}

func TestInstallOperatorKey_RunFailure(t *testing.T) {
	orig := readOperatorPubKey
	defer func() { readOperatorPubKey = orig }()
	readOperatorPubKey = func(string) ([]byte, string, error) {
		return []byte("ssh-ed25519 AAAA k"), "/p", nil
	}
	m := &mockRunner{failOn: map[string]error{"set -e": errors.New("auth failed")}}
	err := InstallOperatorKey(context.Background(), m, "")
	if err == nil || !strings.Contains(err.Error(), "install operator key") {
		t.Errorf("expected install error, got %v", err)
	}
}

// --- HardenSSHD -------------------------------------------------------------

func TestHardenSSHD_WritesDropInAndReloads(t *testing.T) {
	m := &mockRunner{}
	if err := HardenSSHD(context.Background(), m); err != nil {
		t.Fatalf("HardenSSHD: %v", err)
	}
	if len(m.cmds) != 1 {
		t.Fatalf("expected 1 Run, got %d", len(m.cmds))
	}
	if !m.cmds[0].Opts.Sudo {
		t.Error("hardening must run as sudo (writes to /etc/ssh)")
	}
	body := m.cmds[0].Cmd
	for _, want := range []string{
		"/etc/ssh/sshd_config.d/10-convocate-hardening.conf",
		"PermitRootLogin no",
		"PasswordAuthentication no",
		"X11Forwarding no",
		"PubkeyAuthentication yes",
		"sshd -t",
		"systemctl reload ssh",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("hardening script missing %q\n%s", want, body)
		}
	}
}

func TestHardenSSHD_PropagatesError(t *testing.T) {
	m := &mockRunner{failOn: map[string]error{"set -e": errors.New("sshd -t bad config")}}
	err := HardenSSHD(context.Background(), m)
	if err == nil || !strings.Contains(err.Error(), "bad config") {
		t.Errorf("expected propagated error, got %v", err)
	}
}

// --- production sshRunner (read path) ---------------------------------------

// hostInstallStub satisfies hostinstall.Runner well enough for the
// sshRunner ReadFile/Run/CopyFile wrappers to be exercised.
type hostInstallStub struct {
	gotCmd  string
	gotOpts RunOptions
	stdout  string
	runErr  error
	copies  []mockCopy
	closed  bool
}

func (h *hostInstallStub) Run(_ context.Context, cmd string, opts RunOptions) error {
	h.gotCmd = cmd
	h.gotOpts = opts
	if opts.Stdout != nil && h.stdout != "" {
		_, _ = opts.Stdout.Write([]byte(h.stdout))
	}
	return h.runErr
}
func (h *hostInstallStub) CopyFile(_ context.Context, src, dst string, mode os.FileMode) error {
	h.copies = append(h.copies, mockCopy{Src: src, Dst: dst, Mode: mode})
	return nil
}
func (h *hostInstallStub) Target() string { return "u@h" }
func (h *hostInstallStub) Close() error   { h.closed = true; return nil }

func TestSshRunner_ReadFile_StreamsCat(t *testing.T) {
	stub := &hostInstallStub{stdout: "hello\n"}
	r := &sshRunner{ssh: stub}
	got, err := r.ReadFile(context.Background(), "/etc/os-release")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello\n" {
		t.Errorf("got %q", got)
	}
	if !strings.Contains(stub.gotCmd, "cat '/etc/os-release'") {
		t.Errorf("expected cat command, got %q", stub.gotCmd)
	}
}

func TestSshRunner_ReadFile_PropagatesErr(t *testing.T) {
	stub := &hostInstallStub{runErr: errors.New("nope")}
	r := &sshRunner{ssh: stub}
	if _, err := r.ReadFile(context.Background(), "/whatever"); err == nil ||
		!strings.Contains(err.Error(), "read /whatever") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestSshRunner_RunCopyFileTargetClose(t *testing.T) {
	stub := &hostInstallStub{}
	r := &sshRunner{ssh: stub}
	if err := r.Run(context.Background(), "uname -a", RunOptions{Sudo: true}); err != nil {
		t.Fatal(err)
	}
	if !stub.gotOpts.Sudo {
		t.Error("sudo flag not propagated")
	}
	if err := r.CopyFile(context.Background(), "/src", "/dst", 0644); err != nil {
		t.Fatal(err)
	}
	if len(stub.copies) != 1 {
		t.Errorf("expected 1 copy, got %d", len(stub.copies))
	}
	if got := r.Target(); got != "u@h" {
		t.Errorf("Target = %q", got)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if !stub.closed {
		t.Error("Close not propagated")
	}
}
