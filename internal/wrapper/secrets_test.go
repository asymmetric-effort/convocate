package wrapper

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestFetchSecretsFrom(t *testing.T) {
	t.Run("valid secrets", func(t *testing.T) {
		socketPath := filepath.Join(t.TempDir(), "test.sock")
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer listener.Close()

		// Serve one connection.
		go func() {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			defer conn.Close()
			resp := SecretsResponse{
				SSHPrivateKey: "ssh-key-data",
				GitHubPAT:     "ghp_test123",
				CustomSecrets: map[string]string{"FOO": "BAR"},
			}
			json.NewEncoder(conn).Encode(resp)
		}()

		w, _ := testWrapperForSecrets(t)
		result, err := w.FetchSecretsFrom(socketPath)
		if err != nil {
			t.Fatalf("FetchSecretsFrom: %v", err)
		}
		if result.SSHPrivateKey != "ssh-key-data" {
			t.Errorf("SSHPrivateKey: got %q", result.SSHPrivateKey)
		}
		if result.GitHubPAT != "ghp_test123" {
			t.Errorf("GitHubPAT: got %q", result.GitHubPAT)
		}
		if result.CustomSecrets["FOO"] != "BAR" {
			t.Errorf("CustomSecrets: got %v", result.CustomSecrets)
		}
	})

	t.Run("broker returns error", func(t *testing.T) {
		socketPath := filepath.Join(t.TempDir(), "err.sock")
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer listener.Close()

		go func() {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			defer conn.Close()
			resp := SecretsResponse{Error: "project not bound"}
			json.NewEncoder(conn).Encode(resp)
		}()

		w, _ := testWrapperForSecrets(t)
		_, err = w.FetchSecretsFrom(socketPath)
		if err == nil {
			t.Error("expected error when broker returns error")
		}
	})

	t.Run("socket does not exist", func(t *testing.T) {
		w, _ := testWrapperForSecrets(t)
		_, err := w.FetchSecretsFrom("/tmp/nonexistent-sock-" + t.Name() + ".sock")
		if err == nil {
			t.Error("expected error for nonexistent socket")
		}
	})

	t.Run("FetchSecrets uses wrapper socket path", func(t *testing.T) {
		socketPath := filepath.Join(t.TempDir(), "default.sock")
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer listener.Close()

		go func() {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			defer conn.Close()
			resp := SecretsResponse{GitHubPAT: "pat_default"}
			json.NewEncoder(conn).Encode(resp)
		}()

		dir := t.TempDir()
		w, err := New(&Config{
			WorkspaceDir:  dir,
			SecretsSocket: socketPath,
			Logger:        log.New(io.Discard, "", 0),
			CmdRunner:     newMockCommandRunner(),
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		result, err := w.FetchSecrets()
		if err != nil {
			t.Fatalf("FetchSecrets: %v", err)
		}
		if result.GitHubPAT != "pat_default" {
			t.Errorf("GitHubPAT: got %q", result.GitHubPAT)
		}
	})
}

func testWrapperForSecrets(t *testing.T) (*Wrapper, *mockCommandRunner) {
	t.Helper()
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "placeholder.sock")
	// Create a placeholder file so the path exists for Config validation.
	os.WriteFile(socketPath, nil, 0o600)
	runner := newMockCommandRunner()
	w, err := New(&Config{
		WorkspaceDir:  dir,
		SecretsSocket: socketPath,
		Logger:        log.New(io.Discard, "", 0),
		CmdRunner:     runner,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return w, runner
}
