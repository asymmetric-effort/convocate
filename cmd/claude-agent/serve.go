package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/asymmetric-effort/claude-shell/internal/agentclient"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Try to bring up the status emitter. init-agent writes the shell host +
	// private key to /etc/claude-agent during provisioning; if either is
	// missing we run without emission so the agent is still usable
	// standalone.
	emitter, emitterErr := maybeStartEmitter(ctx, agentID)
	if emitterErr != nil {
		fmt.Fprintf(os.Stderr, "claude-agent: status emitter disabled: %v\n", emitterErr)
	}
	var pub agentserver.StatusPublisher
	if emitter != nil {
		pub = emitter
	}
	orch := agentserver.NewSessionOrchestrator(mgr, u, paths, dns.DetectHostIP(), agentID, pub)
	orch.ImageRef = readCurrentImage(defaultCurrentImageFile)
	if orch.ImageRef != "" {
		fmt.Fprintf(os.Stderr, "claude-agent: sessions will use image %q\n", orch.ImageRef)
	}

	d := agentserver.NewDispatcher()
	agentserver.RegisterCoreOps(d, agentID, Version)
	agentserver.RegisterCRUDOps(d, orch)

	// AttachTarget gates attach on "the agent actually owns this session" —
	// prevents an authenticated client from probing for arbitrary container
	// names on the host.
	attachTarget := &agentserver.DockerAttachTarget{
		ExistsFn: func(id string) bool {
			_, err := mgr.Get(id)
			return err == nil
		},
	}

	srv, err := agentserver.New(agentserver.Config{
		HostKeyPath:        defaultHostKeyPath,
		AuthorizedKeysPath: defaultAuthKeysPath,
		Listen:             defaultListen,
		Dispatcher:         d,
		AttachTarget:       attachTarget,
	})
	if err != nil {
		return fmt.Errorf("init server: %w", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	return srv.Serve(ctx)
}

// readCurrentImage returns the trimmed content of path or an empty
// string if the file is missing / unreadable / empty. A blank return
// tells the orchestrator "use the compile-time default" — useful before
// init-agent has pushed an image pointer.
func readCurrentImage(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// maybeStartEmitter reads the shell-host address and key path from
// /etc/claude-agent. If either is missing, the function returns (nil, err)
// and the caller continues without the status plane. The emitter's Run
// goroutine stays alive until ctx is canceled.
func maybeStartEmitter(ctx context.Context, agentID string) (*agentclient.StatusEmitter, error) {
	hostBytes, err := os.ReadFile(defaultShellHostFile)
	if err != nil {
		return nil, fmt.Errorf("read shell host file: %w", err)
	}
	host := strings.TrimSpace(string(hostBytes))
	if host == "" {
		return nil, fmt.Errorf("shell host file is empty")
	}
	if _, err := os.Stat(defaultShellPrivateKeyPath); err != nil {
		return nil, fmt.Errorf("stat agent->shell key: %w", err)
	}
	emitter, err := agentclient.NewStatusEmitter(agentclient.Config{
		ShellHost:         host,
		ShellPort:         223,
		User:              "claude",
		PrivateKeyPath:    defaultShellPrivateKeyPath,
		AgentID:           agentID,
		HeartbeatInterval: 30 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	go emitter.Run(ctx)
	return emitter, nil
}
