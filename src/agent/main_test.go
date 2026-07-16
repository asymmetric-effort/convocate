package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSignalChannel(t *testing.T) {
	ch := signalChannel()
	if ch == nil {
		t.Fatal("signalChannel returned nil")
	}
}

func TestEnvOr_WithValue(t *testing.T) {
	t.Setenv("TEST_ENVOR_EXISTS", "hello")
	got := envOr("TEST_ENVOR_EXISTS", "default")
	if got != "hello" {
		t.Errorf("envOr = %q, want %q", got, "hello")
	}
}

func TestEnvOr_Empty(t *testing.T) {
	os.Unsetenv("TEST_ENVOR_MISSING_12345")
	got := envOr("TEST_ENVOR_MISSING_12345", "fallback")
	if got != "fallback" {
		t.Errorf("envOr = %q, want %q", got, "fallback")
	}
}

func TestFileExists_RegularFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "testfile")
	os.WriteFile(p, []byte("data"), 0644)
	if !fileExists(p) {
		t.Error("fileExists should return true for an existing regular file")
	}
}

func TestFileExists_Directory(t *testing.T) {
	dir := t.TempDir()
	if fileExists(dir) {
		t.Error("fileExists should return false for a directory")
	}
}

func TestFileExists_Nonexistent(t *testing.T) {
	if fileExists("/nonexistent/file/xyz") {
		t.Error("fileExists should return false for nonexistent path")
	}
}

// ---------------------------------------------------------------------------
// parseAgentConfig
// ---------------------------------------------------------------------------

func TestParseAgentConfig_Defaults(t *testing.T) {
	// Clear all relevant env vars
	for _, k := range []string{"TLS_CERT_PATH", "TLS_KEY_PATH", "JWT_PUBLIC_KEY_PATH",
		"CLAUDE_FLAGS", "CLAUDE_MD_PATH", "WORK_DIR", "POD_NAME", "NODE_NAME",
		"LISTEN_ADDR", "SA_TOKEN_PATH"} {
		t.Setenv(k, "")
	}

	cfg := parseAgentConfig()
	if cfg.podName != "unknown" {
		t.Errorf("podName = %q, want %q", cfg.podName, "unknown")
	}
	if cfg.nodeName != "unknown" {
		t.Errorf("nodeName = %q, want %q", cfg.nodeName, "unknown")
	}
	if cfg.listenAddr != ":8443" {
		t.Errorf("listenAddr = %q, want %q", cfg.listenAddr, ":8443")
	}
	if len(cfg.claudeFlags) != 0 {
		t.Errorf("claudeFlags = %v, want empty", cfg.claudeFlags)
	}
}

func TestParseAgentConfig_Custom(t *testing.T) {
	t.Setenv("POD_NAME", "my-pod")
	t.Setenv("NODE_NAME", "my-node")
	t.Setenv("LISTEN_ADDR", ":9999")
	t.Setenv("CLAUDE_FLAGS", "--flag1 --flag2")

	cfg := parseAgentConfig()
	if cfg.podName != "my-pod" {
		t.Errorf("podName = %q, want %q", cfg.podName, "my-pod")
	}
	if cfg.nodeName != "my-node" {
		t.Errorf("nodeName = %q, want %q", cfg.nodeName, "my-node")
	}
	if len(cfg.claudeFlags) != 2 {
		t.Errorf("claudeFlags = %v, want 2 items", cfg.claudeFlags)
	}
}

// ---------------------------------------------------------------------------
// configureHTTPServer
// ---------------------------------------------------------------------------

func TestConfigureHTTPServer_NoTLS(t *testing.T) {
	cfg := agentConfig{
		certPath:   "/nonexistent/cert",
		keyPath:    "/nonexistent/key",
		listenAddr: ":0",
	}
	mux := http.NewServeMux()
	srv, err := configureHTTPServer(cfg, mux)
	if err != nil {
		t.Fatalf("configureHTTPServer: %v", err)
	}
	if srv.TLSConfig != nil {
		t.Error("TLSConfig should be nil when cert files don't exist")
	}
	if srv.Addr != ":0" {
		t.Errorf("Addr = %q, want %q", srv.Addr, ":0")
	}
}

func TestConfigureHTTPServer_InvalidCerts(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")
	os.WriteFile(certPath, []byte("not a cert"), 0644)
	os.WriteFile(keyPath, []byte("not a key"), 0644)

	cfg := agentConfig{
		certPath:   certPath,
		keyPath:    keyPath,
		listenAddr: ":0",
	}
	mux := http.NewServeMux()
	_, err := configureHTTPServer(cfg, mux)
	if err == nil {
		t.Error("expected error for invalid cert/key files")
	}
}


// ---------------------------------------------------------------------------
// startHTTPServer
// ---------------------------------------------------------------------------

func TestStartHTTPServer_PlainHTTP(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: mux,
	}

	cfg := agentConfig{listenAddr: "127.0.0.1:0"}
	startHTTPServer(srv, cfg)

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Shut it down
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func TestStartHTTPServer_TLSBranch(t *testing.T) {
	// Generate a self-signed cert using the Go std library
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, _ := x509.MarshalECPrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}

	cfg := agentConfig{listenAddr: "127.0.0.1:0"}
	startHTTPServer(srv, cfg)

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

// ---------------------------------------------------------------------------
// buildServer
// ---------------------------------------------------------------------------

func TestBuildServer(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no claude binary
	dir := t.TempDir()
	cfg := agentConfig{
		certPath:     "/nonexistent",
		keyPath:      "/nonexistent",
		jwtKeyPath:   "",
		claudeFlags:  nil,
		claudeMdPath: filepath.Join(dir, "CLAUDE.md"),
		workDir:      dir,
		podName:      "test-pod",
		nodeName:     "test-node",
		listenAddr:   ":0",
		saTokenPath:  "",
	}

	m := NewMetrics()
	proc := &Process{
		metrics: m,
		done:    make(chan struct{}),
		flags:   nil,
		workDir: dir,
	}

	httpServer, watcher := buildServer(cfg, proc, m)
	if httpServer == nil {
		t.Fatal("httpServer should not be nil")
	}
	if watcher == nil {
		t.Fatal("watcher should not be nil")
	}
	watcher.Stop()
}

// ---------------------------------------------------------------------------
// shutdown
// ---------------------------------------------------------------------------

func TestShutdown(t *testing.T) {
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: mux,
	}

	dir := t.TempDir()
	mdPath := filepath.Join(dir, "CLAUDE.md")
	os.WriteFile(mdPath, []byte("test"), 0644)

	w := NewWatcher(mdPath, 100*time.Millisecond, func() {})
	go w.Start()
	time.Sleep(50 * time.Millisecond)

	m := NewMetrics()
	proc := &Process{
		metrics: m,
		done:    make(chan struct{}),
	}

	// Should complete without error
	shutdown(srv, w, proc)
}
