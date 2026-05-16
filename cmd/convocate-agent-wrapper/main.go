package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(Version)
		return
	}
	fmt.Fprintf(os.Stderr, "convocate-agent-wrapper %s\n", Version)
	os.Exit(run())
}

func run() int {
	logger := log.New(os.Stderr, "agent-wrapper: ", log.LstdFlags)
	logger.Println("ready, waiting for background tasks via stdin...")

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	logger.Println("shutting down")
	return 0
}
