// Package agentserver implements the claude-agent SSH server, subsystem
// dispatcher, and RPC handlers.
//
// Security posture: the server accepts only the claude-agent-rpc and
// claude-agent-attach subsystems. Shell, exec, and other SSH channel /
// request types are rejected. There is no arbitrary command execution path.
package agentserver

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"

	"golang.org/x/crypto/ssh"

	"github.com/asymmetric-effort/claude-shell/internal/sshutil"
)

// RPCSubsystem is the only subsystem the server accepts on a session
// channel. Any other subsystem (or shell/exec request) is rejected.
const RPCSubsystem = "claude-agent-rpc"

// Config configures a Server.
type Config struct {
	// HostKeyPath is the on-disk path to the ed25519 host key. If missing,
	// a fresh one is generated at first start.
	HostKeyPath string

	// AuthorizedKeysPath is the file listing public keys allowed to connect.
	// A missing file means "no client is allowed", which is the safe default
	// until init-agent populates the file.
	AuthorizedKeysPath string

	// Listen is the tcp address (e.g. ":222"). Empty defaults to ":222".
	Listen string

	// Dispatcher routes op names to handlers. Must be non-nil.
	Dispatcher *Dispatcher

	// AttachTarget resolves claude-agent-attach subsystem requests to a
	// container PTY. When nil, the attach subsystem is refused.
	AttachTarget AttachTarget

	// AttachHooks let the server notify the orchestrator when an attach
	// session opens/closes. Optional — nil fields are skipped.
	AttachHooks AttachHooks

	// Logger receives diagnostic lines. Nil = log to the standard logger.
	Logger *log.Logger
}

// Server runs the claude-agent SSH listener.
type Server struct {
	cfg    Config
	signer ssh.Signer
	auth   *sshutil.AuthorizedKeys
}

// New builds a Server from Config — loads host key, loads authorized keys.
// The listener isn't created until Serve is called.
func New(cfg Config) (*Server, error) {
	if cfg.Dispatcher == nil {
		return nil, fmt.Errorf("agentserver: Dispatcher is required")
	}
	if cfg.Listen == "" {
		cfg.Listen = ":222"
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	signer, err := sshutil.LoadOrCreateHostKey(cfg.HostKeyPath)
	if err != nil {
		return nil, err
	}
	auth, err := sshutil.NewAuthorizedKeys(cfg.AuthorizedKeysPath)
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, signer: signer, auth: auth}, nil
}

// Serve listens on cfg.Listen and accepts SSH connections until ctx is done
// or the listener errors fatally. Each accepted connection runs in its own
// goroutine.
func (s *Server) Serve(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.Listen, err)
	}
	s.cfg.Logger.Printf("claude-agent: listening on %s (authorized keys: %d)", s.cfg.Listen, s.auth.Len())

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}
		go s.handleConn(ctx, conn)
	}
}

// handleConn performs the SSH handshake, then accepts channels. All channel
// types other than "session" are rejected.
func (s *Server) handleConn(ctx context.Context, nconn net.Conn) {
	defer nconn.Close()

	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if s.auth.IsAuthorized(key) {
				return &ssh.Permissions{
					Extensions: map[string]string{
						"pubkey-fp": sshutil.KeyFingerprint(key),
					},
				}, nil
			}
			return nil, fmt.Errorf("key rejected")
		},
		ServerVersion: "SSH-2.0-claude-agent",
	}
	cfg.AddHostKey(s.signer)

	sshConn, chans, reqs, err := ssh.NewServerConn(nconn, cfg)
	if err != nil {
		s.cfg.Logger.Printf("claude-agent: handshake from %s failed: %v", nconn.RemoteAddr(), err)
		return
	}
	defer sshConn.Close()
	s.cfg.Logger.Printf("claude-agent: connection from %s (user=%s, fp=%s)",
		sshConn.RemoteAddr(), sshConn.User(), sshConn.Permissions.Extensions["pubkey-fp"])

	// Global requests aren't something we support — discard cleanly.
	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "only session channels accepted")
			continue
		}
		channel, chReqs, err := newChan.Accept()
		if err != nil {
			s.cfg.Logger.Printf("claude-agent: accept channel failed: %v", err)
			continue
		}
		go s.handleSession(ctx, channel, chReqs)
	}
}

// handleSession processes channel-level requests. Only "subsystem" with a
// recognized subsystem name is accepted. "shell", "exec", "pty-req", "env",
// and anything else are all refused.
func (s *Server) handleSession(ctx context.Context, ch ssh.Channel, reqs <-chan *ssh.Request) {
	// Close is best-effort — individual subsystem handlers may take over and
	// close the channel themselves.
	for req := range reqs {
		switch req.Type {
		case "subsystem":
			name := parseStringPayload(req.Payload)
			switch name {
			case RPCSubsystem:
				_ = req.Reply(true, nil)
				s.cfg.Dispatcher.Handle(ch, ch)
				_ = ch.CloseWrite()
				_ = ch.Close()
				return
			case AttachSubsystem:
				if s.cfg.AttachTarget == nil {
					_ = req.Reply(false, nil)
					s.cfg.Logger.Printf("claude-agent: attach subsystem requested but no AttachTarget configured")
					_ = ch.Close()
					return
				}
				_ = req.Reply(true, nil)
				// Hand over the request stream so attach can forward
				// window-change events to the PTY.
				HandleAttach(ctx, ch, reqs, s.cfg.AttachTarget, s.cfg.AttachHooks)
				return
			default:
				_ = req.Reply(false, nil)
				s.cfg.Logger.Printf("claude-agent: rejected subsystem %q", name)
				_ = ch.Close()
				return
			}
		default:
			// Shell, exec, pty-req, env, window-change, signal, etc — none of
			// these are allowed on the agent API outside of the attach
			// subsystem (where we consume reqs ourselves).
			_ = req.Reply(false, nil)
		}
	}
	_ = ch.Close()
}

// parseStringPayload decodes a single length-prefixed string from an SSH
// request payload (SSH messages encode strings as uint32 length + bytes).
func parseStringPayload(payload []byte) string {
	if len(payload) < 4 {
		return ""
	}
	n := binary.BigEndian.Uint32(payload[:4])
	if int(4+n) > len(payload) {
		return ""
	}
	return string(payload[4 : 4+n])
}

// Close releases any resources. Listener shutdown is handled via ctx in
// Serve; this is kept for future symmetry.
func (s *Server) Close() error { return nil }

// assert interface conformance at compile time.
var _ io.Closer = (*Server)(nil)
