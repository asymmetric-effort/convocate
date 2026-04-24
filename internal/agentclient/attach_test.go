package agentclient

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/asymmetric-effort/claude-shell/internal/agentserver"
	"github.com/asymmetric-effort/claude-shell/internal/sshutil"
)

// pipeAttachTarget is an AttachTarget that uses io.Pipe to relay between
// the test's "container" side and the SSH channel. Real pipes block
// properly on empty reads, so io.Copy doesn't spin and the test runs
// reliably under load.
type pipeAttachTarget struct {
	receivedID string

	// Bytes the server writes end up readable through the rwc (master
	// side of HandleAttach). clientSink collects what the client sent
	// to us via the same rwc's Write.
	serverOut  *io.PipeWriter // server writes here — propagates to rwc.Read
	serverOutR *io.PipeReader
	clientSink *bytes.Buffer
	sinkMu     sync.Mutex

	initCols, initRows     uint16
	resizeCols, resizeRows uint16

	closeOnce sync.Once
	done      chan struct{}
}

func newPipeAttachTarget() *pipeAttachTarget {
	r, w := io.Pipe()
	return &pipeAttachTarget{
		serverOut:  w,
		serverOutR: r,
		clientSink: &bytes.Buffer{},
		done:       make(chan struct{}),
	}
}

// writeToClient queues bytes that the server will "produce" — the SSH
// channel pushes them to the client's stdout.
func (p *pipeAttachTarget) writeToClient(s string) {
	_, _ = p.serverOut.Write([]byte(s))
}

// clientSent returns everything the client has sent to the server so far.
func (p *pipeAttachTarget) clientSent() string {
	p.sinkMu.Lock()
	defer p.sinkMu.Unlock()
	return p.clientSink.String()
}

type attachRWC struct {
	reads  *io.PipeReader
	writes *bytes.Buffer
	mu     *sync.Mutex
	closed chan struct{}
}

func (t *attachRWC) Read(p []byte) (int, error) { return t.reads.Read(p) }

func (t *attachRWC) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.writes.Write(p)
}

func (t *attachRWC) Close() error {
	select {
	case <-t.closed:
	default:
		close(t.closed)
		_ = t.reads.Close()
	}
	return nil
}

func (p *pipeAttachTarget) Start(_ context.Context, sessionID string, cols, rows uint16) (io.ReadWriteCloser, func(uint16, uint16), func() error, func(), error) {
	p.receivedID = sessionID
	p.initCols = cols
	p.initRows = rows
	rwc := &attachRWC{
		reads:  p.serverOutR,
		writes: p.clientSink,
		mu:     &p.sinkMu,
		closed: make(chan struct{}),
	}
	resize := func(c, r uint16) { p.resizeCols = c; p.resizeRows = r }
	wait := func() error {
		<-p.done
		return nil
	}
	kill := func() {
		p.closeOnce.Do(func() {
			close(p.done)
			_ = p.serverOut.Close()
		})
	}
	return rwc, resize, wait, kill, nil
}

// spinUpAgentWithAttach boots an agentserver that serves both the RPC
// subsystem and our pipe-based attach subsystem. Returns the server addr,
// client private key path, the pipe target, and a cancel func.
func spinUpAgentWithAttach(t *testing.T, target agentserver.AttachTarget) (addr, keyPath string, cancel context.CancelFunc) {
	t.Helper()
	dir := t.TempDir()
	hostKeyPath := filepath.Join(dir, "host_key")
	authPath := filepath.Join(dir, "authorized_keys")
	keyPath = filepath.Join(dir, "client_key")

	clientSigner, err := sshutil.LoadOrCreateHostKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authPath, ssh.MarshalAuthorizedKey(clientSigner.PublicKey()), 0600); err != nil {
		t.Fatal(err)
	}

	d := agentserver.NewDispatcher()
	agentserver.RegisterCoreOps(d, "agent-test", "v0")

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr = ln.Addr().String()
	ln.Close()

	srv, err := agentserver.New(agentserver.Config{
		HostKeyPath:        hostKeyPath,
		AuthorizedKeysPath: authPath,
		Listen:             addr,
		Dispatcher:         d,
		AttachTarget:       target,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, c := context.WithCancel(context.Background())
	cancel = c
	go func() { _ = srv.Serve(ctx) }()
	time.Sleep(50 * time.Millisecond)
	return addr, keyPath, cancel
}

func TestAttach_ServerReceivesHeaderAndData(t *testing.T) {
	target := newPipeAttachTarget()
	// Driver goroutine:
	//   1. Emit the server→client bytes.
	//   2. Poll clientSent for "echo one" so we only tear down the
	//      attach after the server-side stdin copy has observed the
	//      payload. This eliminates a race where closing target.done
	//      too early makes the server close its master before the
	//      client's stdin made it through.
	//   3. Close the pipe + done so the attach unblocks cleanly.
	go func() {
		target.writeToClient("hello from container\n")
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if strings.Contains(target.clientSent(), "echo one") {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		target.closeOnce.Do(func() {
			close(target.done)
			_ = target.serverOut.Close()
		})
	}()

	addr, keyPath, cancel := spinUpAgentWithAttach(t, target)
	defer cancel()

	host, port := splitHostPort(t, addr)
	c, err := NewCRUDClient(CRUDConfig{AgentHost: host, AgentPort: port, PrivateKeyPath: keyPath})
	if err != nil {
		t.Fatalf("NewCRUDClient: %v", err)
	}
	defer c.Close()

	var got bytes.Buffer
	clientIn := strings.NewReader("echo one\n")
	err = Attach(c.SSHClient(), AttachOptions{
		SessionID: "session-abc",
		Stdin:     clientIn,
		Stdout:    &got,
		Cols:      120, Rows: 40,
	})
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}

	if target.receivedID != "session-abc" {
		t.Errorf("server got sessionID = %q, want 'session-abc'", target.receivedID)
	}
	if target.initCols != 120 || target.initRows != 40 {
		t.Errorf("server got size = %dx%d, want 120x40", target.initCols, target.initRows)
	}
	if !strings.Contains(got.String(), "hello from container") {
		t.Errorf("client stdout missing server output: %q", got.String())
	}
	if !strings.Contains(target.clientSent(), "echo one") {
		t.Errorf("server didn't receive client stdin: %q", target.clientSent())
	}
}

func TestAttach_RequiresSessionID(t *testing.T) {
	err := Attach(nil, AttachOptions{Stdin: strings.NewReader(""), Stdout: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "SessionID") {
		t.Errorf("expected SessionID-required error, got %v", err)
	}
}

func TestAttach_RequiresIOStreams(t *testing.T) {
	err := Attach(nil, AttachOptions{SessionID: "x"})
	if err == nil || !strings.Contains(err.Error(), "Stdin") {
		t.Errorf("expected streams-required error, got %v", err)
	}
}
