package agentclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/asymmetric-effort/claude-shell/internal/agentserver"
	"github.com/asymmetric-effort/claude-shell/internal/session"
)

// CRUDConfig configures a CRUDClient.
type CRUDConfig struct {
	// AgentHost is the hostname / IP of the claude-agent listener.
	AgentHost string
	// AgentPort defaults to 222 when zero.
	AgentPort int
	// User is the SSH username. Defaults to "claude".
	User string
	// PrivateKeyPath is the shell→agent SSH private key, typically
	// /etc/claude-shell/agent-keys/<id>/shell_to_agent_ed25519_key.
	PrivateKeyPath string
	// DialTimeout caps the initial TCP+SSH handshake. Defaults to 10s.
	DialTimeout time.Duration
}

// CRUDClient wraps an SSH connection to an agent and exposes one method per
// op the agent implements. A client is safe for concurrent use — each Call
// opens a fresh session channel, so overlapping calls don't interfere.
type CRUDClient struct {
	conn *ssh.Client
	cfg  CRUDConfig
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
		cfg.User = "claude"
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 10 * time.Second
	}
	signer, err := loadPrivateKey(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load shell->agent key: %w", err)
	}
	addr := net.JoinHostPort(cfg.AgentHost, fmt.Sprintf("%d", cfg.AgentPort))
	conn, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         cfg.DialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return &CRUDClient{conn: conn, cfg: cfg}, nil
}

// Close releases the SSH connection.
func (c *CRUDClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Call runs one op against the agent, decoding the result into out (which
// may be nil for ops that return an empty object). Returns an error if the
// agent replied with ok=false or the wire format is malformed.
func (c *CRUDClient) Call(op string, params any, out any) error {
	sess, err := c.conn.NewSession()
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
