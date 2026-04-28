package agentclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/asymmetric-effort/convocate/internal/agentserver"
	"github.com/asymmetric-effort/convocate/internal/session"
)

// CRUDConfig configures a CRUDClient.
type CRUDConfig struct {
	// AgentHost is the hostname / IP of the convocate-agent listener.
	AgentHost string
	// AgentPort defaults to 222 when zero.
	AgentPort int
	// User is the SSH username. Defaults to "convocate".
	User string
	// PrivateKeyPath is the shell→agent SSH private key, typically
	// /etc/convocate/agent-keys/<id>/shell_to_agent_ed25519_key.
	PrivateKeyPath string
	// DialTimeout caps the initial TCP+SSH handshake. Defaults to 10s.
	DialTimeout time.Duration

	// HeartbeatInterval controls how often the client pings the agent to
	// prove the SSH connection is alive. Zero disables the heartbeat
	// (useful for tests and short-lived scripts). Typical production
	// value is 30s, matching the agent's own status heartbeat so an
	// asymmetric failure is visible on one side within a single cycle.
	HeartbeatInterval time.Duration

	// ReconnectBackoff is the initial delay between reconnect attempts
	// after a heartbeat failure. Doubles up to MaxReconnectBackoff.
	ReconnectBackoff    time.Duration
	MaxReconnectBackoff time.Duration

	// Logger receives diagnostic lines. Nil = standard logger.
	Logger *log.Logger
}

// CRUDClient wraps an SSH connection to an agent and exposes one method per
// op the agent implements. A client is safe for concurrent use — each Call
// opens a fresh session channel over the shared connection.
//
// When HeartbeatInterval > 0, a background goroutine fires a ping op at
// the configured cadence. If the ping fails, the client marks itself
// unhealthy and attempts to re-dial with exponential backoff; follow-up
// Call invocations wait briefly for the connection to recover before
// returning an error.
type CRUDClient struct {
	cfg    CRUDConfig
	signer ssh.Signer
	addr   string

	mu      sync.RWMutex
	conn    *ssh.Client
	healthy atomic.Bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewCRUDClient dials the agent in cfg and returns a connected client.
// Callers are responsible for Close().
func NewCRUDClient(cfg CRUDConfig) (*CRUDClient, error) {
	if cfg.AgentHost == "" {
		return nil, fmt.Errorf("agentclient: AgentHost is required")
	}
	if cfg.AgentPort == 0 {
		cfg.AgentPort = 222
	}
	if cfg.User == "" {
		cfg.User = "convocate"
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 10 * time.Second
	}
	if cfg.ReconnectBackoff <= 0 {
		cfg.ReconnectBackoff = time.Second
	}
	if cfg.MaxReconnectBackoff <= 0 {
		cfg.MaxReconnectBackoff = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	signer, err := loadPrivateKey(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load shell->agent key: %w", err)
	}
	addr := net.JoinHostPort(cfg.AgentHost, fmt.Sprintf("%d", cfg.AgentPort))
	conn, err := dialAgent(addr, cfg, signer)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &CRUDClient{
		cfg:    cfg,
		signer: signer,
		addr:   addr,
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}
	c.healthy.Store(true)

	if cfg.HeartbeatInterval > 0 {
		c.wg.Add(1)
		go c.runHeartbeat()
	}
	return c, nil
}

// dialAgent is the raw dial used by NewCRUDClient and the reconnect loop.
// Factored out so both paths share the same auth + host-key settings.
func dialAgent(addr string, cfg CRUDConfig, signer ssh.Signer) (*ssh.Client, error) {
	return ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         cfg.DialTimeout,
	})
}

// Close cancels the heartbeat goroutine, waits for it to exit, and closes
// the underlying SSH connection. Safe to call more than once.
func (c *CRUDClient) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Close()
}

// Healthy reports whether the last heartbeat succeeded (or no heartbeat
// has run yet and the initial dial worked). TUI code reads this to
// render per-agent status without making a probe call of its own.
func (c *CRUDClient) Healthy() bool { return c.healthy.Load() }

// runHeartbeat fires ping at cfg.HeartbeatInterval. On ping failure the
// client is marked unhealthy and the goroutine attempts to re-dial with
// exponential backoff; success flips healthy back on.
func (c *CRUDClient) runHeartbeat() {
	defer c.wg.Done()
	tick := time.NewTicker(c.cfg.HeartbeatInterval)
	defer tick.Stop()
	backoff := c.cfg.ReconnectBackoff
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-tick.C:
			err := c.heartbeatOnce()
			if err == nil {
				if !c.healthy.Load() {
					c.cfg.Logger.Printf("agentclient: heartbeat to %s recovered", c.addr)
				}
				c.healthy.Store(true)
				backoff = c.cfg.ReconnectBackoff
				continue
			}
			c.healthy.Store(false)
			c.cfg.Logger.Printf("agentclient: heartbeat to %s failed: %v; reconnecting", c.addr, err)
			if rerr := c.reconnect(); rerr != nil {
				c.cfg.Logger.Printf("agentclient: reconnect to %s failed: %v (retry in %v)", c.addr, rerr, backoff)
				select {
				case <-c.ctx.Done():
					return
				case <-time.After(backoff):
				}
				backoff = nextBackoff(backoff, c.cfg.MaxReconnectBackoff)
			} else {
				c.cfg.Logger.Printf("agentclient: reconnected to %s", c.addr)
				c.healthy.Store(true)
				backoff = c.cfg.ReconnectBackoff
			}
		}
	}
}

