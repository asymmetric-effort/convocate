// Package main is the entry point for claude-host.
//
// claude-host provisions a vanilla Ubuntu machine to host claude-shell and/or
// claude-agent. It can run against the local machine (requiring local sudo)
// or against a remote host over SSH (requiring the remote user to have
// NOPASSWD sudoers privileges). Subcommands scaffolded here are filled in
// across subsequent commits.
package main

import (
	"flag"
	"fmt"
	"os"
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
	return fmt.Errorf("claude-host install: not yet implemented (target=%s)", describeTarget(t))
}

func cmdInitShell(args []string) error {
	t, err := parseTargetFlags("init-shell", args)
	if err != nil {
		return err
	}
	if err := requireLocalRoot(t); err != nil {
		return err
	}
	return fmt.Errorf("claude-host init-shell: not yet implemented (target=%s)", describeTarget(t))
}

func cmdInitAgent(args []string) error {
	t, err := parseTargetFlags("init-agent", args)
	if err != nil {
		return err
	}
	if err := requireLocalRoot(t); err != nil {
		return err
	}
	return fmt.Errorf("claude-host init-agent: not yet implemented (target=%s)", describeTarget(t))
}

func cmdUpdate(args []string) error {
	t, err := parseTargetFlags("update", args)
	if err != nil {
		return err
	}
	if err := requireLocalRoot(t); err != nil {
		return err
	}
	return fmt.Errorf("claude-host update: not yet implemented (target=%s)", describeTarget(t))
}

func describeTarget(t targetFlags) string {
	if t.host == "" {
		return "local"
	}
	return fmt.Sprintf("%s@%s", t.user, t.host)
}

func printUsage() {
	fmt.Printf(`%s - Provision a host for claude-shell / claude-agent

Usage:
  %s install     [--user U --host H]  Prepare a vanilla Ubuntu host.
  %s init-shell  --host H [--user U]  Deploy claude-shell to target.
  %s init-agent  --host H [--user U]  Deploy claude-agent to target.
  %s update      --host H [--user U]  Update installed services on target.
  %s version                          Print version.
  %s help                             Show this message.

Authentication:
  Local: run with sudo. Remote: connecting user must have NOPASSWD sudo.
`, appName, appName, appName, appName, appName, appName, appName)
}
