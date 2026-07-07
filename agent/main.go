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
	run(signalChannel())
}

// signalChannel creates a channel that receives SIGTERM and SIGINT.
func signalChannel() <-chan os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	return ch
}

// agentConfig holds parsed environment configuration for the agent wrapper.
type agentConfig struct {
	certPath    string
	keyPath     string
	jwtKeyPath  string
	claudeFlags []string
	claudeMdPath string
	workDir     string
	podName     string
	nodeName    string
	listenAddr  string
	saTokenPath string
}

// parseAgentConfig reads environment variables and returns configuration.
func parseAgentConfig() agentConfig {
	return agentConfig{
		certPath:     envOr("TLS_CERT_PATH", "/etc/tls/tls.crt"),
		keyPath:      envOr("TLS_KEY_PATH", "/etc/tls/tls.key"),
		jwtKeyPath:   envOr("JWT_PUBLIC_KEY_PATH", ""),
		claudeFlags:  strings.Fields(envOr("CLAUDE_FLAGS", "")),
		claudeMdPath: envOr("CLAUDE_MD_PATH", "/home/claude/CLAUDE.md"),
		workDir:      envOr("WORK_DIR", "/home/claude/workspace"),
		podName:      envOr("POD_NAME", "unknown"),
		nodeName:     envOr("NODE_NAME", "unknown"),
		listenAddr:   envOr("LISTEN_ADDR", ":8443"),
		saTokenPath:  envOr("SA_TOKEN_PATH", "/var/run/secrets/kubernetes.io/serviceaccount/token"),
	}
}

// buildServer creates the HTTP server components from the given config and process.
func buildServer(cfg agentConfig, proc *Process, metrics *Metrics) (*http.Server, *Watcher) {
	claudeVersion := DetectClaudeVersion()
	log.Printf("[wrapper] Claude CLI version: %s", claudeVersion)

	auth := NewAuth(cfg.jwtKeyPath, cfg.saTokenPath)

	watcher := NewWatcher(cfg.claudeMdPath, 500*time.Millisecond, func() {
		if restartErr := proc.Restart(cfg.claudeFlags); restartErr != nil {
			log.Printf("[wrapper] Restart on config change failed: %v", restartErr)
		}
	})

	server := NewServer(proc, metrics, auth, version, claudeVersion, cfg.podName, cfg.nodeName)
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	httpServer, err := configureHTTPServer(cfg, mux)
	if err != nil {
		log.Fatalf("[wrapper] Failed to configure HTTP server: %v", err)
	}
	return httpServer, watcher
}

// configureHTTPServer sets up TLS or plain HTTP based on cert availability.
func configureHTTPServer(cfg agentConfig, handler http.Handler) (*http.Server, error) {
	if fileExists(cfg.certPath) && fileExists(cfg.keyPath) {
		cert, tlsErr := tls.LoadX509KeyPair(cfg.certPath, cfg.keyPath)
		if tlsErr != nil {
			return nil, tlsErr
		}
		return &http.Server{
			Addr:    cfg.listenAddr,
			Handler: handler,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			},
		}, nil
	}
	return &http.Server{
		Addr:    cfg.listenAddr,
		Handler: handler,
	}, nil
}

// startHTTPServer starts the server in TLS or plain mode based on config.
func startHTTPServer(httpServer *http.Server, cfg agentConfig) {
	if httpServer.TLSConfig != nil {
		log.Printf("[wrapper] HTTPS server listening on %s", cfg.listenAddr)
	} else {
		log.Printf("[wrapper] HTTP server listening on %s (no TLS — dev mode)", cfg.listenAddr)
	}
	go serveHTTP(httpServer)
}

// serveHTTP runs the HTTP server (TLS or plain) until shutdown.
func serveHTTP(httpServer *http.Server) {
	var err error
	if httpServer.TLSConfig != nil {
		err = httpServer.ListenAndServeTLS("", "")
	} else {
		err = httpServer.ListenAndServe()
	}
	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("[wrapper] Server error: %v", err)
	}
}

// shutdown performs graceful shutdown of all components.
func shutdown(httpServer *http.Server, watcher *Watcher, proc *Process) {
	watcher.Stop()
	proc.Stop(25 * time.Second) // Stop always returns nil

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx) //nolint: shutdown errors are non-fatal

	log.Printf("[wrapper] Shutdown complete")
}

// run is the main entry point logic. If sigCh is nil, it creates one
// listening for SIGTERM/SIGINT.
func run(sigCh <-chan os.Signal) {
	cfg := parseAgentConfig()

	log.Printf("[wrapper] convocate-agent-wrapper %s starting", version)
	log.Printf("[wrapper] pod=%s node=%s flags=%v", cfg.podName, cfg.nodeName, cfg.claudeFlags)

	metrics := NewMetrics()

	proc, err := NewProcess(cfg.claudeFlags, cfg.workDir, metrics)
	if err != nil {
		log.Fatalf("[wrapper] Failed to start Claude CLI: %v", err)
	}

	httpServer, watcher := buildServer(cfg, proc, metrics)

	go func() {
		if watchErr := watcher.Start(); watchErr != nil {
			log.Printf("[wrapper] Watcher error: %v", watchErr)
		}
	}()

	startHTTPServer(httpServer, cfg)

	sig := <-sigCh
	log.Printf("[wrapper] Received %v, shutting down...", sig)

	shutdown(httpServer, watcher, proc)
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
