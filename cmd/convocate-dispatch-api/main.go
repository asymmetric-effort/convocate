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
	fmt.Fprintf(os.Stderr, "convocate-dispatch-api %s\n", Version)
	os.Exit(run())
}

func run() int {
	logger := log.New(os.Stderr, "dispatch: ", log.LstdFlags)

	hostID := os.Getenv("CONVOCATE_HOST_ID")
	controlURL := os.Getenv("CONVOCATE_CONTROL_URL")

	logger.Printf("host=%s control=%s", hostID, controlURL)
	logger.Println("waiting for dispatch events...")

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	logger.Println("shutting down")
	return 0
}
