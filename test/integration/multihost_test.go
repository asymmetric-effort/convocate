//go:build integration

// Multihost integration: stand up an in-process convocate-agent
// (agentserver + StatusEmitter) and an in-process convocate
// (shellserver + CRUDClient), wire them together with ed25519 keys, and
// verify that:
//
//  1. CRUD ops fired by the shell against the agent take effect through
//     the real SSH plumbing.
//  2. Status events emitted by the agent's orchestrator land on the
//     shell's listener via the persistent emitter.
//
// This stitches the per-package tests into one harness so a regression in
// the wire layer (subsystem mismatch, header framing, key auth) shows up
// here even when each side passes in isolation.
package integration

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/asymmetric-effort/convocate/internal/agentclient"
	"github.com/asymmetric-effort/convocate/internal/agentserver"
	"github.com/asymmetric-effort/convocate/internal/config"
	"github.com/asymmetric-effort/convocate/internal/container"
	"github.com/asymmetric-effort/convocate/internal/session"
	"github.com/asymmetric-effort/convocate/internal/shellserver"
	"github.com/asymmetric-effort/convocate/internal/sshutil"
	"github.com/asymmetric-effort/convocate/internal/statusproto"
	"github.com/asymmetric-effort/convocate/internal/user"
)

// captureListener records every status event the shell receives.
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

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// fakeExec returns docker stand-ins so Restart can run without a real
// daemon: inspect → "false" (not running), run → success.
func fakeExec(name string, args ...string) *exec.Cmd {
	for _, a := range args {
		if a == "inspect" {
			return exec.Command("echo", "false")
		}
	}
	return exec.Command("true")
}

