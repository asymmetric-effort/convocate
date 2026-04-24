// Package main is the entry point for claude-shell.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/asymmetric-effort/claude-shell/internal/agentclient"
	"github.com/asymmetric-effort/claude-shell/internal/capacity"
	"github.com/asymmetric-effort/claude-shell/internal/config"
	"github.com/asymmetric-effort/claude-shell/internal/container"
	"github.com/asymmetric-effort/claude-shell/internal/dns"
	"github.com/asymmetric-effort/claude-shell/internal/install"
	"github.com/asymmetric-effort/claude-shell/internal/logging"
	"github.com/asymmetric-effort/claude-shell/internal/menu"
	"github.com/asymmetric-effort/claude-shell/internal/multihost"
	"github.com/asymmetric-effort/claude-shell/internal/session"
	"github.com/asymmetric-effort/claude-shell/internal/user"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 1 {
		switch args[1] {
		case "install":
			// install runs as root via sudo; it creates the claude user
			// and configures the host. Enforcing "must be claude" here
			// would be a chicken-and-egg problem.
			return install.New().Run()
		case "status-serve":
			// status-serve is the systemd unit for the TLS-authenticated
			// shell-side listener — runs as root so it can bind tcp/222
			// and write to /etc/claude-shell.
			return cmdStatusServe(args[2:])
		case "version":
			fmt.Printf("%s version %s\n", config.AppName, Version)
			return nil
		case "help", "--help", "-h":
			printUsage()
			return nil
		default:
			return fmt.Errorf("unknown command: %q (use 'help' for usage)", args[1])
		}
	}

	// The interactive TUI is claude user only. Every on-disk path
	// (session metadata, ~/.claude, /etc/claude-shell/agent-keys) is
	// owned by that user; running the TUI as anyone else would
	// silently produce broken state.
	if err := user.EnforceRunningAs(config.ClaudeUser); err != nil {
		return err
	}
	return runSessionManager()
}

func runSessionManager() error {
	log, err := logging.New(config.AppName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to connect to syslog: %v\n", err)
		// Continue without syslog
		return runSessionManagerWithLog(nil)
	}
	defer log.Close()
	return runSessionManagerWithLog(log)
}

func runSessionManagerWithLog(log *logging.Logger) error {
	// Check Docker image exists
	exists, err := container.ImageExists(nil)
	if err != nil {
		return fmt.Errorf("failed to check Docker image: %w", err)
	}
	if !exists {
		return fmt.Errorf("Docker image %q not found; run 'claude-shell install' first", config.ContainerImage())
	}

	// Lookup claude user
	userInfo, err := user.Lookup(config.ClaudeUser)
	if err != nil {
		return fmt.Errorf("failed to lookup claude user: %w", err)
	}

	paths := config.PathsFromHome(userInfo.HomeDir)

	mgr := session.NewManager(paths.SessionsBase, paths.SkelDir)

	router, closeAgents := buildRouter(mgr, log)
	defer closeAgents()

	for {
		sessions, err := router.List()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		sel, err := menu.Display(sessions, menu.DisplayOptions{
			IsLocked:          router.IsLocked,
			IsRunning:         router.IsRunning,
			Reload:            router.List,
			OverrideLock:      router.OverrideLock,
			KillSession:       router.Kill,
			BackgroundSession: router.Background,
			RestartSession:    router.Restart,
			SaveSessionEdit: func(id, name, protocol, dnsName string, port int) error {
				if err := router.Update(id, name, protocol, dnsName, port); err != nil {
					return err
				}
				if log != nil {
					log.Infof("updated session %s: name=%q port=%d/%s dns=%q", id, name, port, protocol, dnsName)
				}
				if err := syncDNSRecords(mgr, log); err != nil && log != nil {
					log.Warningf("failed to sync dnsmasq hosts after edit: %v", err)
				}
				return nil
			},
		})
		if err != nil {
			return fmt.Errorf("menu error: %w", err)
		}

		switch sel.Action {
		case menu.ActionQuit:
			fmt.Println("Goodbye!")
			return nil
		case menu.ActionNewSession:
			// New sessions are always created locally for now. Remote
			// provisioning happens via 'claude-host init-agent'.
			if err := handleNewSession(mgr, sel.Name, sel.Port, sel.Protocol, sel.DNSName, userInfo, paths, log); err != nil {
				fmt.Fprintf(os.Stderr, "session error: %v\n", err)
			}
			continue
		case menu.ActionCloneSession:
			if err := handleCloneSession(router, mgr, sel.SessionID, sel.Name, userInfo, paths, log); err != nil {
				fmt.Fprintf(os.Stderr, "session error: %v\n", err)
			}
			continue
		case menu.ActionDeleteSession:
			if err := router.Delete(sel.SessionID); err != nil {
				fmt.Fprintf(os.Stderr, "delete error: %v\n", err)
			} else {
				fmt.Printf("Deleted session %q (%s)\n", sel.Name, shortID(sel.SessionID))
				if err := syncDNSRecords(mgr, log); err != nil && log != nil {
					log.Warningf("failed to sync dnsmasq hosts after delete: %v", err)
				}
			}
			continue
		default:
			// Resume: if the session lives on a remote agent we can't
			// attach from here yet — tell the operator how to reach it
			// rather than silently doing nothing.
			if meta := findInSessions(sessions, sel.SessionID); meta.IsRemote() {
				fmt.Fprintf(os.Stderr, "remote session attach not yet supported — SSH to %s to attach\n", meta.AgentHost)
				continue
			}
			if err := handleResumeSession(mgr, sel.SessionID, userInfo, paths, log); err != nil {
				fmt.Fprintf(os.Stderr, "session error: %v\n", err)
			}
			continue
		}
	}
}

