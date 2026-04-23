package sshutil

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"golang.org/x/crypto/ssh"
)

// AuthorizedKeys holds a set of ssh public keys and reports whether a given
// key is a member. The set is backed by a file in standard authorized_keys
// format that can be rewritten at runtime (init-agent appends new keys);
// Reload() re-parses the file.
type AuthorizedKeys struct {
	path string

	mu   sync.RWMutex
	keys map[string]ssh.PublicKey // keyed by Marshal() bytes as hex string
}

// NewAuthorizedKeys returns an AuthorizedKeys backed by path. The file is
// loaded eagerly; a missing file is not an error (empty allowlist means no
// client can authenticate, which is the safe default).
func NewAuthorizedKeys(path string) (*AuthorizedKeys, error) {
	a := &AuthorizedKeys{path: path}
	if err := a.Reload(); err != nil {
		return nil, err
	}
	return a, nil
}

// Reload re-parses the underlying file. Safe to call concurrently with
// IsAuthorized; readers see the old set until the swap completes.
func (a *AuthorizedKeys) Reload() error {
	f, err := os.Open(a.path)
	if err != nil {
		if os.IsNotExist(err) {
			a.swap(map[string]ssh.PublicKey{})
			return nil
		}
		return fmt.Errorf("open %s: %w", a.path, err)
	}
	defer f.Close()
	keys, err := parseAuthorized(f)
	if err != nil {
		return err
	}
	a.swap(keys)
	return nil
}

func (a *AuthorizedKeys) swap(keys map[string]ssh.PublicKey) {
	a.mu.Lock()
	a.keys = keys
	a.mu.Unlock()
}

// IsAuthorized reports whether key is in the current allowlist.
func (a *AuthorizedKeys) IsAuthorized(key ssh.PublicKey) bool {
	fp := KeyFingerprint(key)
	a.mu.RLock()
	_, ok := a.keys[fp]
	a.mu.RUnlock()
	return ok
}

// Len returns the current allowlist size — handy for logging at startup.
func (a *AuthorizedKeys) Len() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.keys)
}

func KeyFingerprint(key ssh.PublicKey) string {
	return ssh.FingerprintSHA256(key)
}

func parseAuthorized(r io.Reader) (map[string]ssh.PublicKey, error) {
	keys := map[string]ssh.PublicKey{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<16), 1<<20)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		key, _, _, _, err := ssh.ParseAuthorizedKey(line)
		if err != nil {
			// Skip malformed lines rather than refusing the whole file — one
			// bad entry shouldn't lock out every legitimate shell.
			continue
		}
		keys[KeyFingerprint(key)] = key
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan authorized keys: %w", err)
	}
	return keys, nil
}
