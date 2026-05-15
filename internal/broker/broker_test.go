package broker

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/openbao"
	"github.com/asymmetric-effort/convocate/internal/uuid"
	"net/http"
	"net/http/httptest"
)

// mockBaoServer sets up a mock OpenBao that serves project secrets.
func mockBaoServer(t *testing.T, projectSecrets map[string]map[string]interface{}) (*openbao.Client, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// KV v2 read.
		if r.Method == "GET" {
			for path, data := range projectSecrets {
				if r.URL.Path == "/v1/secret/data/"+path {
					json.NewEncoder(w).Encode(map[string]interface{}{
						"data": map[string]interface{}{
							"data": data,
						},
					})
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	client := openbao.NewClient(openbao.Config{
		Address: server.URL,
		Token:   "test-token",
	})
	return client, server.Close
}

func testBroker(t *testing.T, baoClient *openbao.Client) *Broker {
	t.Helper()
	dir := t.TempDir()
	b, err := New(Config{
		SocketDir: dir,
		BaoClient: baoClient,
	})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func TestNewBrokerRequiresSocketDir(t *testing.T) {
	_, err := New(Config{BaoClient: openbao.NewClient(openbao.Config{Address: "http://localhost"})})
	if err == nil {
		t.Error("expected error for missing socket dir")
	}
}

func TestNewBrokerRequiresBaoClient(t *testing.T) {
	_, err := New(Config{SocketDir: t.TempDir()})
	if err == nil {
		t.Error("expected error for missing OpenBao client")
	}
}

func TestBindUnbind(t *testing.T) {
	baoClient := openbao.NewClient(openbao.Config{Address: "http://localhost"})
	b := testBroker(t, baoClient)
	containerID := "container-abc"
	projectID := uuid.MustNew()

	t.Run("bind", func(t *testing.T) {
		err := b.Bind(containerID, projectID)
		if err != nil {
			t.Fatalf("Bind error: %v", err)
		}
	})

	t.Run("is bound", func(t *testing.T) {
		if !b.IsBound(containerID) {
			t.Error("expected container to be bound")
		}
	})

	t.Run("bound project", func(t *testing.T) {
		got := b.BoundProject(containerID)
		if got != projectID {
			t.Errorf("BoundProject: got %s, want %s", got, projectID)
		}
	})

	t.Run("socket file exists", func(t *testing.T) {
		sockPath := b.SocketPath(containerID)
		_, err := os.Stat(sockPath)
		if err != nil {
			t.Errorf("socket file missing: %v", err)
		}
	})

	t.Run("bound containers", func(t *testing.T) {
		containers := b.BoundContainers()
		if len(containers) != 1 {
			t.Fatalf("BoundContainers: got %d, want 1", len(containers))
		}
		if containers[0] != containerID {
			t.Errorf("BoundContainers[0]: got %q, want %q", containers[0], containerID)
		}
	})

	t.Run("double bind fails", func(t *testing.T) {
		err := b.Bind(containerID, projectID)
		if err == nil {
			t.Error("expected error for double bind")
		}
	})

	t.Run("unbind", func(t *testing.T) {
		err := b.Unbind(containerID)
		if err != nil {
			t.Fatalf("Unbind error: %v", err)
		}
	})

	t.Run("not bound after unbind", func(t *testing.T) {
		if b.IsBound(containerID) {
			t.Error("expected container to not be bound after unbind")
		}
	})

	t.Run("socket file removed", func(t *testing.T) {
		sockPath := b.SocketPath(containerID)
		_, err := os.Stat(sockPath)
		if !os.IsNotExist(err) {
			t.Error("socket file should be removed after unbind")
		}
	})

	t.Run("unbind nonexistent fails", func(t *testing.T) {
		err := b.Unbind("nonexistent")
		if err == nil {
			t.Error("expected error for unbinding nonexistent container")
		}
	})
}

func TestSocketPath(t *testing.T) {
	baoClient := openbao.NewClient(openbao.Config{Address: "http://localhost"})
	b := testBroker(t, baoClient)
	path := b.SocketPath("container-xyz")
	if filepath.Base(path) != "container-xyz.sock" {
		t.Errorf("SocketPath: got %q, want *container-xyz.sock", path)
	}
}

func TestBrokerServesSecrets(t *testing.T) {
	projectID := uuid.MustNew()
	projectPath := "projects/" + projectID.String()

	baoClient, cleanup := mockBaoServer(t, map[string]map[string]interface{}{
		projectPath: {
			"ssh_private_key": "the-ssh-key",
			"github_pat":      "ghp_test123",
			"custom_NPM_TOKEN": "tok_npm",
		},
	})
	defer cleanup()

	b := testBroker(t, baoClient)

	err := b.Bind("container-abc", projectID)
	if err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	// Give the listener a moment to start accepting.
	time.Sleep(10 * time.Millisecond)

	// Connect to the socket and read secrets.
	sockPath := b.SocketPath("container-abc")
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial error: %v", err)
	}
	defer conn.Close()

	var resp SecretsResponse
	err = json.NewDecoder(conn).Decode(&resp)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if resp.SSHPrivateKey != "the-ssh-key" {
		t.Errorf("SSHPrivateKey: got %q, want %q", resp.SSHPrivateKey, "the-ssh-key")
	}
	if resp.GitHubPAT != "ghp_test123" {
		t.Errorf("GitHubPAT: got %q, want %q", resp.GitHubPAT, "ghp_test123")
	}
	if resp.CustomSecrets["NPM_TOKEN"] != "tok_npm" {
		t.Errorf("NPM_TOKEN: got %q, want %q", resp.CustomSecrets["NPM_TOKEN"], "tok_npm")
	}
}

func TestBrokerServesErrorForMissingSecrets(t *testing.T) {
	projectID := uuid.MustNew()

	// Mock server with no secrets for this project.
	baoClient, cleanup := mockBaoServer(t, map[string]map[string]interface{}{})
	defer cleanup()

	b := testBroker(t, baoClient)

	err := b.Bind("container-abc", projectID)
	if err != nil {
		t.Fatalf("Bind error: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	sockPath := b.SocketPath("container-abc")
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial error: %v", err)
	}
	defer conn.Close()

	var resp SecretsResponse
	err = json.NewDecoder(conn).Decode(&resp)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if resp.Error == "" {
		t.Error("expected error for missing secrets, got none")
	}
}

func TestBrokerMultipleContainers(t *testing.T) {
	baoClient := openbao.NewClient(openbao.Config{Address: "http://localhost"})
	b := testBroker(t, baoClient)

	ids := []string{"c1", "c2", "c3"}
	for _, containerID := range ids {
		err := b.Bind(containerID, uuid.MustNew())
		if err != nil {
			t.Fatalf("Bind %q error: %v", containerID, err)
		}
	}

	containers := b.BoundContainers()
	if len(containers) != 3 {
		t.Errorf("BoundContainers: got %d, want 3", len(containers))
	}

	// Unbind one.
	err := b.Unbind("c2")
	if err != nil {
		t.Fatalf("Unbind error: %v", err)
	}

	containers = b.BoundContainers()
	if len(containers) != 2 {
		t.Errorf("BoundContainers after unbind: got %d, want 2", len(containers))
	}
}

func TestBrokerCloseUnbindsAll(t *testing.T) {
	baoClient := openbao.NewClient(openbao.Config{Address: "http://localhost"})
	dir := t.TempDir()
	b, err := New(Config{
		SocketDir: dir,
		BaoClient: baoClient,
	})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	for _, containerID := range []string{"c1", "c2"} {
		err := b.Bind(containerID, uuid.MustNew())
		if err != nil {
			t.Fatalf("Bind error: %v", err)
		}
	}

	err = b.Close()
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}

	containers := b.BoundContainers()
	if len(containers) != 0 {
		t.Errorf("BoundContainers after Close: got %d, want 0", len(containers))
	}
}

func TestBoundProjectZeroForUnbound(t *testing.T) {
	baoClient := openbao.NewClient(openbao.Config{Address: "http://localhost"})
	b := testBroker(t, baoClient)
	got := b.BoundProject("nonexistent")
	if !got.IsZero() {
		t.Errorf("expected zero UUID, got %s", got)
	}
}
