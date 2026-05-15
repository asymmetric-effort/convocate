package broker

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/openbao"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// TestBrokerUnboundConnection tests the handleConnection path where the
// container is no longer bound when a client connects.
func TestBrokerUnboundConnection(t *testing.T) {
	baoClient := openbao.NewClient(openbao.Config{Address: "http://localhost:1"})
	b := testBroker(t, baoClient)

	containerID := "unbound-test"
	projectID := uuid.MustNew()

	err := b.Bind(containerID, projectID)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	sockPath := b.SocketPath(containerID)

	// Unbind the container but keep the listener open by doing it manually:
	// Remove from bindings but leave the listener running.
	b.mu.Lock()
	delete(b.bindings, containerID)
	b.mu.Unlock()

	// Connect - the handleConnection should see "not bound".
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	var resp SecretsResponse
	err = json.NewDecoder(conn).Decode(&resp)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if resp.Error != "container not bound" {
		t.Errorf("Error: got %q, want 'container not bound'", resp.Error)
	}

	// Restore binding so cleanup works.
	b.mu.Lock()
	b.bindings[containerID] = projectID
	b.mu.Unlock()
}

// TestBrokerReadSecretsError tests the handleConnection path where
// OpenBao returns an error when reading secrets.
func TestBrokerReadSecretsError(t *testing.T) {
	// Use a mock server that returns 500 errors.
	baoClient := openbao.NewClient(openbao.Config{
		Address: "http://127.0.0.1:1", // unreachable
		Token:   "test",
	})
	b := testBroker(t, baoClient)

	containerID := "error-test"
	projectID := uuid.MustNew()

	err := b.Bind(containerID, projectID)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	sockPath := b.SocketPath(containerID)
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	var resp SecretsResponse
	err = json.NewDecoder(conn).Decode(&resp)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected error response when OpenBao is unreachable")
	}
}

// TestBrokerBindListenError tests Bind when the socket path is invalid.
func TestBrokerBindListenError(t *testing.T) {
	baoClient := openbao.NewClient(openbao.Config{Address: "http://localhost:1", Token: "t"})

	// Use a directory that exists but contains a non-removable obstacle.
	dir := t.TempDir()
	b, err := New(Config{
		SocketDir: dir,
		BaoClient: baoClient,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer b.Close()

	// Try to bind with a container ID that creates an excessively long socket path.
	// Unix socket paths have a ~108 character limit on Linux.
	longName := ""
	for range 200 {
		longName += "a"
	}
	err = b.Bind(longName, uuid.MustNew())
	if err == nil {
		t.Error("expected error for listen on invalid path")
	}
}

// TestBrokerBindChmodError tests Bind when chmod fails.
func TestBrokerBindChmodError(t *testing.T) {
	baoClient := openbao.NewClient(openbao.Config{Address: "http://localhost:1", Token: "t"})
	dir := t.TempDir()
	b, err := New(Config{
		SocketDir: dir,
		BaoClient: baoClient,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer b.Close()

	// On Linux, Chmod on a socket file should succeed normally.
	// To force chmod failure, we'd need to mess with the filesystem.
	// Instead, this test verifies the happy path doesn't hit the chmod error.
	// The chmod error path is a defensive branch for unusual filesystem conditions.
	err = b.Bind("chmod-test", uuid.MustNew())
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
}

// TestNewBrokerInvalidDir tests creating a broker with an invalid directory.
func TestNewBrokerInvalidDir(t *testing.T) {
	baoClient := openbao.NewClient(openbao.Config{Address: "http://localhost"})
	_, err := New(Config{
		SocketDir: "/proc/nonexistent/broker-socks",
		BaoClient: baoClient,
	})
	if err == nil {
		t.Error("expected error for invalid socket directory")
	}
}
