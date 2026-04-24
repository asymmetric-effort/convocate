// Package main is the entry point for claude-host.
//
// claude-host provisions a vanilla Ubuntu machine to host claude-shell and/or
// claude-agent. It can run against the local machine (requiring local sudo)
// or against a remote host over SSH (requiring the remote user to have
// NOPASSWD sudoers privileges). Subcommands scaffolded here are filled in
// across subsequent commits.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/asymmetric-effort/claude-shell/internal/hostinstall"
)

const appName = "claude-host"

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		printUsage()
		return nil
	}
	sub := args[1]
	rest := args[2:]

	switch sub {
	case "install":
		return cmdInstall(rest)
	case "init-shell":
		return cmdInitShell(rest)
	case "init-agent":
		return cmdInitAgent(rest)
	case "update":
		return cmdUpdate(rest)
	case "migrate-session":
		return cmdMigrateSession(rest)
	case "version":
		fmt.Printf("%s version %s\n", appName, Version)
		return nil
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %q (use 'help' for usage)", sub)
	}
}

// parseTargetFlags is shared by the subcommands that accept --host/--user for
// remote execution. An empty host means "run locally".
type targetFlags struct {
	host string
	user string
}

func parseTargetFlags(sub string, args []string) (targetFlags, error) {
	fs := flag.NewFlagSet(sub, flag.ContinueOnError)
	var t targetFlags
	fs.StringVar(&t.host, "host", "", "remote host to target (empty = local)")
	fs.StringVar(&t.user, "user", os.Getenv("USER"), "remote user to connect as (ignored for local)")
	if err := fs.Parse(args); err != nil {
		return t, err
	}
	return t, nil
}

// requireLocalRoot returns a friendly error when the local invocation is
// being asked to operate on the local host without root. Remote invocations
// (i.e. --host set) do not need local root — only remote NOPASSWD sudo.
func requireLocalRoot(t targetFlags) error {
	if t.host != "" {
		return nil
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("%s must be run as root when targeting the local host — try 'sudo %s ...'", appName, appName)
	}
	return nil
}

func cmdInstall(args []string) error {
	t, err := parseTargetFlags("install", args)
	if err != nil {
		return err
	}
	if err := requireLocalRoot(t); err != nil {
		return err
	}

	ctx, cancel := signalContext()
	defer cancel()

	r, sshCfg, err := runnerFor(t)
	if err != nil {
		return err
	}
	defer r.Close()

	return hostinstall.Install(ctx, r, sshCfg, os.Stderr)
}

func cmdInitShell(args []string) error {
	fs := flag.NewFlagSet("init-shell", flag.ContinueOnError)
	var t targetFlags
	fs.StringVar(&t.host, "host", "", "remote host to target (empty = local)")
	fs.StringVar(&t.user, "user", os.Getenv("USER"), "remote user to connect as (ignored for local)")
	binary := fs.String("binary", "", "path to the local claude-shell binary (default: sibling of claude-host)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireLocalRoot(t); err != nil {
		return err
	}

	ctx, cancel := signalContext()
	defer cancel()

	r, sshCfg, err := runnerFor(t)
	if err != nil {
		return err
	}
	defer r.Close()

	return hostinstall.InitShell(ctx, r, sshCfg, hostinstall.InitShellOptions{
		BinaryPath: *binary,
	}, os.Stderr)
}

func cmdInitAgent(args []string) error {
	fs := flag.NewFlagSet("init-agent", flag.ContinueOnError)
	var t targetFlags
	fs.StringVar(&t.host, "host", "", "remote agent host to provision (empty = local)")
	fs.StringVar(&t.user, "user", os.Getenv("USER"), "remote user to connect as (ignored for local)")
	binary := fs.String("binary", "", "path to the local claude-agent binary (default: sibling of claude-host)")
	shellHost := fs.String("shell-host", "", "address of the claude-shell status listener (required)")
	etcDir := fs.String("shell-etc-dir", "/etc/claude-shell", "local path to the claude-shell peering directory")
	imageTag := fs.String("image-tag", "claude-shell:"+Version, "container image tag to push to the agent")
	caCert := fs.String("ca-cert", "", "override path to the rsyslog CA cert (default: <shell-etc-dir>/rsyslog-ca/ca.crt)")
	caKey := fs.String("ca-key", "", "override path to the rsyslog CA key (default: <shell-etc-dir>/rsyslog-ca/ca.key)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireLocalRoot(t); err != nil {
		return err
	}

	ctx, cancel := signalContext()
	defer cancel()

	r, sshCfg, err := runnerFor(t)
	if err != nil {
		return err
	}
	defer r.Close()

	return hostinstall.InitAgent(ctx, r, sshCfg, hostinstall.InitAgentOptions{
		BinaryPath:       *binary,
		ShellHost:        *shellHost,
		LocalShellEtcDir: *etcDir,
		ImageTag:         *imageTag,
		CACertPath:       *caCert,
		CAKeyPath:        *caKey,
	}, os.Stderr)
}

