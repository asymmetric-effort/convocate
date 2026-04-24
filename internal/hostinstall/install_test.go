package hostinstall

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// mockRunner records every Run/CopyFile call and can be told to fail on the
// Nth Run. Covers the install/init-* orchestration without needing a real
// host or SSH connection.
type mockRunner struct {
	cmds       []mockCall
	copies     []mockCopy
	failAt     int // if > 0, Run returns an error on the failAt'th call
	callNum    int
	returnTxt  string
	closeCount int

	// cmdStdout lets a test inject stdout content for a specific command
	// prefix (matched with strings.HasPrefix) — used to simulate reading
	// the agent-id file over SSH.
	cmdStdout map[string]string
}

type mockCall struct {
	Cmd  string
	Opts RunOptions
}

type mockCopy struct {
	Src, Dst string
	Mode     os.FileMode
	Content  []byte // contents of Src at the moment of CopyFile
}

func (m *mockRunner) Run(_ context.Context, cmd string, opts RunOptions) error {
	m.callNum++
	m.cmds = append(m.cmds, mockCall{Cmd: cmd, Opts: opts})
	if opts.Stdout != nil {
		if m.returnTxt != "" {
			_, _ = io.WriteString(opts.Stdout, m.returnTxt)
		}
		for prefix, out := range m.cmdStdout {
			if strings.HasPrefix(cmd, prefix) {
				_, _ = io.WriteString(opts.Stdout, out)
			}
		}
	}
	if m.failAt > 0 && m.callNum == m.failAt {
		return errors.New("mock-fail")
	}
	return nil
}

func (m *mockRunner) CopyFile(_ context.Context, src, dst string, mode os.FileMode) error {
	content, _ := os.ReadFile(src)
	m.copies = append(m.copies, mockCopy{Src: src, Dst: dst, Mode: mode, Content: content})
	return nil
}

func (*mockRunner) Target() string { return "mock" }

func (m *mockRunner) Close() error { m.closeCount++; return nil }

// --- orchestrator tests ----------------------------------------------------

func TestInstall_LocalHappyPath(t *testing.T) {
	var log bytes.Buffer
	m := &mockRunner{}
	if err := Install(context.Background(), m, nil, &log); err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	// Phase 1 (platform + apt) + phase 2 (8 steps) = 10 calls.
	if len(m.cmds) != 10 {
		t.Errorf("expected 10 commands, got %d", len(m.cmds))
	}
	if !strings.Contains(log.String(), "local mode: skipping automatic reboot") {
		t.Error("local mode should warn about skipping reboot")
	}
}

func TestInstall_StopsOnFirstError(t *testing.T) {
	m := &mockRunner{failAt: 1}
	err := Install(context.Background(), m, nil, io.Discard)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Check platform") {
		t.Errorf("error should mention the failing step, got: %v", err)
	}
	if len(m.cmds) != 1 {
		t.Errorf("should have stopped after first failure, got %d calls", len(m.cmds))
	}
}

// --- per-step invariants ---------------------------------------------------
//
// Each test verifies a) the right command is issued, b) the Sudo flag is
// set where a sudo-requiring operation is expected.

func TestStep_CheckPlatform_NoSudo(t *testing.T) {
	m := &mockRunner{}
	if err := stepCheckPlatform(context.Background(), m, io.Discard); err != nil {
		t.Fatal(err)
	}
	if m.cmds[0].Opts.Sudo {
		t.Error("platform check should not require sudo")
	}
	if !strings.Contains(m.cmds[0].Cmd, "os-release") {
		t.Errorf("expected os-release probe, got: %s", m.cmds[0].Cmd)
	}
}

