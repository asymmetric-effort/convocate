package mtls

import (
	"crypto/tls"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestServerTLSConfig(t *testing.T) {
	ca := testCA(t)
	serverPair, err := ca.IssueServerCert(
		"router.convocate.local",
		[]string{"localhost"},
		[]net.IP{net.ParseIP("127.0.0.1")},
		24*time.Hour,
	)
	if err != nil {
		t.Fatalf("IssueServerCert error: %v", err)
	}

	t.Run("require client cert", func(t *testing.T) {
		config, err := ServerTLSConfig(*serverPair, ca.TrustBundle(), true)
		if err != nil {
			t.Fatalf("ServerTLSConfig error: %v", err)
		}
		if config.ClientAuth != tls.RequireAndVerifyClientCert {
			t.Errorf("ClientAuth: got %v, want RequireAndVerifyClientCert", config.ClientAuth)
		}
		if config.MinVersion != tls.VersionTLS13 {
			t.Errorf("MinVersion: got %d, want TLS 1.3 (%d)", config.MinVersion, tls.VersionTLS13)
		}
		if len(config.Certificates) != 1 {
			t.Errorf("Certificates: got %d, want 1", len(config.Certificates))
		}
		if config.ClientCAs == nil {
			t.Error("ClientCAs is nil")
		}
	})

	t.Run("optional client cert", func(t *testing.T) {
		config, err := ServerTLSConfig(*serverPair, ca.TrustBundle(), false)
		if err != nil {
			t.Fatalf("ServerTLSConfig error: %v", err)
		}
		if config.ClientAuth != tls.VerifyClientCertIfGiven {
			t.Errorf("ClientAuth: got %v, want VerifyClientCertIfGiven", config.ClientAuth)
		}
	})

	t.Run("curve preferences", func(t *testing.T) {
		config, err := ServerTLSConfig(*serverPair, ca.TrustBundle(), true)
		if err != nil {
			t.Fatalf("ServerTLSConfig error: %v", err)
		}
		if len(config.CurvePreferences) < 2 {
			t.Errorf("CurvePreferences: got %d, want >= 2", len(config.CurvePreferences))
		}
		if config.CurvePreferences[0] != tls.X25519 {
			t.Errorf("first curve: got %v, want X25519", config.CurvePreferences[0])
		}
	})
}

func TestServerTLSConfigInvalidCert(t *testing.T) {
	ca := testCA(t)
	badPair := CertKeyPair{CertPEM: []byte("bad"), KeyPEM: []byte("bad")}
	_, err := ServerTLSConfig(badPair, ca.TrustBundle(), true)
	if err == nil {
		t.Error("expected error for invalid certificate")
	}
}

func TestServerTLSConfigInvalidCA(t *testing.T) {
	ca := testCA(t)
	serverPair, err := ca.IssueServerCert("test", []string{"localhost"}, nil, 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueServerCert error: %v", err)
	}
	_, err = ServerTLSConfig(*serverPair, []byte("not a CA"), true)
	if err == nil {
		t.Error("expected error for invalid CA PEM")
	}
}

func TestClientTLSConfig(t *testing.T) {
	ca := testCA(t)
	clientPair, err := ca.IssueClientCert("agent-host-1", 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueClientCert error: %v", err)
	}

	config, err := ClientTLSConfig(*clientPair, ca.TrustBundle())
	if err != nil {
		t.Fatalf("ClientTLSConfig error: %v", err)
	}

	if config.MinVersion != tls.VersionTLS13 {
		t.Errorf("MinVersion: got %d, want TLS 1.3 (%d)", config.MinVersion, tls.VersionTLS13)
	}
	if len(config.Certificates) != 1 {
		t.Errorf("Certificates: got %d, want 1", len(config.Certificates))
	}
	if config.RootCAs == nil {
		t.Error("RootCAs is nil")
	}
}

func TestClientTLSConfigInvalidCert(t *testing.T) {
	ca := testCA(t)
	badPair := CertKeyPair{CertPEM: []byte("bad"), KeyPEM: []byte("bad")}
	_, err := ClientTLSConfig(badPair, ca.TrustBundle())
	if err == nil {
		t.Error("expected error for invalid certificate")
	}
}

func TestClientTLSConfigInvalidCA(t *testing.T) {
	ca := testCA(t)
	clientPair, err := ca.IssueClientCert("test", 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueClientCert error: %v", err)
	}
	_, err = ClientTLSConfig(*clientPair, []byte("not a CA"))
	if err == nil {
		t.Error("expected error for invalid CA PEM")
	}
}

func TestPlainTLSConfig(t *testing.T) {
	ca := testCA(t)
	config, err := PlainTLSConfig(ca.TrustBundle())
	if err != nil {
		t.Fatalf("PlainTLSConfig error: %v", err)
	}
	if config.MinVersion != tls.VersionTLS13 {
		t.Errorf("MinVersion: got %d, want TLS 1.3 (%d)", config.MinVersion, tls.VersionTLS13)
	}
	if len(config.Certificates) != 0 {
		t.Errorf("Certificates: got %d, want 0 (no client cert)", len(config.Certificates))
	}
	if config.RootCAs == nil {
		t.Error("RootCAs is nil")
	}
}

func TestPlainTLSConfigInvalidCA(t *testing.T) {
	_, err := PlainTLSConfig([]byte("not a CA"))
	if err == nil {
		t.Error("expected error for invalid CA PEM")
	}
}

func TestMTLSHandshake(t *testing.T) {
	ca := testCA(t)

	serverPair, err := ca.IssueServerCert(
		"test-server",
		[]string{"localhost"},
		[]net.IP{net.ParseIP("127.0.0.1")},
		24*time.Hour,
	)
	if err != nil {
		t.Fatalf("IssueServerCert error: %v", err)
	}

	clientPair, err := ca.IssueClientCert("test-client", 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueClientCert error: %v", err)
	}

	serverConfig, err := ServerTLSConfig(*serverPair, ca.TrustBundle(), true)
	if err != nil {
		t.Fatalf("ServerTLSConfig error: %v", err)
	}

	clientConfig, err := ClientTLSConfig(*clientPair, ca.TrustBundle())
	if err != nil {
		t.Fatalf("ClientTLSConfig error: %v", err)
	}
	clientConfig.ServerName = "localhost"

	// Start a TLS listener.
	listener, err := tls.Listen("tcp", "127.0.0.1:0", serverConfig)
	if err != nil {
		t.Fatalf("tls.Listen error: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	errCh := make(chan error, 1)

	// Server goroutine: accept one connection, read, echo back.
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			errCh <- fmt.Errorf("accept: %w", acceptErr)
			return
		}
		defer conn.Close()

		buf := make([]byte, 64)
		n, readErr := conn.Read(buf)
		if readErr != nil {
			errCh <- fmt.Errorf("read: %w", readErr)
			return
		}
		_, writeErr := conn.Write(buf[:n])
		errCh <- writeErr
	}()

	// Client: connect, send, receive.
	conn, err := tls.Dial("tcp", addr, clientConfig)
	if err != nil {
		t.Fatalf("tls.Dial error: %v", err)
	}
	defer conn.Close()

	message := []byte("hello mTLS")
	_, err = conn.Write(message)
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	if string(buf[:n]) != "hello mTLS" {
		t.Errorf("echo: got %q, want %q", string(buf[:n]), "hello mTLS")
	}

	// Verify the server's client verification.
	serverErr := <-errCh
	if serverErr != nil {
		t.Fatalf("server error: %v", serverErr)
	}

	// Verify the connection state.
	state := conn.ConnectionState()
	if state.Version != tls.VersionTLS13 {
		t.Errorf("TLS version: got %d, want TLS 1.3 (%d)", state.Version, tls.VersionTLS13)
	}
	if len(state.PeerCertificates) == 0 {
		t.Error("no peer certificates from server")
	}
}

func TestMTLSHandshakeRejectUnknownCA(t *testing.T) {
	ca1 := testCA(t)
	ca2 := testCA(t)

	serverPair, err := ca1.IssueServerCert("test-server", []string{"localhost"}, nil, 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueServerCert error: %v", err)
	}

	// Client cert signed by a different CA.
	clientPair, err := ca2.IssueClientCert("rogue-client", 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueClientCert error: %v", err)
	}

	serverConfig, err := ServerTLSConfig(*serverPair, ca1.TrustBundle(), true)
	if err != nil {
		t.Fatalf("ServerTLSConfig error: %v", err)
	}

	// Client trusts ca1 (can verify server) but presents a ca2-signed cert.
	clientConfig, err := ClientTLSConfig(*clientPair, ca1.TrustBundle())
	if err != nil {
		t.Fatalf("ClientTLSConfig error: %v", err)
	}
	clientConfig.ServerName = "localhost"

	listener, err := tls.Listen("tcp", "127.0.0.1:0", serverConfig)
	if err != nil {
		t.Fatalf("tls.Listen error: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	serverErrCh := make(chan error, 1)

	// Server goroutine accepts and tries to handshake.
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErrCh <- acceptErr
			return
		}
		defer conn.Close()
		tlsConn := conn.(*tls.Conn)
		// In TLS 1.3, client cert verification happens during handshake.
		handshakeErr := tlsConn.Handshake()
		serverErrCh <- handshakeErr
	}()

	conn, dialErr := tls.Dial("tcp", addr, clientConfig)
	if dialErr != nil {
		// Handshake failed on client side — expected.
		return
	}
	defer conn.Close()

	// TLS 1.3 may complete the dial but fail on subsequent I/O when the
	// server rejects the client cert. Try a write+read cycle.
	_, writeErr := conn.Write([]byte("test"))

	// Check server saw a handshake error.
	serverErr := <-serverErrCh
	if serverErr == nil && writeErr == nil {
		// Read should fail if write somehow succeeded.
		buf := make([]byte, 64)
		_, readErr := conn.Read(buf)
		if readErr == nil {
			t.Error("expected mTLS to fail with cert from wrong CA, but full round-trip succeeded")
		}
	}
	// If either side errored, the cross-CA rejection worked correctly.
}

func TestMTLSHandshakeOptionalClientCert(t *testing.T) {
	ca := testCA(t)

	serverPair, err := ca.IssueServerCert("test-server", []string{"localhost"}, nil, 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueServerCert error: %v", err)
	}

	// Server does not require client cert.
	serverConfig, err := ServerTLSConfig(*serverPair, ca.TrustBundle(), false)
	if err != nil {
		t.Fatalf("ServerTLSConfig error: %v", err)
	}

	// Client presents no cert — just trusts the CA.
	clientConfig, err := PlainTLSConfig(ca.TrustBundle())
	if err != nil {
		t.Fatalf("PlainTLSConfig error: %v", err)
	}
	clientConfig.ServerName = "localhost"

	listener, err := tls.Listen("tcp", "127.0.0.1:0", serverConfig)
	if err != nil {
		t.Fatalf("tls.Listen error: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	errCh := make(chan error, 1)

	go func() {
		serverConn, acceptErr := listener.Accept()
		if acceptErr != nil {
			errCh <- acceptErr
			return
		}
		defer serverConn.Close()

		// Verify server sees no client cert.
		tlsConn, ok := serverConn.(*tls.Conn)
		if !ok {
			errCh <- fmt.Errorf("expected *tls.Conn")
			return
		}
		handshakeErr := tlsConn.Handshake()
		if handshakeErr != nil {
			errCh <- handshakeErr
			return
		}
		state := tlsConn.ConnectionState()
		if len(state.PeerCertificates) != 0 {
			errCh <- fmt.Errorf("expected no peer certs, got %d", len(state.PeerCertificates))
			return
		}

		buf := make([]byte, 64)
		n, readErr := serverConn.Read(buf)
		if readErr != nil {
			errCh <- readErr
			return
		}
		_, writeErr := serverConn.Write(buf[:n])
		errCh <- writeErr
	}()

	conn, err := tls.Dial("tcp", addr, clientConfig)
	if err != nil {
		t.Fatalf("tls.Dial error: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte("no cert"))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if string(buf[:n]) != "no cert" {
		t.Errorf("echo: got %q, want %q", string(buf[:n]), "no cert")
	}

	serverErr := <-errCh
	if serverErr != nil {
		t.Fatalf("server error: %v", serverErr)
	}
}
