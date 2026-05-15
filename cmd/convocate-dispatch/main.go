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
	fmt.Fprintf(os.Stderr, "convocate-dispatch %s\n", Version)
	os.Exit(run())
}

func run() int {
	// Per-host dispatch executor entrypoint.
	// Implementation lands in Phase 7.
	fmt.Fprintln(os.Stderr, "not yet implemented")
	return 1
}
