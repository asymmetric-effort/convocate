package main

import (
	"fmt"
	"os"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(Version)
		return
	}
	fmt.Fprintf(os.Stderr, "mock-claude %s\n", Version)
	os.Exit(run())
}

func run() int {
	// Deterministic Claude Code stand-in for e2e testing.
	// Implementation lands in Phase 14.
	fmt.Fprintln(os.Stderr, "not yet implemented")
	return 1
}
