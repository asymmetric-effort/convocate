package agentclient

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/asymmetric-effort/claude-shell/internal/shellserver"
	"github.com/asymmetric-effort/claude-shell/internal/sshutil"
	"github.com/asymmetric-effort/claude-shell/internal/statusproto"
)

// --- test helpers ----------------------------------------------------------

// keyPair generates a temp ed25519 SSH key on disk and returns (privatePath,
// publicKey). LoadOrCreateHostKey gives us a disk-backed OpenSSH-format key
// which is exactly what the emitter expects to read.
func keyPair(t *testing.T) (string, ssh.PublicKey) {
	t.Helper()
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "client_key")
	signer, err := sshutil.LoadOrCreateHostKey(keyPath)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	return keyPath, signer.PublicKey()
}

func writeAuthKeys(t *testing.T, path string, keys ...ssh.PublicKey) {
	t.Helper()
	var buf bytes.Buffer
	for _, k := range keys {
		buf.Write(ssh.MarshalAuthorizedKey(k))
	}
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func freePort(t *testing.T) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	a := ln.Addr().(*net.TCPAddr)
	return a.IP.String(), a.Port
}

type captureListener struct {
	mu     sync.Mutex
	events []statusproto.Event
}

func (c *captureListener) HandleEvent(_ context.Context, ev statusproto.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, ev)
	return nil
}

func (c *captureListener) snapshot() []statusproto.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]statusproto.Event, len(c.events))
	copy(out, c.events)
	return out
}

