//go:build integration

// hostinstall sshd sandbox integration: spin up an in-process
// crypto/ssh server, point hostinstall.NewSSHRunner at it, and exercise
// Run + CopyFile end-to-end. Closes coverage gaps the mock-based unit
// tests can't reach because they bypass the production SSH path.
package integration

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/asymmetric-effort/convocate/internal/hostinstall"
	"github.com/asymmetric-effort/convocate/internal/sshutil"
)

// fakeSSHd is a minimal SSH server that accepts public-key auth, accepts
// session channels, and responds to "exec" requests by invoking handler
// with the received command + stdin/stdout.
type fakeSSHd struct {
	t        *testing.T
	addr     string
	signer   ssh.Signer
	allowed  ssh.PublicKey
	handler  func(cmd string, stdin io.Reader, stdout io.Writer) (exit uint32, err error)
	listener net.Listener

	mu       sync.Mutex
	commands []string // every exec command we received
}

func newFakeSSHd(t *testing.T, allowed ssh.PublicKey, handler func(string, io.Reader, io.Writer) (uint32, error)) *fakeSSHd {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &fakeSSHd{
		t: t, addr: ln.Addr().String(), signer: signer,
		allowed: allowed, handler: handler, listener: ln,
	}
	go srv.serve()
	return srv
}

func (s *fakeSSHd) close() { _ = s.listener.Close() }

func (s *fakeSSHd) recordedCommands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.commands))
	copy(out, s.commands)
	return out
}

func (s *fakeSSHd) serve() {
	for {
		c, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(c)
	}
}

func (s *fakeSSHd) handleConn(nconn net.Conn) {
	defer nconn.Close()
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if s.allowed != nil && string(key.Marshal()) == string(s.allowed.Marshal()) {
				return &ssh.Permissions{}, nil
			}
			return nil, errors.New("unauthorized")
		},
		// PasswordCallback declared so the client's auth-method
		// negotiation includes "password" in the server's accepted-
		// list — required for the fall-through test that asserts the
		// client invokes its prompt callback. Always rejects.
		PasswordCallback: func(_ ssh.ConnMetadata, _ []byte) (*ssh.Permissions, error) {
			return nil, errors.New("passwords disabled")
		},
	}
	cfg.AddHostKey(s.signer)
	sshConn, chans, reqs, err := ssh.NewServerConn(nconn, cfg)
	if err != nil {
		return
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(reqs)
	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			_ = newCh.Reject(ssh.UnknownChannelType, "no")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			continue
		}
		go s.handleChannel(ch, chReqs)
	}
}

func (s *fakeSSHd) handleChannel(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		if req.Type != "exec" {
			_ = req.Reply(false, nil)
			continue
		}
		// "exec" payload is a length-prefixed command string.
		cmd := decodeStringPayload(req.Payload)
		s.mu.Lock()
		s.commands = append(s.commands, cmd)
		s.mu.Unlock()
		_ = req.Reply(true, nil)

		exit, herr := s.handler(cmd, ch, ch)
		if herr != nil {
			s.t.Logf("fake exec error: %v", herr)
		}
		// Send exit-status before closing, otherwise client's Run/Wait
		// returns *ExitError instead of nil even on success.
		statusPayload := make([]byte, 4)
		binary.BigEndian.PutUint32(statusPayload, exit)
		_, _ = ch.SendRequest("exit-status", false, statusPayload)
		return
	}
}

func decodeStringPayload(p []byte) string {
	if len(p) < 4 {
		return ""
	}
	n := binary.BigEndian.Uint32(p[:4])
	if int(4+n) > len(p) {
		return ""
	}
	return string(p[4 : 4+n])
}

// setupClientHome writes a fresh ed25519 client key to <tempdir>/.ssh/
// id_ed25519 and points $HOME at the tempdir. Returns the public key
// (for authorizing on the server) and the tempdir.
func setupClientHome(t *testing.T) (ssh.PublicKey, string) {
	t.Helper()
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(sshDir, "id_ed25519")
	signer, err := sshutil.LoadOrCreateHostKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("SSH_AUTH_SOCK", "")
	return signer.PublicKey(), home
}

