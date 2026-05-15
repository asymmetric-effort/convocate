package redis

import (
	"crypto/tls"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/mtls"
)

// startFakeRedis starts a TLS server that speaks minimal RESP3.
// It responds to PING with +PONG, SET with +OK, GET with $-1 (nil bulk).
func startFakeRedis(t *testing.T, tlsConfig *tls.Config) net.Listener {
	t.Helper()
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("tls.Listen: %v", err)
	}
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go handleFakeRedisConn(conn)
		}
	}()
	return listener
}

func handleFakeRedisConn(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		data := string(buf[:n])
		// Simple command detection by looking for the command name.
		switch {
		case containsRESPCommand(data, "PING"):
			_, _ = conn.Write([]byte("+PONG\r\n"))
		case containsRESPCommand(data, "SET"):
			_, _ = conn.Write([]byte("+OK\r\n"))
		case containsRESPCommand(data, "GET"):
			_, _ = conn.Write([]byte("$-1\r\n"))
		default:
			_, _ = conn.Write([]byte("-ERR unknown command\r\n"))
		}
	}
}

func containsRESPCommand(data, cmd string) bool {
	// In RESP3, commands are sent as bulk strings within an array.
	// The command name appears as $<len>\r\n<cmd>\r\n
	needle := fmt.Sprintf("$%d\r\n%s\r\n", len(cmd), cmd)
	return len(data) >= len(needle) && contains(data, needle)
}

func contains(s, substr string) bool {
	for i := range len(s) - len(substr) + 1 {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func testTLSCerts(t *testing.T) (serverConfig, clientConfig *tls.Config) {
	t.Helper()
	ca, err := mtls.GenerateCA("test-redis-ca", 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	serverPair, err := ca.IssueServerCert("localhost",
		[]string{"localhost"}, []net.IP{net.ParseIP("127.0.0.1")}, 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueServerCert: %v", err)
	}
	serverConfig, err = mtls.ServerTLSConfig(*serverPair, ca.TrustBundle(), false)
	if err != nil {
		t.Fatalf("ServerTLSConfig: %v", err)
	}
	clientConfig, err = mtls.PlainTLSConfig(ca.TrustBundle())
	if err != nil {
		t.Fatalf("PlainTLSConfig: %v", err)
	}
	clientConfig.ServerName = "localhost"
	return serverConfig, clientConfig
}

func TestDialAndDo(t *testing.T) {
	serverTLS, clientTLS := testTLSCerts(t)
	listener := startFakeRedis(t, serverTLS)
	defer listener.Close()

	addr := listener.Addr().String()

	t.Run("Dial success", func(t *testing.T) {
		conn, err := Dial(ConnConfig{
			Address:   addr,
			TLSConfig: clientTLS,
			Timeout:   5 * time.Second,
		})
		if err != nil {
			t.Fatalf("Dial: %v", err)
		}
		defer conn.Close()

		// PING
		val, err := conn.Do("PING")
		if err != nil {
			t.Fatalf("Do PING: %v", err)
		}
		if val != "PONG" {
			t.Errorf("PING: got %v, want PONG", val)
		}

		// SET
		val, err = conn.Do("SET", "key", "value")
		if err != nil {
			t.Fatalf("Do SET: %v", err)
		}
		if val != "OK" {
			t.Errorf("SET: got %v, want OK", val)
		}

		// GET (returns nil bulk)
		val, err = conn.Do("GET", "key")
		if err != nil {
			t.Fatalf("Do GET: %v", err)
		}
		if val != nil {
			t.Errorf("GET: got %v, want nil", val)
		}
	})

	t.Run("Dial with default timeout", func(t *testing.T) {
		conn, err := Dial(ConnConfig{
			Address:   addr,
			TLSConfig: clientTLS,
		})
		if err != nil {
			t.Fatalf("Dial: %v", err)
		}
		conn.Close()
	})

	t.Run("Dial with nil TLS config", func(t *testing.T) {
		_, err := Dial(ConnConfig{
			Address: addr,
		})
		if err == nil {
			t.Error("expected error for nil TLS config")
		}
	})

	t.Run("Dial bad address", func(t *testing.T) {
		_, err := Dial(ConnConfig{
			Address:   "127.0.0.1:1",
			TLSConfig: clientTLS,
			Timeout:   500 * time.Millisecond,
		})
		if err == nil {
			t.Error("expected error for bad address")
		}
	})

	t.Run("Do on closed connection", func(t *testing.T) {
		conn, err := Dial(ConnConfig{
			Address:   addr,
			TLSConfig: clientTLS,
			Timeout:   5 * time.Second,
		})
		if err != nil {
			t.Fatalf("Dial: %v", err)
		}
		conn.Close()
		_, err = conn.Do("PING")
		if err == nil {
			t.Error("expected error on closed connection")
		}
	})

	t.Run("TLS min version set automatically", func(t *testing.T) {
		// Create a TLS config without MinVersion set.
		tlsCfg := clientTLS.Clone()
		tlsCfg.MinVersion = 0

		conn, err := Dial(ConnConfig{
			Address:   addr,
			TLSConfig: tlsCfg,
			Timeout:   5 * time.Second,
		})
		if err != nil {
			t.Fatalf("Dial: %v", err)
		}
		conn.Close()
	})
}
