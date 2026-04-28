// Package main is the entry point for convocate.
package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/asymmetric-effort/convocate/internal/agentclient"
	"github.com/asymmetric-effort/convocate/internal/config"
	"github.com/asymmetric-effort/convocate/internal/dns"
	"github.com/asymmetric-effort/convocate/internal/install"
	"github.com/asymmetric-effort/convocate/internal/logging"
	"github.com/asymmetric-effort/convocate/internal/menu"
	"github.com/asymmetric-effort/convocate/internal/multihost"
	"github.com/asymmetric-effort/convocate/internal/session"
	"github.com/asymmetric-effort/convocate/internal/user"
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
			// would be a chicken-and-egg problem. Version is threaded
			// through so the image build tag matches this binary's
			// semver — agents pull the same versioned image later.
			return install.New(Version).Run()
		case "status-serve":
			// status-serve is the systemd unit for the TLS-authenticated
			// shell-side listener — runs as root so it can bind tcp/222
			// and write to /etc/convocate.
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
	// (session metadata, ~/.claude, /etc/convocate/agent-keys) is
	// owned by that user; running the TUI as anyone else would
	// silently produce broken state.
	if err := user.EnforceRunningAs(config.ConvocateUser); err != nil {
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
	// Lookup claude user for path resolution only. The shell no longer
	// runs containers locally — no docker image check — so agent
	// discovery is the only prerequisite beyond the user existing.
	userInfo, err := user.Lookup(config.ConvocateUser)
	if err != nil {
		return fmt.Errorf("failed to lookup claude user: %w", err)
	}
	_ = userInfo

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
			Agents:            router.AgentIDs(),
			SaveSessionEdit: func(id, name, protocol, dnsName string, port int) error {
				if err := router.Update(id, name, protocol, dnsName, port); err != nil {
					return err
				}
				if log != nil {
					log.Infof("updated session %s: name=%q port=%d/%s dns=%q", id, name, port, protocol, dnsName)
				}
				syncDNSRecords(router, log)
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
			if err := handleNewSession(router, sel.AgentID, sel.Name, sel.Port, sel.Protocol, sel.DNSName, log); err != nil {
				fmt.Fprintf(os.Stderr, "session error: %v\n", err)
			}
			syncDNSRecords(router, log)
			continue
		case menu.ActionCloneSession:
			if err := handleCloneSession(router, sel.SessionID, sel.Name, log); err != nil {
				fmt.Fprintf(os.Stderr, "session error: %v\n", err)
			}
			syncDNSRecords(router, log)
			continue
		case menu.ActionDeleteSession:
			if err := router.Delete(sel.SessionID); err != nil {
				fmt.Fprintf(os.Stderr, "delete error: %v\n", err)
			} else {
				fmt.Printf("Deleted session %q (%s)\n", sel.Name, shortID(sel.SessionID))
				syncDNSRecords(router, log)
			}
			continue
		default:
			// Resume: only remote attach is supported. Orphan (local)
			// sessions print a migration notice instead of launching a
			// local container.
			meta := findInSessions(sessions, sel.SessionID)
			if meta.IsRemote() {
				if err := handleRemoteAttach(router, meta, log); err != nil {
					fmt.Fprintf(os.Stderr, "remote attach error: %v\n", err)
				}
				continue
			}
			fmt.Fprintf(os.Stderr, "session %q is a local orphan (shell host no longer runs containers); migrate it to a convocate-agent to resume\n", meta.Name)
			continue
		}
	}
}

