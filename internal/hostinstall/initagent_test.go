package hostinstall

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// testInitAgentOpts returns options pointed at a temp binary + temp shell
// etc dir, ready to pass to InitAgent for unit tests.
func testInitAgentOpts(t *testing.T) (InitAgentOptions, string) {
	t.Helper()
	binDir := t.TempDir()
	bin := filepath.Join(binDir, "claude-agent")
	if err := os.WriteFile(bin, []byte("#!fake-agent-binary"), 0755); err != nil {
		t.Fatal(err)
	}
	etcDir := t.TempDir()
	return InitAgentOptions{
		BinaryPath:       bin,
		ShellHost:        "shell.example.com",
		LocalShellEtcDir: etcDir,
	}, etcDir
}

func newAgentMockRunner(agentID string) *mockRunner {
	return &mockRunner{
		cmdStdout: map[string]string{
			// init-agent reads the agent-id via `cat`.
			"cat /etc/claude-agent/agent-id": agentID + "\n",
		},
	}
}

func TestInitAgent_RequiresShellHost(t *testing.T) {
	opts, _ := testInitAgentOpts(t)
	opts.ShellHost = ""
	m := newAgentMockRunner("whatever")
	err := InitAgent(context.Background(), m, nil, opts, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "--shell-host") {
		t.Errorf("expected shell-host-required error, got %v", err)
	}
}

func TestInitAgent_MissingBinary(t *testing.T) {
	opts := InitAgentOptions{
		BinaryPath: "/does/not/exist/claude-agent",
		ShellHost:  "shell.example.com",
	}
	m := newAgentMockRunner("x")
	err := InitAgent(context.Background(), m, nil, opts, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "locate claude-agent") {
		t.Errorf("expected locate error, got %v", err)
	}
}

