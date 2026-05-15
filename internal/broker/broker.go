package broker

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/asymmetric-effort/convocate/internal/openbao"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// Broker is the per-container Unix socket multiplexer. It creates one
// Unix socket per Agent Container, maps each socket to its bound project,
// and serves secrets from the local OpenBao Agent on read.
type Broker struct {
	mu           sync.RWMutex
	socketDir    string
	baoClient    *openbao.Client
	bindings     map[string]uuid.UUID // containerID -> projectID
	listeners    map[string]net.Listener // containerID -> listener
	stopChannels map[string]chan struct{} // containerID -> stop signal
}

// Config holds the Broker's configuration.
type Config struct {
	SocketDir string
	BaoClient *openbao.Client
}

// New creates a new Broker.
func New(config Config) (*Broker, error) {
	if config.SocketDir == "" {
		return nil, fmt.Errorf("broker: socket directory is required")
	}
	if config.BaoClient == nil {
		return nil, fmt.Errorf("broker: OpenBao client is required")
	}
	err := os.MkdirAll(config.SocketDir, 0o750)
	if err != nil {
		return nil, fmt.Errorf("broker: create socket dir: %w", err)
	}
	return &Broker{
		socketDir:    config.SocketDir,
		baoClient:    config.BaoClient,
		bindings:     make(map[string]uuid.UUID),
		listeners:    make(map[string]net.Listener),
		stopChannels: make(map[string]chan struct{}),
	}, nil
}

// SocketPath returns the path to a container's secrets socket.
func (b *Broker) SocketPath(containerID string) string {
	return filepath.Join(b.socketDir, containerID+".sock")
}

// Bind creates a Unix socket for the given container and maps it to the
// given project. The socket begins accepting connections immediately.
func (b *Broker) Bind(containerID string, projectID uuid.UUID) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.bindings[containerID]; exists {
		return fmt.Errorf("broker: container %q already bound", containerID)
	}

	sockPath := b.SocketPath(containerID)
	// Remove stale socket file if it exists.
	os.Remove(sockPath)

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("broker: listen on %s: %w", sockPath, err)
	}

	// Ensure the socket is accessible by the container.
	err = os.Chmod(sockPath, 0o660)
	if err != nil {
		listener.Close()
		return fmt.Errorf("broker: chmod socket: %w", err)
	}

	stopCh := make(chan struct{})
	b.bindings[containerID] = projectID
	b.listeners[containerID] = listener
	b.stopChannels[containerID] = stopCh

	go b.serve(containerID, listener, stopCh)

	return nil
}

// Unbind stops the socket for the given container, closes the listener,
// removes the socket file, and clears the binding.
func (b *Broker) Unbind(containerID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	stopCh, exists := b.stopChannels[containerID]
	if !exists {
		return fmt.Errorf("broker: container %q not bound", containerID)
	}

	close(stopCh)

	listener := b.listeners[containerID]
	if listener != nil {
		listener.Close()
	}

	sockPath := b.SocketPath(containerID)
	os.Remove(sockPath)

	delete(b.bindings, containerID)
	delete(b.listeners, containerID)
	delete(b.stopChannels, containerID)

	return nil
}

// IsBound checks whether a container has a binding.
func (b *Broker) IsBound(containerID string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, exists := b.bindings[containerID]
	return exists
}

// BoundProject returns the project ID bound to a container, or zero UUID
// if not bound.
func (b *Broker) BoundProject(containerID string) uuid.UUID {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.bindings[containerID]
}

// BoundContainers returns a list of all bound container IDs.
func (b *Broker) BoundContainers() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]string, 0, len(b.bindings))
	for containerID := range b.bindings {
		result = append(result, containerID)
	}
	return result
}

// Close shuts down the broker, unbinding all containers.
func (b *Broker) Close() error {
	b.mu.Lock()
	containerIDs := make([]string, 0, len(b.bindings))
	for containerID := range b.bindings {
		containerIDs = append(containerIDs, containerID)
	}
	b.mu.Unlock()

	for _, containerID := range containerIDs {
		b.Unbind(containerID)
	}
	return nil
}

// serve handles connections on a container's Unix socket.
func (b *Broker) serve(containerID string, listener net.Listener, stopCh chan struct{}) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-stopCh:
				return
			default:
				continue
			}
		}
		go b.handleConnection(containerID, conn)
	}
}

// SecretsResponse is the JSON response sent to an Agent Container over
// its secrets socket.
type SecretsResponse struct {
	SSHPrivateKey string            `json:"ssh_private_key,omitempty"`
	GitHubPAT     string            `json:"github_pat,omitempty"`
	CustomSecrets map[string]string `json:"custom_secrets,omitempty"`
	Error         string            `json:"error,omitempty"`
}

func (b *Broker) handleConnection(containerID string, conn net.Conn) {
	defer conn.Close()

	b.mu.RLock()
	projectID, exists := b.bindings[containerID]
	b.mu.RUnlock()

	var resp SecretsResponse
	if !exists {
		resp.Error = "container not bound"
		json.NewEncoder(conn).Encode(resp)
		return
	}

	secrets, err := b.baoClient.ReadProjectSecrets(projectID)
	if err != nil {
		resp.Error = fmt.Sprintf("read secrets: %v", err)
		json.NewEncoder(conn).Encode(resp)
		return
	}
	if secrets == nil {
		resp.Error = "no secrets found for project"
		json.NewEncoder(conn).Encode(resp)
		return
	}

	resp.SSHPrivateKey = secrets.SSHPrivateKey
	resp.GitHubPAT = secrets.GitHubPAT
	resp.CustomSecrets = secrets.CustomSecrets
	json.NewEncoder(conn).Encode(resp)
}