// buildRouter wires a multihost.Router that falls back to local-only when
// /etc/convocate/agent-keys/ is empty or missing (the common case until
// the operator runs init-agent). Returns a deferred cleanup that closes
// every agent client.
// buildRouter wires a multihost.Router against every registered agent.
// Local sessions come through as read-only orphans (pending migration) —
// the shell no longer runs containers itself.
func buildRouter(mgr *session.Manager, log *logging.Logger) (*multihost.Router, func()) {
	router := &multihost.Router{Local: mgr}
	closers := []func(){}

	records, err := agentclient.DiscoverAgents("")
	if err != nil && log != nil {
		log.Warningf("agent discovery: %v", err)
	}
	for _, rec := range records {
		client, err := agentclient.NewCRUDClient(agentclient.CRUDConfig{
			AgentHost:         rec.Host,
			PrivateKeyPath:    rec.PrivateKeyPath,
			HeartbeatInterval: 30 * time.Second,
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

	return router, func() {
		for _, c := range closers {
			c()
		}
	}
}

// handleRemoteAttach runs an interactive attach against the remote agent
// that owns meta. It reuses the per-agent SSH connection the router holds
// open so attach doesn't re-handshake; the local TUI has already Fini'd
// the tcell screen by the time menu.Display returned, so os.Stdin/Stdout
// are safe to hand off to the pty relay.
func handleRemoteAttach(router *multihost.Router, meta session.Metadata, log *logging.Logger) error {
	ref := router.AgentFor(meta.UUID)
	if ref == nil {
		return fmt.Errorf("no agent registered for session %s", shortID(meta.UUID))
	}
	client, ok := ref.Client.(*agentclient.CRUDClient)
	if !ok {
		return fmt.Errorf("agent client does not support attach (test stub?)")
	}
	if log != nil {
		log.Infof("remote attach: session=%s agent=%s host=%s", meta.UUID, meta.AgentID, meta.AgentHost)
	}
	fmt.Printf("Attaching to %q on agent %s (Ctrl-B d to detach)...\n", meta.Name, meta.AgentID)
	return agentclient.Attach(client.SSHClient(), agentclient.AttachOptions{
		SessionID:         meta.UUID,
		Stdin:             os.Stdin,
		Stdout:            os.Stdout,
		Stderr:            os.Stderr,
		EnableRawTerminal: true,
	})
}

// syncDNSRecords rewrites /var/lib/convocate/dnsmasq-hosts with one
// entry per remote session that has a DNSName set. The record points at
// the agent's host IP since that's where the container runs + publishes
// ports. Orphan (local) sessions are intentionally excluded — the shell
// host no longer runs containers post-v2, so any DNS name on an orphan
// is stale until the session migrates.
//
// Failures are logged at warning level, not returned — the shell must
// keep working if dnsmasq isn't installed, the hosts dir isn't
// writable, or DNS lookups time out.
func syncDNSRecords(router *multihost.Router, log *logging.Logger) {
	if !dns.HostsFileExists(dns.DefaultHostsFile) {
		return
	}
	sessions, err := router.List()
	if err != nil {
		if log != nil {
			log.Warningf("dns sync: list failed: %v", err)
		}
		return
	}
	var records []dns.Record
	for _, s := range sessions {
		if s.DNSName == "" || s.AgentHost == "" {
			continue
		}
		ip, err := resolveAgentIP(s.AgentHost)
		if err != nil {
			if log != nil {
				log.Warningf("dns sync: resolve %s: %v", s.AgentHost, err)
			}
			continue
		}
		records = append(records, dns.Record{Name: s.DNSName, IP: ip})
	}
	if err := dns.WriteHostsFile(dns.DefaultHostsFile, records); err != nil {
		if log != nil {
			log.Warningf("dns sync: write %s: %v", dns.DefaultHostsFile, err)
		}
		return
	}
	if log != nil {
		log.Infof("dns sync: wrote %d records to %s", len(records), dns.DefaultHostsFile)
	}
}

// resolveAgentIP takes the stored agent-host string (which may be an IP
// or a hostname) and returns an IPv4 address suitable for dnsmasq's
// hosts file. A hostname falls through to net.LookupHost and returns
// the first IPv4 match.
func resolveAgentIP(host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		return host, nil
	}
	ips, err := net.LookupHost(host)
	if err != nil {
		return "", err
	}
	for _, s := range ips {
		if ip := net.ParseIP(s); ip != nil && ip.To4() != nil {
			return s, nil
		}
	}
	if len(ips) > 0 {
		return ips[0], nil // IPv6 fallback
	}
	return "", fmt.Errorf("no IPs for %s", host)
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

// handleNewSession routes a Create through the Router to the chosen
// agent. Local creates were removed in v2.x; agentID must be non-empty.
func handleNewSession(router *multihost.Router, agentID, name string, port int, protocol, dnsName string, log *logging.Logger) error {
	meta, err := router.Create(agentID, session.CreateOptions{
		Port:     port,
		Protocol: protocol,
		DNSName:  dnsName,
	}, name)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	if log != nil {
		log.Infof("created session %s (%s) agent=%s port=%d/%s dns=%q",
			meta.UUID, meta.Name, meta.AgentID, meta.Port, meta.EffectiveProtocol(), meta.DNSName)
	}
	fmt.Printf("Created session %q (%s) on agent %s (%s)\n",
		meta.Name, shortID(meta.UUID), meta.AgentID, meta.AgentHost)
	if meta.Port > 0 {
		fmt.Printf("  published port %d/%s\n", meta.Port, meta.EffectiveProtocol())
	}
	if meta.DNSName != "" {
		fmt.Printf("  DNS name %q registered on agent\n", meta.DNSName)
	}
	fmt.Println("  press Enter on the session from the menu to attach.")
	return nil
}

// handleCloneSession clones a remote session on its owning agent. Local
// (orphan) sessions cannot be cloned — the Router returns
// ErrOrphanNeedsMigration which surfaces here.
func handleCloneSession(router *multihost.Router, sourceID, name string, log *logging.Logger) error {
	meta, err := router.Clone(sourceID, name)
	if err != nil {
		return fmt.Errorf("failed to clone session: %w", err)
	}
	if log != nil {
		log.Infof("cloned session %s from %s (%s)", meta.UUID, sourceID, meta.Name)
	}
	fmt.Printf("Cloned session %q (%s) from %s on agent %s\n",
		meta.Name, shortID(meta.UUID), shortID(sourceID), meta.AgentID)
	return nil
}

func printUsage() {
	fmt.Printf(`%s - Isolated Claude CLI sessions in Docker containers

Usage:
  %s              Launch the session manager
  %s install      Install and configure convocate
  %s version      Print version information
  %s help         Show this help message
`, config.AppName, config.AppName, config.AppName, config.AppName, config.AppName)
}