func TestHostinstallSSH_Run_ExecutesAndCapturesStdout(t *testing.T) {
	clientPub, _ := setupClientHome(t)

	// Handler echoes the received command to stdout so we can assert
	// the wire framing carried our request through verbatim.
	srv := newFakeSSHd(t, clientPub, func(cmd string, stdin io.Reader, stdout io.Writer) (uint32, error) {
		fmt.Fprintf(stdout, "ran: %s\n", cmd)
		return 0, nil
	})
	defer srv.close()

	host, port := splitHostPort(t, srv.addr)
	r, err := hostinstall.NewSSHRunner(hostinstall.SSHConfig{
		Host: host, Port: port, User: "tester",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		PasswordPrompt:  func(string, string) (string, error) { return "", errors.New("never") },
		Timeout:         3 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewSSHRunner: %v", err)
	}
	defer r.Close()

	if got := r.Target(); got != "tester@"+host {
		t.Errorf("Target = %q, want tester@%s", got, host)
	}

	var out bytes.Buffer
	if err := r.Run(context.Background(), "uname -a", hostinstall.RunOptions{Stdout: &out}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), "ran: uname -a") {
		t.Errorf("stdout = %q", out.String())
	}

	// Sudo wrapping: the server should see the sudo'd form, not the raw
	// command, when opts.Sudo is true.
	out.Reset()
	if err := r.Run(context.Background(), "whoami", hostinstall.RunOptions{Sudo: true, Stdout: &out}); err != nil {
		t.Fatalf("Run sudo: %v", err)
	}
	cmds := srv.recordedCommands()
	last := cmds[len(cmds)-1]
	if !strings.Contains(last, "sudo -n -- bash -c") || !strings.Contains(last, "whoami") {
		t.Errorf("expected sudo wrapper in last command, got %q", last)
	}
}

