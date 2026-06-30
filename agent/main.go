// convocate-agent-wrapper — manages a Claude CLI process inside a K8s pod.
//
// This binary is the ENTRYPOINT for the convocate/agent container. It:
//   - Spawns Claude CLI in a shell with configurable flags
//   - Exposes an HTTPS API for authenticated I/O relay and control
//   - Watches CLAUDE.md for ConfigMap changes and restarts Claude CLI
//   - Handles graceful shutdown on SIGTERM
//
// Environment variables:
//   TLS_CERT_PATH        — path to TLS certificate (default: /etc/tls/tls.crt)
//   TLS_KEY_PATH         — path to TLS private key (default: /etc/tls/tls.key)
//   JWT_PUBLIC_KEY_PATH  — path to JWT verification public key (default: empty = dev mode)
//   CLAUDE_FLAGS         — space-separated Claude CLI flags (default: empty)
//   CLAUDE_MD_PATH       — path to CLAUDE.md config file (default: /home/claude/CLAUDE.md)
//   WORK_DIR             — Claude CLI working directory (default: /home/claude/workspace)
//   POD_NAME             — K8s pod name (from downward API)
//   NODE_NAME            — K8s node name (from downward API)
//   LISTEN_ADDR          — server listen address (default: :8443)

package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	// Parse environment
	certPath := envOr("TLS_CERT_PATH", "/etc/tls/tls.crt")
	keyPath := envOr("TLS_KEY_PATH", "/etc/tls/tls.key")
	jwtKeyPath := envOr("JWT_PUBLIC_KEY_PATH", "")
	claudeFlags := strings.Fields(envOr("CLAUDE_FLAGS", ""))
	claudeMdPath := envOr("CLAUDE_MD_PATH", "/home/claude/CLAUDE.md")
	workDir := envOr("WORK_DIR", "/home/claude/workspace")
	podName := envOr("POD_NAME", "unknown")
	nodeName := envOr("NODE_NAME", "unknown")
	listenAddr := envOr("LISTEN_ADDR", ":8443")

	log.Printf("[wrapper] convocate-agent-wrapper %s starting", version)
	log.Printf("[wrapper] pod=%s node=%s flags=%v", podName, nodeName, claudeFlags)

	// Initialize metrics
	metrics := NewMetrics()

	// Detect Claude CLI version
	claudeVersion := DetectClaudeVersion()
	log.Printf("[wrapper] Claude CLI version: %s", claudeVersion)

	// Initialize auth
	auth := NewAuth(jwtKeyPath)

	// Spawn Claude CLI process
	proc, err := NewProcess(claudeFlags, workDir, metrics)
	if err != nil {
		log.Fatalf("[wrapper] Failed to start Claude CLI: %v", err)
	}

	// Start config watcher for CLAUDE.md
	watcher := NewWatcher(claudeMdPath, 500*time.Millisecond, func() {
		if restartErr := proc.Restart(claudeFlags); restartErr != nil {
			log.Printf("[wrapper] Restart on config change failed: %v", restartErr)
		}
	})
	go func() {
		if watchErr := watcher.Start(); watchErr != nil {
			log.Printf("[wrapper] Watcher error: %v", watchErr)
		}
	}()

	// Build HTTP server
	server := NewServer(proc, metrics, auth, version, claudeVersion, podName, nodeName)
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// Configure TLS
	var httpServer *http.Server
	if fileExists(certPath) && fileExists(keyPath) {
		cert, tlsErr := tls.LoadX509KeyPair(certPath, keyPath)
		if tlsErr != nil {
			log.Fatalf("[wrapper] Failed to load TLS cert: %v", tlsErr)
		}
		httpServer = &http.Server{
			Addr:    listenAddr,
			Handler: mux,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			},
		}
		log.Printf("[wrapper] HTTPS server listening on %s", listenAddr)
		go func() {
			if srvErr := httpServer.ListenAndServeTLS("", ""); srvErr != nil && srvErr != http.ErrServerClosed {
				log.Fatalf("[wrapper] Server error: %v", srvErr)
			}
		}()
	} else {
		// Dev mode: plain HTTP (no TLS certs available)
		httpServer = &http.Server{
			Addr:    listenAddr,
			Handler: mux,
		}
		log.Printf("[wrapper] HTTP server listening on %s (no TLS — dev mode)", listenAddr)
		go func() {
			if srvErr := httpServer.ListenAndServe(); srvErr != nil && srvErr != http.ErrServerClosed {
				log.Fatalf("[wrapper] Server error: %v", srvErr)
			}
		}()
	}

	// Wait for SIGTERM/SIGINT
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	log.Printf("[wrapper] Received %v, shutting down...", sig)

	// Stop watcher
	watcher.Stop()

	// Stop Claude CLI
	if stopErr := proc.Stop(25 * time.Second); stopErr != nil {
		log.Printf("[wrapper] Process stop error: %v", stopErr)
	}

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if shutErr := httpServer.Shutdown(ctx); shutErr != nil {
		log.Printf("[wrapper] Server shutdown error: %v", shutErr)
	}

	log.Printf("[wrapper] Shutdown complete")
}

// envOr returns the value of an environment variable or a fallback.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// fileExists returns true if the path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
