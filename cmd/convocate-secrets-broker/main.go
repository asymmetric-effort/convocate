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
	fmt.Fprintf(os.Stderr, "convocate-secrets-broker %s\n", Version)
	os.Exit(run())
}

func run() int {
	logger := log.New(os.Stderr, "secrets-broker: ", log.LstdFlags)

	hostID := os.Getenv("CONVOCATE_HOST_ID")
	logger.Printf("host=%s", hostID)
	logger.Println("ready, serving secrets sockets...")

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	logger.Println("shutting down")
	return 0
}