func TestHostinstallSSH_Run_PropagatesNonzeroExit(t *testing.T) {
	clientPub, _ := setupClientHome(t)
	srv := newFakeSSHd(t, clientPub, func(_ string, _ io.Reader, _ io.Writer) (uint32, error) {
		return 7, nil // simulate exit 7
	})
	defer srv.close()

	host, port := splitHostPort(t, srv.addr)
	r, err := hostinstall.NewSSHRunner(hostinstall.SSHConfig{
		Host: host, Port: port, User: "u",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		PasswordPrompt:  func(string, string) (string, error) { return "", errors.New("nope") },
		Timeout:         3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	err = r.Run(context.Background(), "anything", hostinstall.RunOptions{})
	if err == nil {
		t.Error("expected error from non-zero exit")
	}
}

func TestHostinstallSSH_Run_HonorsCtxCancel(t *testing.T) {
	clientPub, _ := setupClientHome(t)
	srv := newFakeSSHd(t, clientPub, func(_ string, _ io.Reader, _ io.Writer) (uint32, error) {
		// Simulate a long-running remote command.
		time.Sleep(2 * time.Second)
		return 0, nil
	})
	defer srv.close()

	host, port := splitHostPort(t, srv.addr)
	r, err := hostinstall.NewSSHRunner(hostinstall.SSHConfig{
		Host: host, Port: port, User: "u",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		PasswordPrompt:  func(string, string) (string, error) { return "", errors.New("nope") },
		Timeout:         3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = r.Run(ctx, "sleep-equivalent", hostinstall.RunOptions{})
	if err == nil {
		t.Error("expected context-deadline error")
	}
}

func TestHostinstallSSH_CopyFile_StreamsBytesAndPropagatesExit(t *testing.T) {
	clientPub, _ := setupClientHome(t)

	// Server captures the streamed stdin so we can verify CopyFile
	// pumped the file body through correctly.
	var receivedStdin bytes.Buffer
	srv := newFakeSSHd(t, clientPub, func(cmd string, stdin io.Reader, _ io.Writer) (uint32, error) {
		// Drain stdin — that's the file body.
		_, err := io.Copy(&receivedStdin, stdin)
		return 0, err
	})
	defer srv.close()

	host, port := splitHostPort(t, srv.addr)
	r, err := hostinstall.NewSSHRunner(hostinstall.SSHConfig{
		Host: host, Port: port, User: "u",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		PasswordPrompt:  func(string, string) (string, error) { return "", errors.New("nope") },
		Timeout:         3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("hello there\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.CopyFile(context.Background(), src, "/etc/somewhere", 0640); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}
	if got := receivedStdin.String(); got != "hello there\n" {
		t.Errorf("server stdin = %q, want %q", got, "hello there\n")
	}
	cmds := srv.recordedCommands()
	if len(cmds) == 0 {
		t.Fatal("server saw no exec command")
	}
	if !strings.Contains(cmds[0], "install -D -m 640") || !strings.Contains(cmds[0], "/etc/somewhere") {
		t.Errorf("expected install/move command, got %q", cmds[0])
	}
}

func TestHostinstallSSH_CopyFile_MissingSrcErrors(t *testing.T) {
	clientPub, _ := setupClientHome(t)
	srv := newFakeSSHd(t, clientPub, func(string, io.Reader, io.Writer) (uint32, error) { return 0, nil })
	defer srv.close()
	host, port := splitHostPort(t, srv.addr)
	r, err := hostinstall.NewSSHRunner(hostinstall.SSHConfig{
		Host: host, Port: port, User: "u",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		PasswordPrompt:  func(string, string) (string, error) { return "", errors.New("nope") },
		Timeout:         3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.CopyFile(context.Background(), "/does/not/exist", "/dst", 0644); err == nil {
		t.Error("expected error for missing src")
	}
}

func TestHostinstallReboot_HappyPath(t *testing.T) {
	clientPub, _ := setupClientHome(t)
	srv := newFakeSSHd(t, clientPub, func(_ string, _ io.Reader, _ io.Writer) (uint32, error) {
		return 0, nil
	})
	defer srv.close()

	host, port := splitHostPort(t, srv.addr)
	cfg := hostinstall.SSHConfig{
		Host: host, Port: port, User: "u",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		PasswordPrompt:  func(string, string) (string, error) { return "", errors.New("nope") },
		Timeout:         3 * time.Second,
	}
	r, err := hostinstall.NewSSHRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var progress bytes.Buffer
	r2, err := hostinstall.RebootAndReconnect(context.Background(), r, cfg, hostinstall.RebootOptions{
		InitialWait:  10 * time.Millisecond,
		PollInterval: 10 * time.Millisecond,
		Timeout:      2 * time.Second,
		Progress:     &progress,
	})
	if err != nil {
		t.Fatalf("RebootAndReconnect: %v", err)
	}
	defer r2.Close()
	if !strings.Contains(progress.String(), "rebooting") {
		t.Errorf("progress should mention rebooting, got %q", progress.String())
	}
	if !strings.Contains(progress.String(), "reachable again") {
		t.Errorf("progress should mention reconnect success, got %q", progress.String())
	}
	// First two recorded commands: the reboot itself, then the
	// post-reconnect "true" reachability probe.
	cmds := srv.recordedCommands()
	if len(cmds) < 2 {
		t.Fatalf("expected at least 2 commands, got %v", cmds)
	}
	if !strings.Contains(cmds[0], "systemctl reboot") {
		t.Errorf("first command should be reboot, got %q", cmds[0])
	}
}

func TestHostinstallReboot_Timeout(t *testing.T) {
	clientPub, _ := setupClientHome(t)
	// Server shuts down before we attempt reconnect — every
	// NewSSHRunner attempt will fail with connection refused, so
	// RebootAndReconnect hits its Timeout.
	srv := newFakeSSHd(t, clientPub, func(_ string, _ io.Reader, _ io.Writer) (uint32, error) {
		return 0, nil
	})
	host, port := splitHostPort(t, srv.addr)
	cfg := hostinstall.SSHConfig{
		Host: host, Port: port, User: "u",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		PasswordPrompt:  func(string, string) (string, error) { return "", errors.New("nope") },
		Timeout:         200 * time.Millisecond,
	}
	r, err := hostinstall.NewSSHRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv.close() // kill the listener after the client connects

	_, err = hostinstall.RebootAndReconnect(context.Background(), r, cfg, hostinstall.RebootOptions{
		InitialWait:  10 * time.Millisecond,
		PollInterval: 50 * time.Millisecond,
		Timeout:      300 * time.Millisecond,
	})
	if err == nil || !strings.Contains(err.Error(), "did not come back") {
		t.Errorf("expected did-not-come-back error, got %v", err)
	}
}

func TestHostinstallSSH_KeyAuthRejected_FallsBackToPassword(t *testing.T) {
	// Set up a client key but DON'T authorize it on the server. The
	// server should reject pubkey auth, the client falls through to the
	// password callback, which returns an error → dial fails.
	_, _ = setupClientHome(t)

	// Different (un-authorized) signer for the server's allowlist.
	_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	otherSigner, _ := ssh.NewSignerFromKey(otherPriv)

	srv := newFakeSSHd(t, otherSigner.PublicKey(), func(string, io.Reader, io.Writer) (uint32, error) { return 0, nil })
	defer srv.close()

	host, port := splitHostPort(t, srv.addr)
	pwAsked := false
	_, err := hostinstall.NewSSHRunner(hostinstall.SSHConfig{
		Host: host, Port: port, User: "u",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		PasswordPrompt: func(string, string) (string, error) {
			pwAsked = true
			return "", errors.New("no password configured")
		},
		Timeout: 2 * time.Second,
	})
	if err == nil {
		t.Error("dial should fail when no auth method works")
	}
	if !pwAsked {
		t.Error("expected password prompt to be called as fallback")
	}
}