func cmdMigrateSession(args []string) error {
	fs := flag.NewFlagSet("migrate-session", flag.ContinueOnError)
	agent := fs.String("agent", "", "target agent ID (must be registered under /etc/claude-shell/agent-keys/)")
	session := fs.String("session", "", "session UUID to migrate (directory under /home/claude)")
	base := fs.String("shell-base", "/home/claude", "local directory holding orphan session dirs")
	keysDir := fs.String("agent-keys-dir", "/etc/claude-shell/agent-keys", "per-agent key + agent-host directory")
	deleteSrc := fs.Bool("delete-source", false, "rm the local session dir after a successful transfer")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, cancel := signalContext()
	defer cancel()

	return hostinstall.MigrateSession(ctx, hostinstall.MigrateSessionOptions{
		AgentID:           *agent,
		SessionUUID:       *session,
		ShellSessionsBase: *base,
		AgentKeysDir:      *keysDir,
		DeleteSource:      *deleteSrc,
	}, os.Stderr)
}

func cmdUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	var t targetFlags
	fs.StringVar(&t.host, "host", "", "remote host to update (empty = local)")
	fs.StringVar(&t.user, "user", os.Getenv("USER"), "remote user to connect as (ignored for local)")
	shellBin := fs.String("shell-binary", "", "path to new claude-shell binary (default: sibling of claude-host)")
	agentBin := fs.String("agent-binary", "", "path to new claude-agent binary (default: sibling of claude-host)")
	imageTag := fs.String("image-tag", "claude-shell:"+Version, "container image to push to the agent (empty disables the push)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireLocalRoot(t); err != nil {
		return err
	}

	ctx, cancel := signalContext()
	defer cancel()

	r, sshCfg, err := runnerFor(t)
	if err != nil {
		return err
	}
	defer r.Close()

	return hostinstall.Update(ctx, r, sshCfg, hostinstall.UpdateOptions{
		ShellBinaryPath: *shellBin,
		AgentBinaryPath: *agentBin,
		ImageTag:        *imageTag,
	}, os.Stderr)
}

func describeTarget(t targetFlags) string {
	if t.host == "" {
		return "local"
	}
	return fmt.Sprintf("%s@%s", t.user, t.host)
}

// runnerFor returns a Runner connected to t. For local targets it returns a
// LocalRunner and a nil SSHConfig; for remote targets it dials SSH and
// returns both the runner and the SSHConfig so callers can reconnect after
// a reboot. The caller is responsible for calling Close() on the runner.
func runnerFor(t targetFlags) (hostinstall.Runner, *hostinstall.SSHConfig, error) {
	if t.host == "" {
		return hostinstall.NewLocalRunner(), nil, nil
	}
	cfg := hostinstall.SSHConfig{
		Host: t.host,
		User: t.user,
	}
	r, err := hostinstall.NewSSHRunner(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to %s: %w", describeTarget(t), err)
	}
	return r, &cfg, nil
}

// signalContext returns a context that is canceled on SIGINT or SIGTERM so a
// Ctrl-C during a long-running step aborts cleanly rather than leaving the
// remote mid-operation.
func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()
	return ctx, cancel
}

func printUsage() {
	fmt.Printf(`%s - Provision a host for claude-shell / claude-agent

Usage:
  %s install         [--user U --host H]        Prepare a vanilla Ubuntu host.
  %s init-shell      --host H [--user U]        Deploy claude-shell to target.
  %s init-agent      --host H [--user U]        Deploy claude-agent to target.
  %s update          --host H [--user U]        Update installed services on target.
  %s migrate-session --agent A --session UUID   Move a local orphan session to an agent.
  %s version                                    Print version.
  %s help                                       Show this message.

Authentication:
  Local: run with sudo. Remote: connecting user must have NOPASSWD sudo.
`, appName, appName, appName, appName, appName, appName, appName, appName)
}
