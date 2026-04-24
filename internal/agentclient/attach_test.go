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

// pipeAttachTarget is an AttachTarget that relays to in-memory pipes
// instead of launching docker+pty. Tests can drive the remote end
// directly and read back what the client wrote / wrote back.
type pipeAttachTarget struct {
	// received* collect what the "container" end observes; reply is
	// what the container writes back to the client.
	receivedID string

	// serverToClient is what the remote end writes; clientToServer is
	// what the remote end reads.
	serverToClient *bytes.Buffer
	clientToServer *bytes.Buffer

	// cols/rows capture the initial size + latest resize.
	initCols, initRows uint16
	resizeCols, resizeRows uint16

	closeOnce sync.Once
	done      chan struct{}
}

func newPipeAttachTarget() *pipeAttachTarget {
	return &pipeAttachTarget{
		serverToClient: &bytes.Buffer{},
		clientToServer: &bytes.Buffer{},
		done:           make(chan struct{}),
	}
}

type testRWC struct {
	readFrom io.Reader
	writeTo  io.Writer
	closed   chan struct{}
}

func (t *testRWC) Read(p []byte) (int, error) {
	select {
	case <-t.closed:
		return 0, io.EOF
	default:
	}
	// Block-waiting read is inconvenient for our test purposes; we use
	// a reader that has bytes already queued. When empty, return EOF so
	// io.Copy unblocks — the test fills the buffer before calling attach.
	n, err := t.readFrom.Read(p)
	if err == io.EOF {
		select {
		case <-t.closed:
			return 0, io.EOF
		case <-time.After(10 * time.Millisecond):
			return 0, nil // busy-ish; io.Copy retries
		}
	}
	return n, err
}

func (t *testRWC) Write(p []byte) (int, error) {
	return t.writeTo.Write(p)
}

func (t *testRWC) Close() error {
	select {
	case <-t.closed:
	default:
		close(t.closed)
	}
	return nil
}

func (p *pipeAttachTarget) Start(ctx context.Context, sessionID string, cols, rows uint16) (io.ReadWriteCloser, func(uint16, uint16), func() error, func(), error) {
	p.receivedID = sessionID
	p.initCols = cols
	p.initRows = rows

	rwc := &testRWC{
		readFrom: p.serverToClient, // what the server writes, the client reads
		writeTo:  p.clientToServer, // what the client writes, the server records
		closed:   make(chan struct{}),
	}
	resize := func(c, r uint16) { p.resizeCols = c; p.resizeRows = r }
	wait := func() error {
		<-p.done
		return nil
	}
	kill := func() {
		p.closeOnce.Do(func() { close(p.done) })
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
	// Pre-load bytes the server will "send" to the client.
	target.serverToClient.WriteString("hello from container\n")
	// Schedule the target to finish after a short time so attach unblocks.
	go func() {
		time.Sleep(150 * time.Millisecond)
		target.closeOnce.Do(func() { close(target.done) })
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
	if !strings.Contains(target.clientToServer.String(), "echo one") {
		t.Errorf("server didn't receive client stdin: %q", target.clientToServer.String())
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
