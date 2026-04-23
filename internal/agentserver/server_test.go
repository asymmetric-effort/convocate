package agentserver

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// --- hostkey ---------------------------------------------------------------

func TestLoadOrCreateHostKey_GeneratesAndReuses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host_key")

	first, err := LoadOrCreateHostKey(path)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if first == nil {
		t.Fatal("expected signer, got nil")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat host key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("host key mode = %v, want 0600", info.Mode().Perm())
	}

	second, err := LoadOrCreateHostKey(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !bytes.Equal(first.PublicKey().Marshal(), second.PublicKey().Marshal()) {
		t.Error("expected stable host key across reloads")
	}
}

func TestLoadOrCreateHostKey_MalformedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host_key")
	if err := os.WriteFile(path, []byte("not a key"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadOrCreateHostKey(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

// --- authorized keys ------------------------------------------------------

func generateAuthKey(t *testing.T) (ssh.Signer, ssh.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("parse pub: %v", err)
	}
	return signer, sshPub
}

func writeAuthorizedKeys(t *testing.T, path string, keys ...ssh.PublicKey) {
	t.Helper()
	var buf bytes.Buffer
	for _, k := range keys {
		buf.Write(ssh.MarshalAuthorizedKey(k))
	}
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorizedKeys_MatchesPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth")
	_, pub := generateAuthKey(t)
	_, strangerPub := generateAuthKey(t)
	writeAuthorizedKeys(t, path, pub)

	a, err := NewAuthorizedKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	if !a.IsAuthorized(pub) {
		t.Error("expected pub to be authorized")
	}
	if a.IsAuthorized(strangerPub) {
		t.Error("expected unknown pub to be rejected")
	}
	if a.Len() != 1 {
		t.Errorf("Len = %d, want 1", a.Len())
	}
}

func TestAuthorizedKeys_MissingFileEmptyAllowlist(t *testing.T) {
	a, err := NewAuthorizedKeys("/definitely/not/here")
	if err != nil {
		t.Fatalf("missing file must not error: %v", err)
	}
	if a.Len() != 0 {
		t.Errorf("Len = %d, want 0", a.Len())
	}
	_, pub := generateAuthKey(t)
	if a.IsAuthorized(pub) {
		t.Error("empty allowlist must reject everything")
	}
}

func TestAuthorizedKeys_CommentsAndBlanksSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth")
	_, pub := generateAuthKey(t)
	content := append([]byte("# leading comment\n\n"), ssh.MarshalAuthorizedKey(pub)...)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	a, err := NewAuthorizedKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.Len() != 1 {
		t.Errorf("Len = %d, want 1", a.Len())
	}
}

func TestAuthorizedKeys_Reload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	a, err := NewAuthorizedKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	_, pub := generateAuthKey(t)
	writeAuthorizedKeys(t, path, pub)
	if err := a.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !a.IsAuthorized(pub) {
		t.Error("reload did not pick up new key")
	}
}

// --- dispatcher ------------------------------------------------------------

func TestDispatcher_Handle_OK(t *testing.T) {
	d := NewDispatcher()
	d.Register("echo", func(p json.RawMessage) (any, error) {
		var in map[string]string
		if err := json.Unmarshal(p, &in); err != nil {
			return nil, err
		}
		return in, nil
	})
	req := strings.NewReader(`{"op":"echo","params":{"hello":"world"}}`)
	var out bytes.Buffer
	d.Handle(req, &out)
	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK {
		t.Errorf("response ok=false: %+v", resp)
	}
	if !strings.Contains(string(resp.Result), "world") {
		t.Errorf("result missing payload: %s", resp.Result)
	}
}

func TestDispatcher_UnknownOp(t *testing.T) {
	d := NewDispatcher()
	req := strings.NewReader(`{"op":"nope"}`)
	var out bytes.Buffer
	d.Handle(req, &out)
	var resp Response
	_ = json.Unmarshal(out.Bytes(), &resp)
	if resp.OK {
		t.Error("expected ok=false for unknown op")
	}
	if !strings.Contains(resp.Error, "unknown op") {
		t.Errorf("error = %q, want 'unknown op'", resp.Error)
	}
}

func TestDispatcher_MalformedJSON(t *testing.T) {
	d := NewDispatcher()
	var out bytes.Buffer
	d.Handle(strings.NewReader("this isn't json"), &out)
	var resp Response
	_ = json.Unmarshal(out.Bytes(), &resp)
	if resp.OK || !strings.Contains(resp.Error, "malformed") {
		t.Errorf("got %+v", resp)
	}
}

func TestDispatcher_HandlerError(t *testing.T) {
	d := NewDispatcher()
	d.Register("boom", func(_ json.RawMessage) (any, error) {
		return nil, io.EOF
	})
	var out bytes.Buffer
	d.Handle(strings.NewReader(`{"op":"boom"}`), &out)
	var resp Response
	_ = json.Unmarshal(out.Bytes(), &resp)
	if resp.OK {
		t.Error("expected ok=false")
	}
}

func TestDispatcher_Register_PanicOnDup(t *testing.T) {
	d := NewDispatcher()
	d.Register("op", func(_ json.RawMessage) (any, error) { return nil, nil })
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate op")
		}
	}()
	d.Register("op", func(_ json.RawMessage) (any, error) { return nil, nil })
}

func TestDispatcher_OpsSorted(t *testing.T) {
	d := NewDispatcher()
	for _, op := range []string{"zz", "aa", "mm"} {
		d.Register(op, func(_ json.RawMessage) (any, error) { return nil, nil })
	}
	ops := d.Ops()
	for i := 1; i < len(ops); i++ {
		if ops[i-1] > ops[i] {
			t.Errorf("ops not sorted: %v", ops)
		}
	}
}

