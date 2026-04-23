// Package shellserver runs the claude-shell host's SSH listener on tcp/222.
// Its only job is to accept the claude-shell-status subsystem from
// claude-agent hosts and deliver pushed events to a registered Listener.
// Everything else (shells, exec, other subsystems) is refused.
package shellserver

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"

	"golang.org/x/crypto/ssh"

	"github.com/asymmetric-effort/claude-shell/internal/sshutil"
	"github.com/asymmetric-effort/claude-shell/internal/statusproto"
)

// Listener receives every Event the server decodes. Returning a non-nil
// error is logged and the remote connection is closed.
type Listener interface {
	HandleEvent(ctx context.Context, ev statusproto.Event) error
}

// ListenerFunc adapts an ordinary function to Listener.
type ListenerFunc func(ctx context.Context, ev statusproto.Event) error

// HandleEvent implements Listener.
func (f ListenerFunc) HandleEvent(ctx context.Context, ev statusproto.Event) error {
	return f(ctx, ev)
}

// Config configures the shell-side status listener.
type Config struct {
	// HostKeyPath is the on-disk path to the ed25519 host key. If missing,
	// a fresh one is generated at first start.
	HostKeyPath string

	// AuthorizedKeysPath lists the agent public keys allowed to push status
	// events. Missing file = empty allowlist = no agent can connect.
	AuthorizedKeysPath string

	// Listen is the tcp address. Empty defaults to ":222".
	Listen string

	// Listener receives every decoded status event. Must be non-nil.
	Listener Listener

	// Logger receives diagnostic lines. Nil = standard logger.
	Logger *log.Logger
}

// Server runs the claude-shell status listener.
type Server struct {
	cfg    Config
	signer ssh.Signer
	auth   *sshutil.AuthorizedKeys
}

// New builds a Server from Config. Listener setup (net.Listen) is deferred
// until Serve.
func New(cfg Config) (*Server, error) {
	if cfg.Listener == nil {
		return nil, fmt.Errorf("shellserver: Listener is required")
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

// Serve accepts connections until ctx is done.
func (s *Server) Serve(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.Listen, err)
	}
	s.cfg.Logger.Printf("claude-shell: status listener on %s (authorized keys: %d)", s.cfg.Listen, s.auth.Len())

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

func (s *Server) handleConn(ctx context.Context, nconn net.Conn) {
	defer nconn.Close()

	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if s.auth.IsAuthorized(key) {
				return &ssh.Permissions{Extensions: map[string]string{
					"pubkey-fp": sshutil.KeyFingerprint(key),
				}}, nil
			}
			return nil, fmt.Errorf("key rejected")
		},
		ServerVersion: "SSH-2.0-claude-shell",
	}
	cfg.AddHostKey(s.signer)

	sshConn, chans, reqs, err := ssh.NewServerConn(nconn, cfg)
	if err != nil {
		s.cfg.Logger.Printf("claude-shell: handshake from %s failed: %v", nconn.RemoteAddr(), err)
		return
	}
	defer sshConn.Close()
	s.cfg.Logger.Printf("claude-shell: status connection from %s (user=%s, fp=%s)",
		sshConn.RemoteAddr(), sshConn.User(), sshConn.Permissions.Extensions["pubkey-fp"])

	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "only session channels accepted")
			continue
		}
		channel, chReqs, err := newChan.Accept()
		if err != nil {
			s.cfg.Logger.Printf("claude-shell: accept channel failed: %v", err)
			continue
		}
		go s.handleSession(ctx, channel, chReqs)
	}
}

func (s *Server) handleSession(ctx context.Context, ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		if req.Type != "subsystem" {
			_ = req.Reply(false, nil)
			continue
		}
		name := parseStringPayload(req.Payload)
		if name != statusproto.Subsystem {
			_ = req.Reply(false, nil)
			s.cfg.Logger.Printf("claude-shell: rejected subsystem %q", name)
			return
		}
		_ = req.Reply(true, nil)
		s.runStatusStream(ctx, ch)
		return
	}
}

// runStatusStream reads newline-delimited JSON events from ch and hands
// each one to the configured Listener. Returns when the channel closes or
// a decode error occurs (malformed events abort the stream; the next
// reconnect from the agent will pick up where it left off).
func (s *Server) runStatusStream(ctx context.Context, ch io.Reader) {
	dec := bufio.NewReader(ch)
	for {
		line, err := dec.ReadBytes('\n')
		if len(line) > 0 {
			var ev statusproto.Event
			if jerr := json.Unmarshal(line, &ev); jerr != nil {
				s.cfg.Logger.Printf("claude-shell: malformed event, dropping: %v", jerr)
				continue
			}
			if herr := s.cfg.Listener.HandleEvent(ctx, ev); herr != nil {
				s.cfg.Logger.Printf("claude-shell: listener error on %s: %v", ev.Type, herr)
			}
		}
		if err != nil {
			return
		}
	}
}

// parseStringPayload decodes a length-prefixed string from an SSH request
// payload. Duplicated from agentserver because there's no obvious better
// home for the 6-line helper.
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
