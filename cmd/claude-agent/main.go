// Package main is the entry point for claude-agent.
//
// claude-agent is a skeleton binary scaffolded so claude-host can deploy it
// alongside claude-shell. The install subcommand is a no-op placeholder; the
// real implementation will land in a later iteration.
package main

import (
	"fmt"
	"os"
)

const appName = "claude-agent"

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
			fmt.Printf("%s: install placeholder — not yet implemented.\n", appName)
			return nil
		case "version":
			fmt.Printf("%s version %s\n", appName, Version)
			return nil
		case "help", "--help", "-h":
			printUsage()
			return nil
		default:
			return fmt.Errorf("unknown command: %q (use 'help' for usage)", args[1])
		}
	}
	printUsage()
	return nil
}

func printUsage() {
	fmt.Printf(`%s - (skeleton) Claude agent service

Usage:
  %s install      Install and configure the claude-agent service
  %s version      Print version information
  %s help         Show this help message

Note: %s is currently a skeleton. The install step is a placeholder.
`, appName, appName, appName, appName, appName)
}