// startShell boots an in-process shellserver with a temp host key and an
// authorized_keys file containing clientPub. Returns (host, port, listener,
// cancel).
func startShell(t *testing.T, clientPub ssh.PublicKey) (string, int, *captureListener, context.CancelFunc) {
	t.Helper()
	dir := t.TempDir()
	hostKey := filepath.Join(dir, "host_key")
	authFile := filepath.Join(dir, "authorized_keys")
	writeAuthKeys(t, authFile, clientPub)

	host, port := freePort(t)
	listener := &captureListener{}
	srv, err := shellserver.New(shellserver.Config{
		HostKeyPath:        hostKey,
		AuthorizedKeysPath: authFile,
		Listen:             net.JoinHostPort(host, itoa(port)),
		Listener:           listener,
		Logger:             silentLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Serve(ctx) }()
	time.Sleep(50 * time.Millisecond)
	return host, port, listener, cancel
}

func itoa(n int) string {
	// Tiny helper; avoids pulling strconv into every call site in this test.
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var out []byte
	for n > 0 {
		out = append([]byte{digits[n%10]}, out...)
		n /= 10
	}
	return string(out)
}

func silentLogger() *log.Logger { return log.New(io.Discard, "", 0) }

// waitFor polls cond until it returns true or the deadline passes.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

// --- NewStatusEmitter validation ------------------------------------------

func TestNew_RequiresShellHost(t *testing.T) {
	keyPath, _ := keyPair(t)
	_, err := NewStatusEmitter(Config{AgentID: "a", PrivateKeyPath: keyPath})
	if err == nil {
		t.Fatal("expected error without ShellHost")
	}
}

func TestNew_RequiresAgentID(t *testing.T) {
	keyPath, _ := keyPair(t)
	_, err := NewStatusEmitter(Config{ShellHost: "h", PrivateKeyPath: keyPath})
	if err == nil {
		t.Fatal("expected error without AgentID")
	}
}

func TestNew_RequiresPrivateKey(t *testing.T) {
	_, err := NewStatusEmitter(Config{ShellHost: "h", AgentID: "a", PrivateKeyPath: ""})
	if err == nil {
		t.Fatal("expected error without key path")
	}
}

func TestNew_DefaultsApplied(t *testing.T) {
	keyPath, _ := keyPair(t)
	e, err := NewStatusEmitter(Config{ShellHost: "h", AgentID: "a", PrivateKeyPath: keyPath})
	if err != nil {
		t.Fatal(err)
	}
	if e.cfg.ShellPort != 223 {
		t.Errorf("ShellPort default = %d, want 223", e.cfg.ShellPort)
	}
	if e.cfg.User != "claude" {
		t.Errorf("User default = %q, want claude", e.cfg.User)
	}
	if e.cfg.BufferSize != 256 {
		t.Errorf("BufferSize default = %d, want 256", e.cfg.BufferSize)
	}
	if e.cfg.ReconnectBackoff != time.Second {
		t.Errorf("ReconnectBackoff default = %v", e.cfg.ReconnectBackoff)
	}
	if e.cfg.MaxReconnectBackoff != 30*time.Second {
		t.Errorf("MaxReconnectBackoff default = %v", e.cfg.MaxReconnectBackoff)
	}
}

// --- end-to-end --------------------------------------------------------------

func TestEmitter_EndToEnd_PublishesEvents(t *testing.T) {
	keyPath, clientPub := keyPair(t)
	host, port, listener, cancelServer := startShell(t, clientPub)
	defer cancelServer()

	e, err := NewStatusEmitter(Config{
		ShellHost:        host,
		ShellPort:        port,
		User:             "claude",
		PrivateKeyPath:   keyPath,
		AgentID:          "agent-e2e",
		ReconnectBackoff: 10 * time.Millisecond,
		Logger:           silentLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go e.Run(ctx)

	// Wait for the opportunistic agent.started marker to land first.
	if !waitFor(t, 2*time.Second, func() bool {
		return len(listener.snapshot()) >= 1
	}) {
		t.Fatal("agent.started never arrived")
	}

	// Now publish some real events.
	e.Publish(statusproto.NewEvent(statusproto.TypeContainerCreated, "agent-e2e", "sess-1"))
	e.Publish(statusproto.NewEvent(statusproto.TypeContainerStopped, "agent-e2e", "sess-1"))

	if !waitFor(t, 2*time.Second, func() bool {
		return len(listener.snapshot()) >= 3
	}) {
		t.Fatalf("expected 3+ events, got %d", len(listener.snapshot()))
	}

	cancel()
	// Allow shutdown event to propagate.
	time.Sleep(100 * time.Millisecond)

	types := map[string]bool{}
	for _, ev := range listener.snapshot() {
		types[ev.Type] = true
	}
	for _, want := range []string{
		statusproto.TypeAgentStarted,
		statusproto.TypeContainerCreated,
		statusproto.TypeContainerStopped,
	} {
		if !types[want] {
			t.Errorf("missing event type %q in %v", want, types)
		}
	}
}

func TestEmitter_StampsAgentIDAndTimestamp(t *testing.T) {
	keyPath, clientPub := keyPair(t)
	host, port, listener, cancelServer := startShell(t, clientPub)
	defer cancelServer()

	e, err := NewStatusEmitter(Config{
		ShellHost:        host,
		ShellPort:        port,
		PrivateKeyPath:   keyPath,
		AgentID:          "agent-z",
		ReconnectBackoff: 10 * time.Millisecond,
		Logger:           silentLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	// Hand-rolled event with blank AgentID and zero Timestamp — the emitter
	// must fill both in before the wire.
	e.Publish(statusproto.Event{Type: statusproto.TypeContainerEdited, SessionID: "s1"})

	if !waitFor(t, 2*time.Second, func() bool {
		for _, ev := range listener.snapshot() {
			if ev.Type == statusproto.TypeContainerEdited {
				return true
			}
		}
		return false
	}) {
		t.Fatal("edited event never arrived")
	}
	var got statusproto.Event
	for _, ev := range listener.snapshot() {
		if ev.Type == statusproto.TypeContainerEdited {
			got = ev
			break
		}
	}
	if got.AgentID != "agent-z" {
		t.Errorf("AgentID = %q, want agent-z", got.AgentID)
	}
	if got.Timestamp.IsZero() {
		t.Error("Timestamp was not stamped")
	}
}

func TestEmitter_Heartbeat(t *testing.T) {
	keyPath, clientPub := keyPair(t)
	host, port, listener, cancelServer := startShell(t, clientPub)
	defer cancelServer()

	e, err := NewStatusEmitter(Config{
		ShellHost:         host,
		ShellPort:         port,
		PrivateKeyPath:    keyPath,
		AgentID:           "agent-hb",
		HeartbeatInterval: 60 * time.Millisecond,
		ReconnectBackoff:  10 * time.Millisecond,
		Logger:            silentLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	// Expect at least two heartbeats within a reasonable window.
	ok := waitFor(t, 2*time.Second, func() bool {
		count := 0
		for _, ev := range listener.snapshot() {
			if ev.Type == statusproto.TypeAgentHeartbeat {
				count++
			}
		}
		return count >= 2
	})
	if !ok {
		t.Fatalf("expected >=2 heartbeats; events=%v", listener.snapshot())
	}
}

// --- unit checks -----------------------------------------------------------

func TestPublish_DropsWhenBufferFull(t *testing.T) {
	keyPath, _ := keyPair(t)
	e, err := NewStatusEmitter(Config{
		ShellHost:      "never-dialed",
		AgentID:        "a",
		PrivateKeyPath: keyPath,
		BufferSize:     1,
		Logger:         silentLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Nothing is draining the queue — after the first Publish the buffer is
	// full and subsequent Publishes drop silently. Must not block.
	done := make(chan struct{})
	go func() {
		e.Publish(statusproto.NewEvent(statusproto.TypeContainerCreated, "a", ""))
		e.Publish(statusproto.NewEvent(statusproto.TypeContainerCreated, "a", ""))
		e.Publish(statusproto.NewEvent(statusproto.TypeContainerCreated, "a", ""))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked when buffer full")
	}
	if got := len(e.queue); got != 1 {
		t.Errorf("queue len = %d, want 1 (buffer cap)", got)
	}
}

func TestPublish_AfterCloseIsNoop(t *testing.T) {
	keyPath, _ := keyPair(t)
	e, err := NewStatusEmitter(Config{
		ShellHost:      "h",
		AgentID:        "a",
		PrivateKeyPath: keyPath,
		Logger:         silentLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	e.closed.Store(true)
	// Close() would wait on Run's wg — we only care about the "closed gate"
	// in Publish here, so flip the atomic directly.
	e.Publish(statusproto.NewEvent(statusproto.TypeAgentHeartbeat, "a", ""))
	if len(e.queue) != 0 {
		t.Errorf("expected Publish after close to be a no-op, queue len = %d", len(e.queue))
	}
}

func TestNextBackoff(t *testing.T) {
	max := 8 * time.Second
	cases := []struct{ in, want time.Duration }{
		{time.Second, 2 * time.Second},
		{2 * time.Second, 4 * time.Second},
		{4 * time.Second, 8 * time.Second},
		{8 * time.Second, 8 * time.Second}, // capped
		{16 * time.Second, 8 * time.Second},
	}
	for _, c := range cases {
		if got := nextBackoff(c.in, max); got != c.want {
			t.Errorf("nextBackoff(%v, %v) = %v, want %v", c.in, max, got, c.want)
		}
	}
}

func TestLoadPrivateKey_Errors(t *testing.T) {
	if _, err := loadPrivateKey(""); err == nil {
		t.Error("expected error for empty path")
	}
	if _, err := loadPrivateKey("/does/not/exist/ever"); err == nil {
		t.Error("expected error for missing file")
	}
	// Garbage bytes in a real path — parse should fail.
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad")
	if err := os.WriteFile(bad, []byte("not a key"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPrivateKey(bad); err == nil {
		t.Error("expected parse error")
	}
}

func TestEmitter_Close_StopsPublishingAndIsIdempotentOnQueue(t *testing.T) {
	// Build an emitter without ever calling Run — no goroutines, so wg
	// is empty and Close returns immediately.
	keyPath, _ := keyPair(t)
	e, err := NewStatusEmitter(Config{
		ShellHost:      "127.0.0.1",
		ShellPort:      65535, // unreachable; we never connect
		AgentID:        "agent-x",
		PrivateKeyPath: keyPath,
		BufferSize:     4,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pre-Close, Publish enqueues normally.
	e.Publish(statusproto.Event{Type: "before"})

	e.Close()
	if !e.closed.Load() {
		t.Error("Close did not flip closed flag")
	}

	// Post-Close, Publish becomes a no-op (early-return). If it tried to
	// send on the closed channel it would panic — the absence of a panic
	// is the assertion.
	e.Publish(statusproto.Event{Type: "after"})
}