// --- end-to-end SSH + ping -------------------------------------------------

func TestServer_PingRoundTrip(t *testing.T) {
	// Spin up a real server on an ephemeral port, connect with a Go SSH
	// client, request the claude-agent-rpc subsystem, write a ping, assert
	// the response. This is the canonical "the whole stack works" test.
	dir := t.TempDir()
	hostKeyPath := filepath.Join(dir, "host_key")
	authKeysPath := filepath.Join(dir, "auth")

	clientSigner, clientPub := generateAuthKey(t)
	writeAuthorizedKeys(t, authKeysPath, clientPub)

	d := NewDispatcher()
	RegisterCoreOps(d, "abc123def456", "test-0.0.0")

	srv, err := New(Config{
		HostKeyPath:        hostKeyPath,
		AuthorizedKeysPath: authKeysPath,
		Listen:             "127.0.0.1:0", // ephemeral
		Dispatcher:         d,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Launch Serve and capture the chosen port. Since Config.Listen is used
	// directly by net.Listen, we have to fish the real port out after. For
	// this test we just use a known port and hope for the best — switch to
	// an injectable listener if we see flakes.
	listenAddr := findFreePort(t)
	srv.cfg.Listen = listenAddr

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()

	// Give Serve a moment to bind.
	time.Sleep(50 * time.Millisecond)

	clientCfg := &ssh.ClientConfig{
		User:            "claude",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         3 * time.Second,
	}
	client, err := ssh.Dial("tcp", listenAddr, clientCfg)
	if err != nil {
		t.Fatalf("client dial: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := session.RequestSubsystem(RPCSubsystem); err != nil {
		t.Fatalf("request subsystem: %v", err)
	}
	if _, err := stdin.Write([]byte(`{"op":"ping"}`)); err != nil {
		t.Fatal(err)
	}
	_ = stdin.Close()

	// Read until the server closes the channel; RequestSubsystem-driven
	// sessions don't emit an exit-status, so session.Wait() can't be used.
	respBytes, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	var resp Response
	if err := json.Unmarshal(bytes.TrimSpace(respBytes), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, respBytes)
	}
	if !resp.OK {
		t.Fatalf("ping ok=false: %+v", resp)
	}
	var r PingResult
	if err := json.Unmarshal(resp.Result, &r); err != nil {
		t.Fatalf("decode ping result: %v", err)
	}
	if r.AgentID != "abc123def456" {
		t.Errorf("agent_id = %q, want abc123def456", r.AgentID)
	}
	if r.Version != "test-0.0.0" {
		t.Errorf("version = %q", r.Version)
	}
	if r.ServerTime == "" {
		t.Error("expected server time")
	}

	cancel()
	if err := <-done; err != nil {
		t.Errorf("Serve: %v", err)
	}
}

func TestServer_ShellAndExecRejected(t *testing.T) {
	// Confirm the "no arbitrary command execution" invariant — any exec or
	// shell request on a session channel must be rejected cleanly.
	dir := t.TempDir()
	hostKeyPath := filepath.Join(dir, "host_key")
	authKeysPath := filepath.Join(dir, "auth")
	clientSigner, clientPub := generateAuthKey(t)
	writeAuthorizedKeys(t, authKeysPath, clientPub)

	d := NewDispatcher()
	RegisterCoreOps(d, "id", "v")
	srv, err := New(Config{
		HostKeyPath:        hostKeyPath,
		AuthorizedKeysPath: authKeysPath,
		Listen:             findFreePort(t),
		Dispatcher:         d,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()
	time.Sleep(50 * time.Millisecond)

	client, err := ssh.Dial("tcp", srv.cfg.Listen, &ssh.ClientConfig{
		User:            "claude",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Exec request
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	err = session.Run("rm -rf /")
	if err == nil {
		t.Error("exec must be rejected")
	}
	session.Close()

	// Shell request
	session2, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := session2.Shell(); err == nil {
		// Shell() failing here is the pass condition.
		_ = session2.Wait()
	}
	session2.Close()

	// Bogus subsystem
	session3, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := session3.RequestSubsystem("some-other-subsystem"); err == nil {
		t.Error("unknown subsystem must be rejected")
	}
	session3.Close()

	cancel()
	_ = <-done
}

func TestServer_UnauthorizedKeyRejected(t *testing.T) {
	dir := t.TempDir()
	authKeysPath := filepath.Join(dir, "auth")
	// Intentionally empty file -> no one is authorized.
	if err := os.WriteFile(authKeysPath, nil, 0600); err != nil {
		t.Fatal(err)
	}

	clientSigner, _ := generateAuthKey(t)

	d := NewDispatcher()
	RegisterCoreOps(d, "id", "v")
	srv, err := New(Config{
		HostKeyPath:        filepath.Join(dir, "hk"),
		AuthorizedKeysPath: authKeysPath,
		Listen:             findFreePort(t),
		Dispatcher:         d,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()
	time.Sleep(50 * time.Millisecond)

	_, err = ssh.Dial("tcp", srv.cfg.Listen, &ssh.ClientConfig{
		User:            "claude",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         3 * time.Second,
	})
	if err == nil {
		t.Fatal("expected dial to fail for unauthorized client")
	}
	cancel()
	_ = <-done
}

// findFreePort asks the kernel for an ephemeral port, closes the listener,
// and returns the resulting "127.0.0.1:<port>" string. Not perfectly
// race-free (another process could grab the port in between), but good
// enough for unit tests.
func findFreePort(t *testing.T) string {
	t.Helper()
	ln, err := netListenReal("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}
