package shellserver

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/asymmetric-effort/convocate/internal/statusproto"
)

func generateKey(t *testing.T) (ssh.Signer, ssh.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	p, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("pub: %v", err)
	}
	return s, p
}

func writeAuthFile(t *testing.T, path string, pubs ...ssh.PublicKey) {
	t.Helper()
	var buf bytes.Buffer
	for _, p := range pubs {
		buf.Write(ssh.MarshalAuthorizedKey(p))
	}
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

type collectingListener struct {
	mu     sync.Mutex
	events []statusproto.Event
	errFn  func(ev statusproto.Event) error
}

func (c *collectingListener) HandleEvent(_ context.Context, ev statusproto.Event) error {
	c.mu.Lock()
	c.events = append(c.events, ev)
	c.mu.Unlock()
	if c.errFn != nil {
		return c.errFn(ev)
	}
	return nil
}

func (c *collectingListener) snapshot() []statusproto.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]statusproto.Event, len(c.events))
	copy(out, c.events)
	return out
}

// startServer is a small helper: returns the listen addr, a cancel func,
// and the collecting listener.
func startServer(t *testing.T, clientPub ssh.PublicKey) (string, context.CancelFunc, *collectingListener) {
	t.Helper()
	dir := t.TempDir()
	hostKey := filepath.Join(dir, "host_key")
	authFile := filepath.Join(dir, "auth")
	writeAuthFile(t, authFile, clientPub)

	listener := &collectingListener{}
	addr := freePort(t)

	srv, err := New(Config{
		HostKeyPath:        hostKey,
		AuthorizedKeysPath: authFile,
		Listen:             addr,
		Listener:           listener,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Serve(ctx) }()
	time.Sleep(40 * time.Millisecond)
	return addr, cancel, listener
}

func TestShellServer_PushSingleEvent(t *testing.T) {
	signer, pub := generateKey(t)
	addr, cancel, listener := startServer(t, pub)
	defer cancel()

	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            "agent",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         3 * time.Second,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer sess.Close()

	if err := sess.RequestSubsystem(statusproto.Subsystem); err != nil {
		t.Fatalf("subsystem: %v", err)
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	ev := statusproto.NewEvent(statusproto.TypeAgentStarted, "agent-1", "")
	enc, _ := json.Marshal(ev)
	_, _ = stdin.Write(append(enc, '\n'))
	_ = stdin.Close()

	// Wait until the event surfaces (or fail after a short grace period).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(listener.snapshot()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	got := listener.snapshot()
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if got[0].Type != statusproto.TypeAgentStarted {
		t.Errorf("type = %q, want %q", got[0].Type, statusproto.TypeAgentStarted)
	}
	if got[0].AgentID != "agent-1" {
		t.Errorf("agent_id = %q", got[0].AgentID)
	}
}

func TestShellServer_MultipleEvents(t *testing.T) {
	signer, pub := generateKey(t)
	addr, cancel, listener := startServer(t, pub)
	defer cancel()

	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            "agent",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	if err := sess.RequestSubsystem(statusproto.Subsystem); err != nil {
		t.Fatal(err)
	}
	stdin, _ := sess.StdinPipe()

	events := []string{
		statusproto.TypeAgentStarted,
		statusproto.TypeContainerCreated,
		statusproto.TypeContainerStarted,
		statusproto.TypeAgentHeartbeat,
	}
	for _, typ := range events {
		ev := statusproto.NewEvent(typ, "agent-2", "sess-x")
		enc, _ := json.Marshal(ev)
		_, _ = stdin.Write(append(enc, '\n'))
	}
	_ = stdin.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(listener.snapshot()) == len(events) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	got := listener.snapshot()
	if len(got) != len(events) {
		t.Fatalf("got %d events, want %d", len(got), len(events))
	}
	for i := range events {
		if got[i].Type != events[i] {
			t.Errorf("event[%d].Type = %q, want %q", i, got[i].Type, events[i])
		}
	}
}

func TestShellServer_RejectsExecAndShell(t *testing.T) {
	signer, pub := generateKey(t)
	addr, cancel, _ := startServer(t, pub)
	defer cancel()

	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            "agent",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Exec
	sess, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := sess.Run("rm -rf /"); err == nil {
		t.Error("expected exec to be rejected")
	}
	sess.Close()

	// Unknown subsystem
	sess2, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := sess2.RequestSubsystem("sftp"); err == nil {
		t.Error("expected unknown subsystem to be rejected")
	}
	sess2.Close()
}

func TestShellServer_UnauthorizedKeyRejected(t *testing.T) {
	// Authorize one key, try to connect with a different one.
	_, authorized := generateKey(t)
	addr, cancel, _ := startServer(t, authorized)
	defer cancel()
	outsider, _ := generateKey(t)

	_, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            "agent",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(outsider)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         3 * time.Second,
	})
	if err == nil {
		t.Fatal("expected dial to fail")
	}
}

