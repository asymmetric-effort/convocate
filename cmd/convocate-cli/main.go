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
	fmt.Fprintf(os.Stderr, "convocate-cli %s\n", Version)
	os.Exit(run())
}

func run() int {
	// Operator admin CLI (ca print-bundle, host issue-cert, openbao init).
	// Implementation lands in Phase 9.
	fmt.Fprintln(os.Stderr, "not yet implemented")
	return 1
}