// TestMultihost_EndToEnd boots both servers and a status emitter + CRUD
// client in one process, fires a CRUD op, and asserts both the op took
// effect on the agent's session manager AND the corresponding status
// event reached the shell's listener.
func TestMultihost_EndToEnd(t *testing.T) {
	dir := t.TempDir()

	// ---------- ed25519 keypair: shell ↔ agent CRUD ----------
	shellToAgentKey := filepath.Join(dir, "shell_to_agent")
	shellSigner, err := sshutil.LoadOrCreateHostKey(shellToAgentKey)
	if err != nil {
		t.Fatal(err)
	}

	// ---------- ed25519 keypair: agent ↔ shell status ----------
	agentToShellKey := filepath.Join(dir, "agent_to_shell")
	agentSigner, err := sshutil.LoadOrCreateHostKey(agentToShellKey)
	if err != nil {
		t.Fatal(err)
	}

	// ---------- agent server config ----------
	agentHostKey := filepath.Join(dir, "agent_host_key")
	agentAuth := filepath.Join(dir, "agent_authorized_keys")
	if err := os.WriteFile(agentAuth, ssh.MarshalAuthorizedKey(shellSigner.PublicKey()), 0600); err != nil {
		t.Fatal(err)
	}

	// Real session.Manager + production SessionOrchestrator. NewRunner
	// is replaced with a fake exec so Restart works without docker.
	sessionsBase := filepath.Join(dir, "sessions")
	skelDir := filepath.Join(dir, "skel")
	if err := os.MkdirAll(skelDir, 0750); err != nil {
		t.Fatal(err)
	}
	mgr := session.NewManager(sessionsBase, skelDir)
	uinfo := user.Info{UID: 1337, GID: 1337, Username: "convocate", HomeDir: "/home/convocate"}
	paths := config.Paths{ConvocateHome: "/home/convocate", SessionsBase: sessionsBase, SkelDir: skelDir}

	// We give the orchestrator a Publisher we'll wire to the emitter
	// in a moment.
	emitterReady := make(chan struct{})
	var emitter *agentclient.StatusEmitter
	publisher := publisherFn(func(ev statusproto.Event) {
		<-emitterReady
		emitter.Publish(ev)
	})

	orch := agentserver.NewSessionOrchestrator(mgr, uinfo, paths, "", "agent-it", publisher)
	orch.NewRunner = func(id, sdir string, u user.Info, p config.Paths) *container.Runner {
		return container.NewRunnerWithExec(id, sdir, u, p, fakeExec)
	}

	d := agentserver.NewDispatcher()
	agentserver.RegisterCoreOps(d, "agent-it", "v-test")
	agentserver.RegisterCRUDOps(d, orch)

	agentAddr := freeAddr(t)
	agentSrv, err := agentserver.New(agentserver.Config{
		HostKeyPath:        agentHostKey,
		AuthorizedKeysPath: agentAuth,
		Listen:             agentAddr,
		Dispatcher:         d,
	})
	if err != nil {
		t.Fatal(err)
	}

	// ---------- shell server config ----------
	shellHostKey := filepath.Join(dir, "shell_host_key")
	shellAuth := filepath.Join(dir, "shell_authorized_keys")
	if err := os.WriteFile(shellAuth, ssh.MarshalAuthorizedKey(agentSigner.PublicKey()), 0600); err != nil {
		t.Fatal(err)
	}
	listener := &captureListener{}
	shellAddr := freeAddr(t)
	shellSrv, err := shellserver.New(shellserver.Config{
		HostKeyPath:        shellHostKey,
		AuthorizedKeysPath: shellAuth,
		Listen:             shellAddr,
		Listener:           listener,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = agentSrv.Serve(ctx) }()
	go func() { _ = shellSrv.Serve(ctx) }()
	time.Sleep(80 * time.Millisecond) // both listeners bind

	// ---------- agent's StatusEmitter dialing the shell ----------
	shellHost, shellPort := splitHostPort(t, shellAddr)
	emitter, err = agentclient.NewStatusEmitter(agentclient.Config{
		ShellHost:           shellHost,
		ShellPort:           shellPort,
		PrivateKeyPath:      agentToShellKey,
		AgentID:             "agent-it",
		ReconnectBackoff:    20 * time.Millisecond,
		MaxReconnectBackoff: 80 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	emitCtx, emitCancel := context.WithCancel(ctx)
	defer emitCancel()
	go emitter.Run(emitCtx)
	close(emitterReady) // unblock publisher

	// ---------- shell's CRUD client dialing the agent ----------
	agentHost, agentPort := splitHostPort(t, agentAddr)
	cli, err := agentclient.NewCRUDClient(agentclient.CRUDConfig{
		AgentHost:      agentHost,
		AgentPort:      agentPort,
		PrivateKeyPath: shellToAgentKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	// ---------- exercise: ping ----------
	if _, err := cli.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	// ---------- exercise: Create → Restart → Delete with status flow ----------
	created, err := cli.Create(agentserver.CreateRequest{Name: "svc", Port: 9090, Protocol: "tcp"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Name != "svc" || created.Port != 9090 {
		t.Errorf("create returned wrong meta: %+v", created)
	}

	if err := cli.Restart(created.UUID); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if err := cli.Delete(created.UUID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// ---------- assert: events arrived at shell ----------
	if !waitFor(2*time.Second, func() bool {
		seen := map[string]bool{}
		for _, ev := range listener.snapshot() {
			seen[ev.Type] = true
		}
		return seen[statusproto.TypeContainerCreated] &&
			seen[statusproto.TypeContainerStarted] &&
			seen[statusproto.TypeContainerDeleted]
	}) {
		t.Errorf("expected created+started+deleted events at shell, got %+v", listener.snapshot())
	}

	// Each delivered event should carry the agent's ID — the routing
	// stamp the shell uses to pick a per-agent log destination.
	for _, ev := range listener.snapshot() {
		if ev.AgentID != "agent-it" {
			t.Errorf("event %s has AgentID=%q, want agent-it", ev.Type, ev.AgentID)
		}
	}
}

// TestMultihost_AttachThroughSSH boots an agent with a stub AttachTarget
// and exercises the full convocate-agent-attach handshake from agentclient.
// This wires HandleAttach (server) to Attach (client) over real SSH.
func TestMultihost_AttachThroughSSH(t *testing.T) {
	dir := t.TempDir()

	clientKey := filepath.Join(dir, "ck")
	signer, err := sshutil.LoadOrCreateHostKey(clientKey)
	if err != nil {
		t.Fatal(err)
	}
	hostKey := filepath.Join(dir, "hk")
	authPath := filepath.Join(dir, "auth")
	if err := os.WriteFile(authPath, ssh.MarshalAuthorizedKey(signer.PublicKey()), 0600); err != nil {
		t.Fatal(err)
	}

	target := newPipeTarget()

	d := agentserver.NewDispatcher()
	agentserver.RegisterCoreOps(d, "a", "v")
	addr := freeAddr(t)
	srv, err := agentserver.New(agentserver.Config{
		HostKeyPath:        hostKey,
		AuthorizedKeysPath: authPath,
		Listen:             addr,
		Dispatcher:         d,
		AttachTarget:       target,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Serve(ctx) }()
	time.Sleep(50 * time.Millisecond)

	host, port := splitHostPort(t, addr)
	cli, err := agentclient.NewCRUDClient(agentclient.CRUDConfig{
		AgentHost: host, AgentPort: port, PrivateKeyPath: clientKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	// Server pushes a fixed greeting, then EOFs, which triggers the
	// client's stdout pump to finish and Attach to return.
	go func() {
		target.queue("welcome from container\n")
		time.Sleep(40 * time.Millisecond)
		target.finish()
	}()

	var buf strings.Builder
	err = agentclient.Attach(cli.SSHClient(), agentclient.AttachOptions{
		SessionID: "sess-1",
		Stdin:     strings.NewReader(""),
		Stdout:    &buf,
		Cols:      100, Rows: 30,
	})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if !strings.Contains(buf.String(), "welcome from container") {
		t.Errorf("client never saw server output: %q", buf.String())
	}
	if target.gotID != "sess-1" {
		t.Errorf("server got sessionID = %q, want sess-1", target.gotID)
	}
	if target.gotCols != 100 || target.gotRows != 30 {
		t.Errorf("server got size = %dx%d, want 100x30", target.gotCols, target.gotRows)
	}
}

// --- helpers ----------------------------------------------------------------

func splitHostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	var p int
	for _, c := range portStr {
		if c < '0' || c > '9' {
			t.Fatalf("bad port: %q", portStr)
		}
		p = p*10 + int(c-'0')
	}
	return host, p
}

func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}

// publisherFn adapts a function literal to the StatusPublisher interface
// expected by SessionOrchestrator.
type publisherFn func(statusproto.Event)

func (f publisherFn) Publish(ev statusproto.Event) { f(ev) }

// pipeTarget is an AttachTarget that bridges in-memory pipes — no docker.
// The pipe is created up front so queue() can write before Start() is
// invoked from the server goroutine.
type pipeTarget struct {
	mu               sync.Mutex
	gotID            string
	gotCols, gotRows uint16

	rw     *pipeReadWriter
	doneCh chan struct{}
	once   sync.Once
}

func newPipeTarget() *pipeTarget {
	return &pipeTarget{doneCh: make(chan struct{}), rw: newPipeReadWriter()}
}

func (p *pipeTarget) queue(s string) {
	_, _ = p.rw.Write([]byte(s))
}

func (p *pipeTarget) finish() {
	p.once.Do(func() { close(p.doneCh); _ = p.rw.Close() })
}

func (p *pipeTarget) Start(_ context.Context, sessionID string, cols, rows uint16) (io.ReadWriteCloser, func(uint16, uint16), func() error, func(), error) {
	p.mu.Lock()
	p.gotID = sessionID
	p.gotCols = cols
	p.gotRows = rows
	p.mu.Unlock()
	resize := func(c, r uint16) { p.gotCols = c; p.gotRows = r }
	wait := func() error { <-p.doneCh; return nil }
	kill := func() { p.finish() }
	return p.rw, resize, wait, kill, nil
}

type pipeReadWriter struct {
	mu    sync.Mutex
	cond  *sync.Cond
	buf   []byte
	close bool
}

func newPipeReadWriter() *pipeReadWriter {
	p := &pipeReadWriter{}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *pipeReadWriter) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(p.buf) == 0 && !p.close {
		p.cond.Wait()
	}
	if len(p.buf) == 0 && p.close {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(b, p.buf)
	p.buf = p.buf[n:]
	return n, nil
}

func (p *pipeReadWriter) Write(b []byte) (int, error) {
	p.mu.Lock()
	p.buf = append(p.buf, b...)
	p.cond.Broadcast()
	p.mu.Unlock()
	return len(b), nil
}

func (p *pipeReadWriter) Close() error {
	p.mu.Lock()
	p.close = true
	p.cond.Broadcast()
	p.mu.Unlock()
	return nil
}