// buildRouter wires a multihost.Router that falls back to local-only when
// /etc/claude-shell/agent-keys/ is empty or missing (the common case until
// the operator runs init-agent). Returns a deferred cleanup that closes
// every agent client.
func buildRouter(mgr *session.Manager, log *logging.Logger) (*multihost.Router, func()) {
	router := &multihost.Router{
		Local:           mgr,
		LocalKill:       container.StopContainer,
		LocalBackground: container.DetachClients,
		LocalIsRunning:  container.IsContainerRunning,
	}
	closers := []func(){}

	records, err := agentclient.DiscoverAgents("")
	if err != nil && log != nil {
		log.Warningf("agent discovery: %v", err)
	}
	for _, rec := range records {
		client, err := agentclient.NewCRUDClient(agentclient.CRUDConfig{
			AgentHost:      rec.Host,
			PrivateKeyPath: rec.PrivateKeyPath,
		})
		if err != nil {
			if log != nil {
				log.Warningf("dial agent %s (%s): %v", rec.ID, rec.Host, err)
			}
			fmt.Fprintf(os.Stderr, "warning: agent %s (%s) unreachable: %v\n", rec.ID, rec.Host, err)
			continue
		}
		closers = append(closers, func() { _ = client.Close() })
		router.Agents = append(router.Agents, multihost.AgentRef{
			Record: rec,
			Client: client,
		})
	}

	// LocalRestart needs the session lookup + paths so we bind it after
	// mgr is in scope.
	router.LocalRestart = func(id string) error {
		userInfo, err := user.Lookup(config.ClaudeUser)
		if err != nil {
			return err
		}
		paths := config.PathsFromHome(userInfo.HomeDir)
		return restartSessionDetached(mgr, id, userInfo, paths, log)
	}

	return router, func() {
		for _, c := range closers {
			c()
		}
	}
}

// findInSessions returns the first metadata whose UUID matches id. Zero
// value when not found.
func findInSessions(sessions []session.Metadata, id string) session.Metadata {
	for _, s := range sessions {
		if s.UUID == id {
			return s
		}
	}
	return session.Metadata{}
}