// heartbeatOnce sends a ping; its success is the signal that the SSH
// connection is still good.
func (c *CRUDClient) heartbeatOnce() error {
	_, err := c.Ping()
	return err
}

// reconnect redials and atomically swaps the connection. The old conn is
// closed after the swap so any in-flight Call on it fails quickly rather
// than hanging until TCP timeout.
func (c *CRUDClient) reconnect() error {
	conn, err := dialAgent(c.addr, c.cfg, c.signer)
	if err != nil {
		return err
	}
	c.mu.Lock()
	old := c.conn
	c.conn = conn
	c.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

// Call runs one op against the agent, decoding the result into out (which
// may be nil for ops that return an empty object). Returns an error if the
// agent replied with ok=false or the wire format is malformed.
func (c *CRUDClient) Call(op string, params any, out any) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("agent %s: connection closed", c.addr)
	}
	sess, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	stdin, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	if err := sess.RequestSubsystem(agentserver.RPCSubsystem); err != nil {
		return fmt.Errorf("request subsystem: %w", err)
	}

	req := agentserver.Request{Op: op}
	if params != nil {
		raw, merr := json.Marshal(params)
		if merr != nil {
			return fmt.Errorf("marshal params: %w", merr)
		}
		req.Params = raw
	}
	if err := json.NewEncoder(stdin).Encode(req); err != nil {
		return fmt.Errorf("write request: %w", err)
	}
	// The server reads one request, writes one response, then closes. Closing
	// our stdin here is a hint — not strictly required since the decoder will
	// receive one object and stop — but it matches the server's framing.
	_ = stdin.Close()

	var resp agentserver.Response
	if err := json.NewDecoder(stdout).Decode(&resp); err != nil {
		// Drain any trailing bytes to surface a real server error in logs.
		_, _ = io.Copy(io.Discard, stdout)
		return fmt.Errorf("decode response: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("agent op %q: %s", op, resp.Error)
	}
	if out == nil || len(resp.Result) == 0 {
		return nil
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		return fmt.Errorf("decode result: %w", err)
	}
	return nil
}

// --- op helpers -------------------------------------------------------------
//
// Each helper wraps Call with the right params/result shape. Keeps call
// sites in the TUI free of op-name strings and lets the compiler catch
// typos.

// Ping verifies the agent is reachable. Returns the agent's reported id,
// version, and server time — mirrors agentserver.PingResult.
type PingResponse struct {
	AgentID    string `json:"agent_id"`
	Version    string `json:"version"`
	ServerTime string `json:"server_time"`
}

func (c *CRUDClient) Ping() (PingResponse, error) {
	var out PingResponse
	if err := c.Call("ping", nil, &out); err != nil {
		return PingResponse{}, err
	}
	return out, nil
}

func (c *CRUDClient) List() ([]session.Metadata, error) {
	var out []session.Metadata
	if err := c.Call("list", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *CRUDClient) Get(id string) (session.Metadata, error) {
	var out session.Metadata
	err := c.Call("get", agentserver.IDRequest{ID: id}, &out)
	return out, err
}

func (c *CRUDClient) Create(req agentserver.CreateRequest) (session.Metadata, error) {
	var out session.Metadata
	err := c.Call("create", req, &out)
	return out, err
}

func (c *CRUDClient) Edit(req agentserver.EditRequest) (session.Metadata, error) {
	var out session.Metadata
	err := c.Call("edit", req, &out)
	return out, err
}

func (c *CRUDClient) Clone(sourceID, name string) (session.Metadata, error) {
	var out session.Metadata
	err := c.Call("clone", agentserver.CloneRequest{SourceID: sourceID, Name: name}, &out)
	return out, err
}

func (c *CRUDClient) Delete(id string) error {
	return c.Call("delete", agentserver.IDRequest{ID: id}, nil)
}
func (c *CRUDClient) Kill(id string) error {
	return c.Call("kill", agentserver.IDRequest{ID: id}, nil)
}
func (c *CRUDClient) Background(id string) error {
	return c.Call("background", agentserver.IDRequest{ID: id}, nil)
}
func (c *CRUDClient) Override(id string) error {
	return c.Call("override", agentserver.IDRequest{ID: id}, nil)
}
func (c *CRUDClient) Restart(id string) error {
	return c.Call("restart", agentserver.IDRequest{ID: id}, nil)
}
