package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/asymmetric-effort/claude-shell/internal/shellserver"
	"github.com/asymmetric-effort/claude-shell/internal/statusproto"
)

// Well-known paths for the claude-shell status listener. These match the
// conventions set by init-shell (CA + host key co-located; agent-side
// public keys delivered by init-agent and appended to the allowlist).
const (
	defaultStatusHostKeyPath    = "/etc/claude-shell/status_host_ed25519_key"
	defaultStatusAuthKeysPath   = "/etc/claude-shell/status_authorized_keys"
	defaultStatusListen         = ":222"
	defaultStatusLogDir         = "/var/log/claude-agent"
	defaultStatusShellLogFile   = "/var/log/claude-shell.log"
)

// cmdStatusServe runs the shell-side SSH listener that receives status
// events from claude-agent hosts. Intended for invocation by a systemd
// unit written during claude-host init-shell; runnable manually for
// testing.
func cmdStatusServe(_ []string) error {
	logger := newStatusLogger()

	listener := shellserver.ListenerFunc(func(ctx context.Context, ev statusproto.Event) error {
		// For now, just log. Feature 2d-follow-up will land the per-agent
		// log file sink writing to /var/log/claude-agent/<agent-id>.log.
		logger.Printf("[status] %s agent=%s session=%s data=%s",
			ev.Type, ev.AgentID, ev.SessionID, string(ev.Data))
		return nil
	})

	srv, err := shellserver.New(shellserver.Config{
		HostKeyPath:        defaultStatusHostKeyPath,
		AuthorizedKeysPath: defaultStatusAuthKeysPath,
		Listen:             defaultStatusListen,
		Listener:           listener,
		Logger:             logger,
	})
	if err != nil {
		return fmt.Errorf("shell status server: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	return srv.Serve(ctx)
}
