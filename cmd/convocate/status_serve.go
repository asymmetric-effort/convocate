package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/asymmetric-effort/convocate/internal/shellserver"
	"github.com/asymmetric-effort/convocate/internal/statusproto"
)

// Well-known paths for the convocate status listener. These match the
// conventions set by init-shell (CA + host key co-located; agent-side
// public keys delivered by init-agent and appended to the allowlist).
const (
	defaultStatusHostKeyPath  = "/etc/convocate/status_host_ed25519_key"
	defaultStatusAuthKeysPath = "/etc/convocate/status_authorized_keys"
	// :223 not :222 — convocate-agent owns :222 for its CRUD listener.
	// A combined host (agent + shell on one machine) needs the two SSH
	// servers on different ports or the second to bind fails. Agent keeps
	// the well-known slot because shell→agent dials happen on every CRUD
	// op while shell's inbound status is lower-frequency.
	defaultStatusListen       = ":223"
	defaultStatusLogDir       = "/var/log/convocate-agent"
	defaultStatusShellLogFile = "/var/log/convocate.log"
)

// cmdStatusServe runs the shell-side SSH listener that receives status
// events from convocate-agent hosts. Intended for invocation by a systemd
// unit written during convocate-host init-shell; runnable manually for
// testing.
func cmdStatusServe(_ []string) error {
	logger := newStatusLogger()

	listener := shellserver.ListenerFunc(func(ctx context.Context, ev statusproto.Event) error {
		// For now, just log. Feature 2d-follow-up will land the per-agent
		// log file sink writing to /var/log/convocate-agent/<agent-id>.log.
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
