// Package agentserver implements the claude-agent SSH server, subsystem
// dispatcher, and RPC handlers.
//
// Security posture: the server accepts **only** the "claude-agent-rpc" and
// "claude-agent-attach" subsystems (the latter lands in a later commit).
// Shell, exec, direct-tcpip, and any other SSH channel/request types are
// rejected. There is no arbitrary command execution path.
package agentserver

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

// LoadOrCreateHostKey reads an ed25519 private key from path. If the file is
// missing, a fresh key is generated and written with mode 0600. Returns the
// parsed ssh.Signer ready to plug into a server config.
func LoadOrCreateHostKey(path string) (ssh.Signer, error) {
	if data, err := os.ReadFile(path); err == nil {
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("parse host key %s: %w", path, err)
		}
		return signer, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read host key %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key: %w", err)
	}
	pemBlock, err := ssh.MarshalPrivateKey(priv, "claude-agent-host")
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	encoded := pem.EncodeToMemory(pemBlock)
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		return nil, fmt.Errorf("write host key %s: %w", path, err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, fmt.Errorf("new signer: %w", err)
	}
	return signer, nil
}
