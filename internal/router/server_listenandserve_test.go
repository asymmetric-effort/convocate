package router

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/mtls"
	"github.com/asymmetric-effort/convocate/internal/openbao"
	"github.com/asymmetric-effort/convocate/internal/protocol"
	redispkg "github.com/asymmetric-effort/convocate/internal/redis"
)

func TestListenAndServe(t *testing.T) {
	// Generate CA and certs.
	ca, err := mtls.GenerateCA("test-ca", 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	serverPair, err := ca.IssueServerCert("localhost",
		[]string{"localhost"}, []net.IP{net.ParseIP("127.0.0.1")}, 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueServerCert: %v", err)
	}
	// Each listener needs its own TLS config to avoid data race in ServeTLS.
	publicServerTLS, err := mtls.ServerTLSConfig(*serverPair, ca.TrustBundle(), false)
	if err != nil {
		t.Fatalf("ServerTLSConfig public: %v", err)
	}
	internalServerTLS, err := mtls.ServerTLSConfig(*serverPair, ca.TrustBundle(), false)
	if err != nil {
		t.Fatalf("ServerTLSConfig internal: %v", err)
	}
	clientTLS, err := mtls.PlainTLSConfig(ca.TrustBundle())
	if err != nil {
		t.Fatalf("PlainTLSConfig: %v", err)
	}
	clientTLS.ServerName = "localhost"

	// Create two TCP listeners.
	publicLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen public: %v", err)
	}
	internalLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen internal: %v", err)
	}

	publicAddr := publicLn.Addr().String()
	internalAddr := internalLn.Addr().String()

	// Create server.
	mockConn := redispkg.NewMockConn()
	store := redispkg.NewRouterStore(mockConn)
	baoClient := openbao.NewClient(openbao.Config{Address: "http://localhost:1", Token: "test"})
	srv := NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test-las",
		Logger:  log.New(io.Discard, "", 0),
	})

	// Start ListenAndServe in background.
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(publicLn, internalLn, publicServerTLS, internalServerTLS)
	}()

	// Give servers time to start.
	time.Sleep(50 * time.Millisecond)

	httpClient := &http.Client{
		Transport: &http.Transport{TLSClientConfig: clientTLS},
		Timeout:   5 * time.Second,
	}

	// Test public listener (should serve /v1/health).
	t.Run("public health", func(t *testing.T) {
		req, reqErr := http.NewRequestWithContext(context.Background(), "GET",
			"https://"+publicAddr+"/v1/health", http.NoBody)
		if reqErr != nil {
			t.Fatalf("new request: %v", reqErr)
		}
		resp, reqErr := httpClient.Do(req)
		if reqErr != nil {
			t.Fatalf("GET public /v1/health: %v", reqErr)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("public health status: got %d, want 200", resp.StatusCode)
		}
		var health protocol.HealthResponse
		json.NewDecoder(resp.Body).Decode(&health)
		if health.Status != "ok" {
			t.Errorf("health status: got %q, want ok", health.Status)
		}
	})

	// Test internal listener (should serve /v1/health and other endpoints).
	t.Run("internal health", func(t *testing.T) {
		req, reqErr := http.NewRequestWithContext(context.Background(), "GET",
			"https://"+internalAddr+"/v1/health", http.NoBody)
		if reqErr != nil {
			t.Fatalf("new request: %v", reqErr)
		}
		resp, reqErr := httpClient.Do(req)
		if reqErr != nil {
			t.Fatalf("GET internal /v1/health: %v", reqErr)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("internal health status: got %d, want 200", resp.StatusCode)
		}
	})

	// Clean up by closing listeners (causes ServeTLS to return).
	publicLn.Close()
	internalLn.Close()

	select {
	case <-errCh:
		// Expected error from closed listener.
	case <-time.After(2 * time.Second):
		t.Error("ListenAndServe did not return after closing listeners")
	}
}

// TestListenAndServeTLSOnly verifies the listeners use the provided TLS configs.
func TestListenAndServeTLSOnly(t *testing.T) {
	ca, err := mtls.GenerateCA("test-ca-2", 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	serverPair, err := ca.IssueServerCert("localhost",
		[]string{"localhost"}, []net.IP{net.ParseIP("127.0.0.1")}, 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueServerCert: %v", err)
	}

	publicTLS, err := mtls.ServerTLSConfig(*serverPair, ca.TrustBundle(), false)
	if err != nil {
		t.Fatalf("ServerTLSConfig public: %v", err)
	}
	internalTLS, err := mtls.ServerTLSConfig(*serverPair, ca.TrustBundle(), false)
	if err != nil {
		t.Fatalf("ServerTLSConfig internal: %v", err)
	}

	publicLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	internalLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}

	mockConn := redispkg.NewMockConn()
	store := redispkg.NewRouterStore(mockConn)
	baoClient := openbao.NewClient(openbao.Config{Address: "http://localhost:1", Token: "test"})
	srv := NewServer(Config{
		Store:   store,
		Bao:     baoClient,
		Version: "test",
		Logger:  log.New(io.Discard, "", 0),
	})

	go func() {
		_ = srv.ListenAndServe(publicLn, internalLn, publicTLS, internalTLS)
	}()
	time.Sleep(50 * time.Millisecond)

	// Try connecting without TLS trust - should fail.
	plainClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
		Timeout: 1 * time.Second,
	}
	req, _ := http.NewRequestWithContext(context.Background(), "GET",
		"https://"+publicLn.Addr().String()+"/v1/health", http.NoBody)
	resp, err := plainClient.Do(req)
	if err == nil {
		resp.Body.Close()
		t.Error("expected TLS error when connecting without trust")
	}

	publicLn.Close()
	internalLn.Close()
}
