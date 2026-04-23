package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/asymmetric-effort/claude-shell/internal/agentserver"
)

// cmdServe starts the claude-agent SSH listener in the foreground. Systemd
// invokes this via the unit's ExecStart. It does NOT require root — the
// service runs as the claude user.
func cmdServe(_ []string) error {
	agentID, err := loadOrCreateAgentID(defaultAgentIDPath)
	if err != nil {
		return fmt.Errorf("agent-id: %w", err)
	}

	d := agentserver.NewDispatcher()
	agentserver.RegisterCoreOps(d, agentID, Version)

	srv, err := agentserver.New(agentserver.Config{
		HostKeyPath:        defaultHostKeyPath,
		AuthorizedKeysPath: defaultAuthKeysPath,
		Listen:             defaultListen,
		Dispatcher:         d,
	})
	if err != nil {
		return fmt.Errorf("init server: %w", err)
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
