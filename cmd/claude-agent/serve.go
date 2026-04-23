package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/asymmetric-effort/claude-shell/internal/agentserver"
	"github.com/asymmetric-effort/claude-shell/internal/config"
	"github.com/asymmetric-effort/claude-shell/internal/dns"
	"github.com/asymmetric-effort/claude-shell/internal/session"
	"github.com/asymmetric-effort/claude-shell/internal/user"
)

// cmdServe starts the claude-agent SSH listener in the foreground. Systemd
// invokes this via the unit's ExecStart. It does NOT require root — the
// service runs as the claude user.
func cmdServe(_ []string) error {
	agentID, err := loadOrCreateAgentID(defaultAgentIDPath)
	if err != nil {
		return fmt.Errorf("agent-id: %w", err)
	}

	u, err := user.Lookup(defaultClaudeUsername)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", defaultClaudeUsername, err)
	}
	paths := config.PathsFromHome(u.HomeDir)
	mgr := session.NewManager(paths.SessionsBase, paths.SkelDir)
	orch := agentserver.NewSessionOrchestrator(mgr, u, paths, dns.DetectHostIP())

	d := agentserver.NewDispatcher()
	agentserver.RegisterCoreOps(d, agentID, Version)
	agentserver.RegisterCRUDOps(d, orch)

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
