// Package agentclient is the claude-agent-side half of the
// agent↔shell control plane. It holds a persistent SSH connection to the
// shell's status listener and publishes statusproto.Event values over the
// claude-shell-status subsystem.
//
// The emitter is designed to be non-blocking from the op-handler's point of
// view: Publish drops events on the floor when the buffer is full rather
// than stalling a CRUD op. This is the right trade for a status channel —
// missed updates are recovered on the next real event or heartbeat.
package agentclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/asymmetric-effort/claude-shell/internal/statusproto"
)

// Config configures a StatusEmitter.
type Config struct {
	// ShellHost is the hostname/IP of the claude-shell host's status
	// listener. Port defaults to 222 if ShellPort is 0.
	ShellHost string
	ShellPort int

	// User is the SSH username used when dialing the shell. Typically
	// "claude".
	User string

	// PrivateKeyPath is the path to the agent→shell SSH private key (paired
	// with the public key init-agent installed in the shell's
	// status_authorized_keys).
	PrivateKeyPath string

	// AgentID is stamped onto every event the emitter publishes.
	AgentID string

	// HeartbeatInterval controls how often agent.heartbeat events are
	// emitted. Zero disables the heartbeat.
	HeartbeatInterval time.Duration

	// ReconnectBackoff is the initial delay between reconnect attempts
	// when the shell is unreachable. The emitter doubles it up to
	// MaxReconnectBackoff.
	ReconnectBackoff    time.Duration
	MaxReconnectBackoff time.Duration

	// BufferSize is the in-memory queue depth. Publishes beyond this
	// bound are dropped. Zero defaults to 256.
	BufferSize int

	// Logger receives diagnostic lines. Nil = standard logger.
	Logger *log.Logger
}

// StatusEmitter publishes Events to the shell host over a persistent SSH
// connection. Safe to call Publish concurrently from any number of op
// handlers; a single background goroutine drains the queue and writes to
// the wire.
type StatusEmitter struct {
	cfg    Config
	queue  chan statusproto.Event
	closed atomic.Bool

	// signer is cached after the first successful key load — we only re-load
	// it on a fresh-reconnect if a Reload() were added.
	signer ssh.Signer

	wg sync.WaitGroup
}

// NewStatusEmitter constructs a StatusEmitter with sane defaults. The
// configured private key is loaded eagerly so a misconfiguration surfaces
// at startup rather than silently after the first event.
func NewStatusEmitter(cfg Config) (*StatusEmitter, error) {
	if cfg.ShellHost == "" {
		return nil, fmt.Errorf("agentclient: ShellHost is required")
	}
	if cfg.ShellPort == 0 {
		cfg.ShellPort = 222
	}
	if cfg.User == "" {
		cfg.User = "claude"
	}
	if cfg.AgentID == "" {
		return nil, fmt.Errorf("agentclient: AgentID is required")
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 256
	}
	if cfg.ReconnectBackoff <= 0 {
		cfg.ReconnectBackoff = time.Second
	}
	if cfg.MaxReconnectBackoff <= 0 {
		cfg.MaxReconnectBackoff = 30 * time.Second
	}
	if cfg.HeartbeatInterval < 0 {
		cfg.HeartbeatInterval = 0
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}

	signer, err := loadPrivateKey(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load agent private key: %w", err)
	}

	return &StatusEmitter{
		cfg:    cfg,
		queue:  make(chan statusproto.Event, cfg.BufferSize),
		signer: signer,
	}, nil
}