func TestStep_AptUpgrade_Sudo(t *testing.T) {
	m := &mockRunner{}
	if err := stepAptUpgrade(context.Background(), m, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !m.cmds[0].Opts.Sudo {
		t.Error("apt upgrade requires sudo")
	}
	for _, want := range []string{"apt-get update", "dist-upgrade", "DEBIAN_FRONTEND=noninteractive"} {
		if !strings.Contains(m.cmds[0].Cmd, want) {
			t.Errorf("apt upgrade cmd missing %q: %s", want, m.cmds[0].Cmd)
		}
	}
}

func TestStep_InstallDocker_InstallsDockerIo(t *testing.T) {
	m := &mockRunner{}
	if err := stepInstallDocker(context.Background(), m, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.cmds[0].Cmd, "docker.io") {
		t.Errorf("expected docker.io install, got: %s", m.cmds[0].Cmd)
	}
	if !strings.Contains(m.cmds[0].Cmd, "systemctl enable --now docker") {
		t.Errorf("expected docker service enable, got: %s", m.cmds[0].Cmd)
	}
	if !m.cmds[0].Opts.Sudo {
		t.Error("docker install requires sudo")
	}
}

func TestStep_InstallDnsmasq(t *testing.T) {
	m := &mockRunner{}
	if err := stepInstallDnsmasq(context.Background(), m, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.cmds[0].Cmd, "dnsmasq") {
		t.Errorf("expected dnsmasq install, got: %s", m.cmds[0].Cmd)
	}
}

func TestStep_DisableResolvedStub_WritesDropIn(t *testing.T) {
	m := &mockRunner{}
	if err := stepDisableResolvedStub(context.Background(), m, io.Discard); err != nil {
		t.Fatal(err)
	}
	cmd := m.cmds[0].Cmd
	for _, want := range []string{
		"/etc/systemd/resolved.conf.d",
		"DNSStubListener=no",
		"systemctl restart systemd-resolved",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("stub-disable cmd missing %q", want)
		}
	}
}

func TestStep_CreateClaudeUser_Idempotent(t *testing.T) {
	m := &mockRunner{}
	if err := stepCreateClaudeUser(context.Background(), m, io.Discard); err != nil {
		t.Fatal(err)
	}
	cmd := m.cmds[0].Cmd
	// Must not fail if the user already exists.
	if !strings.Contains(cmd, "id claude") {
		t.Errorf("expected existence check before useradd, got: %s", cmd)
	}
	if !strings.Contains(cmd, "useradd -u 1337") {
		t.Errorf("expected uid 1337, got: %s", cmd)
	}
	if !strings.Contains(cmd, "usermod -aG docker claude") {
		t.Errorf("expected docker group membership, got: %s", cmd)
	}
}

func TestStep_EnableUFW_DefaultsDeny(t *testing.T) {
	m := &mockRunner{}
	if err := stepEnableUFW(context.Background(), m, io.Discard); err != nil {
		t.Fatal(err)
	}
	cmd := m.cmds[0].Cmd
	for _, want := range []string{
		"ufw default deny incoming",
		"ufw default allow outgoing",
		"ufw --force enable",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("ufw cmd missing %q", want)
		}
	}
}

func TestStep_SetTimezoneUTC(t *testing.T) {
	m := &mockRunner{}
	if err := stepSetTimezoneUTC(context.Background(), m, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.cmds[0].Cmd, "timedatectl set-timezone Etc/UTC") {
		t.Errorf("unexpected cmd: %s", m.cmds[0].Cmd)
	}
}

func TestStep_UnattendedUpgrades(t *testing.T) {
	m := &mockRunner{}
	if err := stepUnattendedUpgrades(context.Background(), m, io.Discard); err != nil {
		t.Fatal(err)
	}
	cmd := m.cmds[0].Cmd
	if !strings.Contains(cmd, "unattended-upgrades") {
		t.Errorf("expected unattended-upgrades install, got: %s", cmd)
	}
}

// --- LocalRunner round-trip ------------------------------------------------

func TestLocalRunner_Run_Success(t *testing.T) {
	r := NewLocalRunner()
	var out bytes.Buffer
	err := r.Run(context.Background(), "echo hello", RunOptions{Stdout: &out})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), "hello") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

func TestLocalRunner_Run_NonZeroExit(t *testing.T) {
	r := NewLocalRunner()
	err := r.Run(context.Background(), "exit 7", RunOptions{})
	if err == nil {
		t.Fatal("expected error from non-zero exit")
	}
}

func TestLocalRunner_Target(t *testing.T) {
	if NewLocalRunner().Target() != "local" {
		t.Errorf("wrong target")
	}
}

func TestLocalRunner_CopyFile(t *testing.T) {
	r := NewLocalRunner()
	src := t.TempDir() + "/src"
	dst := t.TempDir() + "/dst"
	if err := os.WriteFile(src, []byte("hi"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := r.CopyFile(context.Background(), src, dst, 0755); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hi" {
		t.Errorf("got %q, want 'hi'", data)
	}
	info, _ := os.Stat(dst)
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 0755", info.Mode().Perm())
	}
}

func TestLocalRunner_CopyFile_MissingSource(t *testing.T) {
	r := NewLocalRunner()
	err := r.CopyFile(context.Background(), "/nonexistent/file/does/not/exist", "/tmp/whatever", 0644)
	if err == nil {
		t.Error("expected error for missing source")
	}
}