func TestInitAgent_EndToEnd(t *testing.T) {
	opts, etcDir := testInitAgentOpts(t)
	m := newAgentMockRunner("abcdef012345")
	var log bytes.Buffer
	if err := InitAgent(context.Background(), m, nil, opts, &log); err != nil {
		t.Fatalf("InitAgent failed: %v\nlog:\n%s", err, log.String())
	}

	// One binary upload + three writeRemoteContent calls = 4 copies.
	if len(m.copies) != 4 {
		t.Fatalf("expected 4 copies, got %d: %+v", len(m.copies), copyDsts(m.copies))
	}
	wantDests := map[string]os.FileMode{
		"/usr/local/bin/claude-agent":                      0755,
		"/home/claude/.ssh/authorized_keys":                0600,
		"/etc/claude-agent/agent_to_shell_ed25519_key":     0600,
		"/etc/claude-agent/shell-host":                     0644,
	}
	for _, c := range m.copies {
		mode, ok := wantDests[c.Dst]
		if !ok {
			t.Errorf("unexpected copy dest %q", c.Dst)
			continue
		}
		if c.Mode != mode {
			t.Errorf("%s: mode = %o, want %o", c.Dst, c.Mode, mode)
		}
	}

	// Verify the shell-host file carries the trimmed value.
	shellHostCopy := findCopy(m.copies, "/etc/claude-agent/shell-host")
	if shellHostCopy == nil || strings.TrimSpace(string(shellHostCopy.Content)) != "shell.example.com" {
		t.Errorf("shell-host copy content = %q", shellHostCopy.Content)
	}

	// The private key pushed to the agent must parse as an SSH key and must
	// be tagged with the agent-id.
	privCopy := findCopy(m.copies, "/etc/claude-agent/agent_to_shell_ed25519_key")
	if privCopy == nil {
		t.Fatal("agent->shell private key not copied")
	}
	if _, err := ssh.ParsePrivateKey(privCopy.Content); err != nil {
		t.Errorf("agent->shell key doesn't parse: %v", err)
	}

	// authorized_keys on the agent must contain a valid public key line
	// tagged with the matching comment.
	authCopy := findCopy(m.copies, "/home/claude/.ssh/authorized_keys")
	if authCopy == nil {
		t.Fatal("authorized_keys not copied")
	}
	pub, comment, _, _, err := ssh.ParseAuthorizedKey(authCopy.Content)
	if err != nil {
		t.Fatalf("auth key parse: %v", err)
	}
	if pub == nil {
		t.Error("nil pub key")
	}
	if !strings.Contains(comment, "abcdef012345") {
		t.Errorf("comment %q missing agent id", comment)
	}

	// Remote Run calls should include the install, a cat for agent-id,
	// three chowns (one per writeRemoteContent), and a service restart.
	// Exact ordering: install, cat, chown auth, chown privkey, chown
	// shell-host, restart.
	wantSubstrings := []string{
		"/usr/local/bin/claude-agent install",
		"cat /etc/claude-agent/agent-id",
		"chown claude:claude '/home/claude/.ssh/authorized_keys'",
		"chown claude:claude '/etc/claude-agent/agent_to_shell_ed25519_key'",
		"chown root:root '/etc/claude-agent/shell-host'",
		"systemctl restart claude-agent.service",
	}
	if len(m.cmds) != len(wantSubstrings) {
		t.Fatalf("cmd count = %d, want %d:\n%v", len(m.cmds), len(wantSubstrings), cmdNames(m.cmds))
	}
	for i, want := range wantSubstrings {
		if !strings.Contains(m.cmds[i].Cmd, want) {
			t.Errorf("cmd[%d] = %q, want substring %q", i, firstLine(m.cmds[i].Cmd), want)
		}
	}
	for i, c := range m.cmds {
		if !c.Opts.Sudo {
			t.Errorf("cmd[%d] should be sudo", i)
		}
	}

	// --- shell-side local files -------------------------------------------
	// status_authorized_keys should have been created with exactly one
	// public-key line (the agent->shell pubkey we generated).
	authPath := filepath.Join(etcDir, "status_authorized_keys")
	authData, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("read %s: %v", authPath, err)
	}
	sshPub, authComment, _, _, err := ssh.ParseAuthorizedKey(authData)
	if err != nil {
		t.Fatalf("parse local authorized_keys: %v", err)
	}
	if sshPub == nil {
		t.Error("nil authorized key")
	}
	if !strings.Contains(authComment, "abcdef012345") {
		t.Errorf("comment = %q, expected agent id", authComment)
	}

	// shell->agent private key was staged under agent-keys/<id>/.
	keyPath := filepath.Join(etcDir, "agent-keys", "abcdef012345", "shell_to_agent_ed25519_key")
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read %s: %v", keyPath, err)
	}
	if _, err := ssh.ParsePrivateKey(keyData); err != nil {
		t.Errorf("shell->agent private key doesn't parse: %v", err)
	}
	// Mode must be 0600 — this key grants full CRUD of the agent's
	// containers and should never be world-readable.
	info, _ := os.Stat(keyPath)
	if info.Mode().Perm() != 0600 {
		t.Errorf("shell->agent key mode = %o, want 0600", info.Mode().Perm())
	}

	// agent-host file records where to dial.
	hostPath := filepath.Join(etcDir, "agent-keys", "abcdef012345", "agent-host")
	hostData, err := os.ReadFile(hostPath)
	if err != nil {
		t.Fatalf("read %s: %v", hostPath, err)
	}
	if strings.TrimSpace(string(hostData)) != "mock" {
		t.Errorf("agent-host = %q, want 'mock' (mockRunner.Target())", hostData)
	}
}

func TestInitAgent_AppendsToExistingAuthorizedKeys(t *testing.T) {
	// Prepopulate status_authorized_keys with one line, then run init-agent
	// and verify the file grows rather than being clobbered.
	opts, etcDir := testInitAgentOpts(t)
	authPath := filepath.Join(etcDir, "status_authorized_keys")
	existing := []byte("# existing\nssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGzZxHIT0xU4WIaMDIbj5DD/exxxxxxxxxxxxxxxxxxx existing\n")
	if err := os.WriteFile(authPath, existing, 0644); err != nil {
		t.Fatal(err)
	}
	m := newAgentMockRunner("zzz")
	if err := InitAgent(context.Background(), m, nil, opts, io.Discard); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(after, existing) {
		t.Error("pre-existing content was clobbered")
	}
	if !bytes.Contains(after, []byte("agent=zzz")) {
		t.Error("new key entry missing")
	}
}

func TestInitAgent_EmptyAgentIDIsError(t *testing.T) {
	opts, _ := testInitAgentOpts(t)
	m := newAgentMockRunner("") // cat returns empty
	err := InitAgent(context.Background(), m, nil, opts, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "agent-id") {
		t.Errorf("expected agent-id error, got %v", err)
	}
}

// --- small helpers ---------------------------------------------------------

func findCopy(copies []mockCopy, dst string) *mockCopy {
	for i := range copies {
		if copies[i].Dst == dst {
			return &copies[i]
		}
	}
	return nil
}

func copyDsts(copies []mockCopy) []string {
	out := make([]string, len(copies))
	for i, c := range copies {
		out[i] = c.Dst
	}
	return out
}
