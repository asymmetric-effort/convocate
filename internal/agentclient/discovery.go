package agentclient

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultAgentKeysDir is where convocate-host init-agent stows the shell->agent
// private key and agent-host metadata for each registered agent.
const DefaultAgentKeysDir = "/etc/convocate/agent-keys"

// AgentRecord describes one registered agent's connection parameters. The
// TUI iterates these to build the list of remote hosts to dial.
type AgentRecord struct {
	// ID is the 12-char alphanumeric agent identifier.
	ID string
	// Host is the agent's network address (hostname or IP, no port).
	Host string
	// PrivateKeyPath is the path to the shell->agent SSH private key.
	PrivateKeyPath string
}

// DiscoverAgents enumerates subdirectories of dir (default:
// DefaultAgentKeysDir when dir is empty) and returns one AgentRecord per
// directory whose contents look complete (agent-host + shell->agent key).
// Malformed or partial directories are skipped with no error — they
// typically correspond to an in-progress init-agent run.
func DiscoverAgents(dir string) ([]AgentRecord, error) {
	if dir == "" {
		dir = DefaultAgentKeysDir
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// No agents registered yet — return an empty slice, not an error.
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var out []AgentRecord
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		agentDir := filepath.Join(dir, id)
		keyPath := filepath.Join(agentDir, "shell_to_agent_ed25519_key")
		hostPath := filepath.Join(agentDir, "agent-host")

		if _, err := os.Stat(keyPath); err != nil {
			continue
		}
		hostBytes, err := os.ReadFile(hostPath)
		if err != nil {
			continue
		}
		host := strings.TrimSpace(string(hostBytes))
		if host == "" {
			continue
		}
		// agent-host was recorded as Runner.Target() which is "user@host"
		// for SSH and "local" for a local install. Strip the user prefix
		// so we can dial by hostname.
		if i := strings.Index(host, "@"); i >= 0 {
			host = host[i+1:]
		}
		out = append(out, AgentRecord{ID: id, Host: host, PrivateKeyPath: keyPath})
	}
	// Deterministic order for UI stability.
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