// Run drives the persistent connection until ctx is canceled. Under steady
// state it sits in a tight loop: dial → push events → on error reconnect
// after backoff. Returns when ctx is done.
func (e *StatusEmitter) Run(ctx context.Context) {
	e.wg.Add(1)
	defer e.wg.Done()

	// Heartbeat goroutine fires ticks into the event queue at the configured
	// cadence. Kept simple: we enqueue alongside regular events and the
	// downstream writer handles ordering.
	if e.cfg.HeartbeatInterval > 0 {
		hbCtx, hbCancel := context.WithCancel(ctx)
		defer hbCancel()
		go e.runHeartbeat(hbCtx)
	}

	backoff := e.cfg.ReconnectBackoff
	for {
		if ctx.Err() != nil {
			return
		}
		if err := e.sessionOnce(ctx); err != nil {
			e.cfg.Logger.Printf("claude-agent: status session ended: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = nextBackoff(backoff, e.cfg.MaxReconnectBackoff)
		} else {
			// Clean exit (ctx cancel) — no need to back off.
			backoff = e.cfg.ReconnectBackoff
		}
	}
}

// Publish enqueues ev for eventual delivery. Never blocks: if the buffer is
// full, the event is dropped and logged. This is deliberate — a stalled
// status channel must not back-pressure a container CRUD op.
func (e *StatusEmitter) Publish(ev statusproto.Event) {
	if e.closed.Load() {
		return
	}
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now().UTC()
	}
	if ev.AgentID == "" {
		ev.AgentID = e.cfg.AgentID
	}
	select {
	case e.queue <- ev:
	default:
		e.cfg.Logger.Printf("claude-agent: status queue full, dropped event %s", ev.Type)
	}
}

// Close marks the emitter as closed and waits for Run to exit. Callers
// should cancel the Run context first.
func (e *StatusEmitter) Close() {
	e.closed.Store(true)
	close(e.queue)
	e.wg.Wait()
}

// runHeartbeat emits an agent.heartbeat event at the configured cadence
// until ctx is canceled.
func (e *StatusEmitter) runHeartbeat(ctx context.Context) {
	t := time.NewTicker(e.cfg.HeartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.Publish(statusproto.NewEvent(statusproto.TypeAgentHeartbeat, e.cfg.AgentID, ""))
		}
	}
}

// sessionOnce dials the shell, requests the status subsystem, and pumps
// queued events onto the wire until either the context is canceled or the
// remote closes the channel.
func (e *StatusEmitter) sessionOnce(ctx context.Context) error {
	addr := net.JoinHostPort(e.cfg.ShellHost, fmt.Sprintf("%d", e.cfg.ShellPort))
	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            e.cfg.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(e.signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	stdin, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	if err := sess.RequestSubsystem(statusproto.Subsystem); err != nil {
		return fmt.Errorf("request subsystem: %w", err)
	}

	e.cfg.Logger.Printf("claude-agent: status connected to %s", addr)

	// Opportunistic "I'm here" marker — lets the shell log the connection
	// alongside the authenticated fingerprint.
	_ = e.writeOne(stdin, statusproto.NewEvent(statusproto.TypeAgentStarted, e.cfg.AgentID, ""))

	for {
		select {
		case <-ctx.Done():
			_ = e.writeOne(stdin, statusproto.NewEvent(statusproto.TypeAgentShutdown, e.cfg.AgentID, ""))
			return nil
		case ev, ok := <-e.queue:
			if !ok {
				return nil
			}
			if err := e.writeOne(stdin, ev); err != nil {
				return fmt.Errorf("write event: %w", err)
			}
		}
	}
}

// writeOne marshals ev to JSON and writes one newline-terminated line to w.
func (e *StatusEmitter) writeOne(w io.Writer, ev statusproto.Event) error {
	buf := bufio.NewWriter(w)
	if err := json.NewEncoder(buf).Encode(ev); err != nil {
		return err
	}
	return buf.Flush()
}

// nextBackoff doubles b capped at max. Pulled out so tests can exercise the
// math without fighting the ctx/timer plumbing.
func nextBackoff(b, max time.Duration) time.Duration {
	b *= 2
	if b > max {
		b = max
	}
	return b
}

// loadPrivateKey reads an ed25519 (or RSA) SSH private key from path and
// returns an ssh.Signer ready for a ClientConfig.
func loadPrivateKey(path string) (ssh.Signer, error) {
	if path == "" {
		return nil, fmt.Errorf("no private key path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return signer, nil
}