func TestShellServer_MalformedEventDoesNotTerminate(t *testing.T) {
	signer, pub := generateKey(t)
	addr, cancel, listener := startServer(t, pub)
	defer cancel()

	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            "agent",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	if err := sess.RequestSubsystem(statusproto.Subsystem); err != nil {
		t.Fatal(err)
	}
	stdin, _ := sess.StdinPipe()

	// Send a junk line, then a valid one — server should drop the bad one
	// and process the good one.
	_, _ = stdin.Write([]byte("{not json}\n"))
	ev := statusproto.NewEvent(statusproto.TypeAgentHeartbeat, "a", "")
	enc, _ := json.Marshal(ev)
	_, _ = stdin.Write(append(enc, '\n'))
	_ = stdin.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(listener.snapshot()) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(listener.snapshot()) != 1 {
		t.Fatalf("expected 1 event, got %d", len(listener.snapshot()))
	}
}

// --- New / config errors --------------------------------------------------

func TestNew_RequiresListener(t *testing.T) {
	_, err := New(Config{HostKeyPath: "/tmp/x"})
	if err == nil {
		t.Fatal("expected error without Listener")
	}
}

func TestNew_DefaultsListenPort(t *testing.T) {
	// Bad paths still error, but we can check defaulting by passing a
	// valid temp dir for the host key.
	dir := t.TempDir()
	srv, err := New(Config{
		HostKeyPath:        filepath.Join(dir, "hk"),
		AuthorizedKeysPath: filepath.Join(dir, "auth"),
		Listener:           ListenerFunc(func(_ context.Context, _ statusproto.Event) error { return nil }),
	})
	if err != nil {
		t.Fatal(err)
	}
	if srv.cfg.Listen != ":223" {
		t.Errorf("default Listen = %q, want :223", srv.cfg.Listen)
	}
}

// --- payload parser -------------------------------------------------------

func TestParseStringPayload(t *testing.T) {
	// SSH subsystem payload: uint32 length + string bytes.
	payload := []byte{0, 0, 0, 5, 'h', 'e', 'l', 'l', 'o'}
	if got := parseStringPayload(payload); got != "hello" {
		t.Errorf("got %q, want 'hello'", got)
	}
	if got := parseStringPayload(nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
	// Length claims more bytes than present.
	if got := parseStringPayload([]byte{0, 0, 0, 99, 'x'}); got != "" {
		t.Errorf("got %q, want empty (length overflow)", got)
	}
}

// --- runStatusStream unit -------------------------------------------------

func TestRunStatusStream_StopsOnEOF(t *testing.T) {
	listener := &collectingListener{}
	srv := &Server{cfg: Config{Listener: listener, Logger: newSilentLogger()}}
	ev := statusproto.NewEvent(statusproto.TypeAgentStarted, "a", "")
	enc, _ := json.Marshal(ev)
	rdr := bytes.NewReader(append(enc, '\n'))

	done := make(chan struct{})
	go func() {
		srv.runStatusStream(context.Background(), rdr)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runStatusStream did not return on EOF")
	}
	if n := len(listener.snapshot()); n != 1 {
		t.Errorf("got %d events, want 1", n)
	}
}

// newSilentLogger returns a logger that discards output — keeps test noise
// down when exercising error paths.
func newSilentLogger() *log.Logger { return log.New(io.Discard, "", 0) }

func TestListenerFunc_HandleEvent(t *testing.T) {
	var got statusproto.Event
	wantErr := errors.New("listener says no")
	lf := ListenerFunc(func(_ context.Context, ev statusproto.Event) error {
		got = ev
		return wantErr
	})

	in := statusproto.Event{Type: "x", AgentID: "a", SessionID: "s"}
	if err := lf.HandleEvent(context.Background(), in); err != wantErr {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
	if got.Type != in.Type || got.AgentID != in.AgentID {
		t.Errorf("event not delivered: %+v", got)
	}
}