// shortID returns the first 8 characters of a session UUID, or the full id
// if it's already short (e.g. a routing sentinel).
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func handleNewSession(mgr *session.Manager, name string, port int, protocol, dnsName string, userInfo user.Info, paths config.Paths, log *logging.Logger) error {
	meta, err := mgr.CreateWithOptions(name, session.CreateOptions{
		Port:     port,
		Protocol: protocol,
		DNSName:  dnsName,
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	if log != nil {
		log.Infof("created session %s (%s) port=%d/%s dns=%q", meta.UUID, meta.Name, meta.Port, meta.EffectiveProtocol(), meta.DNSName)
	}
	if meta.Port > 0 {
		fmt.Printf("Created session %q (%s) on port %d/%s\n", meta.Name, meta.UUID[:8], meta.Port, meta.EffectiveProtocol())
	} else {
		fmt.Printf("Created session %q (%s)\n", meta.Name, meta.UUID[:8])
	}
	if meta.DNSName != "" {
		fmt.Printf("Registered DNS name %q\n", meta.DNSName)
	}

	if err := syncDNSRecords(mgr, log); err != nil && log != nil {
		log.Warningf("failed to sync dnsmasq hosts after create: %v", err)
	}

	return launchSession(mgr, meta.UUID, meta.Port, meta.EffectiveProtocol(), userInfo, paths, log)
}

// syncDNSRecords rewrites the dnsmasq-hosts file claude-shell manages from
// the current session metadata. If the install hasn't set up the dnsmasq
// integration (parent directory missing) the call is a no-op so stock
// claude-shell users aren't blocked.
func syncDNSRecords(mgr *session.Manager, log *logging.Logger) error {
	if !dns.HostsFileExists(dns.DefaultHostsFile) {
		return nil
	}
	sessions, err := mgr.List()
	if err != nil {
		return err
	}
	hostIP := dns.DetectHostIP()
	var records []dns.Record
	for _, s := range sessions {
		if s.DNSName == "" {
			continue
		}
		records = append(records, dns.Record{Name: s.DNSName, IP: hostIP})
	}
	if log != nil {
		log.Infof("syncing %d dnsmasq records -> %s", len(records), dns.DefaultHostsFile)
	}
	return dns.WriteHostsFile(dns.DefaultHostsFile, records)
}

func handleCloneSession(router *multihost.Router, mgr *session.Manager, sourceID, name string, userInfo user.Info, paths config.Paths, log *logging.Logger) error {
	meta, err := router.Clone(sourceID, name)
	if err != nil {
		return fmt.Errorf("failed to clone session: %w", err)
	}

	if log != nil {
		log.Infof("cloned session %s from %s (%s)", meta.UUID, sourceID, meta.Name)
	}
	fmt.Printf("Cloned session %q (%s) from %s\n", meta.Name, shortID(meta.UUID), shortID(sourceID))

	// Cloning a remote session produces a remote session — no local attach.
	if meta.IsRemote() {
		fmt.Printf("(remote clone on agent %s — SSH to %s to attach)\n", meta.AgentID, meta.AgentHost)
		return nil
	}
	return launchSession(mgr, meta.UUID, meta.Port, meta.EffectiveProtocol(), userInfo, paths, log)
}

// newRunner is the package-level factory for container runners. Tests override
// it to substitute a runner backed by a mock exec function.
var newRunner = container.NewRunner

// restartSessionDetached launches the session's container in background mode
// without attaching a user terminal. The container runs autonomously until the
// user attaches (via Enter on a running session) or kills it.
func restartSessionDetached(mgr *session.Manager, sessionID string, userInfo user.Info, paths config.Paths, log *logging.Logger) error {
	meta, err := mgr.Get(sessionID)
	if err != nil {
		return err
	}

	if err := mgr.Touch(sessionID); err != nil {
		if log != nil {
			log.Warningf("failed to update last accessed time: %v", err)
		}
	}

	runner := newRunner(sessionID, mgr.SessionDir(sessionID), userInfo, paths)
	runner.SetPort(meta.Port)
	runner.SetProtocol(meta.EffectiveProtocol())
	runner.SetDNSServer(dns.DetectHostIP())

	running, err := runner.IsRunning()
	if err != nil {
		return fmt.Errorf("failed to check container status: %w", err)
	}
	if running {
		return fmt.Errorf("session %q is already running", meta.Name)
	}

	if err := capacity.Check(capacity.DefaultThreshold); err != nil {
		return err
	}

	if log != nil {
		log.Infof("restarting session %s (%s) in background", meta.UUID, meta.Name)
	}
	return runner.StartDetached()
}

func handleResumeSession(mgr *session.Manager, sessionID string, userInfo user.Info, paths config.Paths, log *logging.Logger) error {
	meta, err := mgr.Get(sessionID)
	if err != nil {
		return err
	}

	if log != nil {
		log.Infof("resuming session %s (%s)", meta.UUID, meta.Name)
	}
	fmt.Printf("Resuming session %q (%s)\n", meta.Name, meta.UUID[:8])

	return launchSession(mgr, sessionID, meta.Port, meta.EffectiveProtocol(), userInfo, paths, log)
}

func launchSession(mgr *session.Manager, sessionID string, port int, protocol string, userInfo user.Info, paths config.Paths, log *logging.Logger) error {
	unlock, err := mgr.Lock(sessionID)
	if err != nil {
		return err
	}
	defer unlock()

	if err := mgr.Touch(sessionID); err != nil {
		if log != nil {
			log.Warningf("failed to update last accessed time: %v", err)
		}
	}

	runner := container.NewRunner(sessionID, mgr.SessionDir(sessionID), userInfo, paths)
	runner.SetPort(port)
	runner.SetProtocol(protocol)
	runner.SetDNSServer(dns.DetectHostIP())

	// Check if container is already running
	running, err := runner.IsRunning()
	if err != nil {
		return fmt.Errorf("failed to check container status: %w", err)
	}
	if running {
		if log != nil {
			log.Infof("attaching to running container for session %s", sessionID)
		}
		if err := runner.Attach(); err != nil {
			// Ignore normal exit status from tmux detach
			if !strings.Contains(err.Error(), "exit status") {
				return fmt.Errorf("attach error: %w", err)
			}
		}
		return nil
	}

	// Refuse to start new containers when the host is already saturated.
	if err := capacity.Check(capacity.DefaultThreshold); err != nil {
		return err
	}

	// Set up signal handling for graceful termination
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	done := make(chan error, 1)
	go func() {
		done <- runner.Start()
	}()

	select {
	case sig := <-sigCh:
		if log != nil {
			log.Infof("received signal %v, stopping session %s", sig, sessionID)
		}
		fmt.Printf("\n\nReceived %v. Gracefully stopping session...\n", sig)
		if err := runner.Stop(); err != nil {
			if log != nil {
				log.Errorf("failed to stop container: %v", err)
			}
		}
		return nil
	case err := <-done:
		if err != nil {
			// Don't report error if it was just the container exiting normally
			if strings.Contains(err.Error(), "exit status") {
				return nil
			}
			return fmt.Errorf("session error: %w", err)
		}
		return nil
	}
}

func printUsage() {
	fmt.Printf(`%s - Isolated Claude CLI sessions in Docker containers

Usage:
  %s              Launch the session manager
  %s install      Install and configure claude-shell
  %s version      Print version information
  %s help         Show this help message
`, config.AppName, config.AppName, config.AppName, config.AppName, config.AppName)
}
