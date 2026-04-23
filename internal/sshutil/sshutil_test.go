package sshutil

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

// --- hostkey ---------------------------------------------------------------

func TestLoadOrCreateHostKey_GeneratesAndReuses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host_key")

	first, err := LoadOrCreateHostKey(path)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if first == nil {
		t.Fatal("expected signer, got nil")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat host key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("host key mode = %v, want 0600", info.Mode().Perm())
	}

	second, err := LoadOrCreateHostKey(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !bytes.Equal(first.PublicKey().Marshal(), second.PublicKey().Marshal()) {
		t.Error("expected stable host key across reloads")
	}
}

func TestLoadOrCreateHostKey_MalformedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host_key")
	if err := os.WriteFile(path, []byte("not a key"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadOrCreateHostKey(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

// --- authorized keys ------------------------------------------------------

// generateAuthKey returns a fresh ed25519 keypair wrapped for SSH use, used
// throughout this package (and by dependent packages via
// sshutiltest.GenerateKey if we ever extract one).
func generateAuthKey(t *testing.T) (ssh.Signer, ssh.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("parse pub: %v", err)
	}
	return signer, sshPub
}

func writeAuthorizedKeys(t *testing.T, path string, keys ...ssh.PublicKey) {
	t.Helper()
	var buf bytes.Buffer
	for _, k := range keys {
		buf.Write(ssh.MarshalAuthorizedKey(k))
	}
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorizedKeys_MatchesPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth")
	_, pub := generateAuthKey(t)
	_, strangerPub := generateAuthKey(t)
	writeAuthorizedKeys(t, path, pub)

	a, err := NewAuthorizedKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	if !a.IsAuthorized(pub) {
		t.Error("expected pub to be authorized")
	}
	if a.IsAuthorized(strangerPub) {
		t.Error("expected unknown pub to be rejected")
	}
	if a.Len() != 1 {
		t.Errorf("Len = %d, want 1", a.Len())
	}
}

func TestAuthorizedKeys_MissingFileEmptyAllowlist(t *testing.T) {
	a, err := NewAuthorizedKeys("/definitely/not/here")
	if err != nil {
		t.Fatalf("missing file must not error: %v", err)
	}
	if a.Len() != 0 {
		t.Errorf("Len = %d, want 0", a.Len())
	}
	_, pub := generateAuthKey(t)
	if a.IsAuthorized(pub) {
		t.Error("empty allowlist must reject everything")
	}
}

func TestAuthorizedKeys_CommentsAndBlanksSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth")
	_, pub := generateAuthKey(t)
	content := append([]byte("# leading comment\n\n"), ssh.MarshalAuthorizedKey(pub)...)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	a, err := NewAuthorizedKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.Len() != 1 {
		t.Errorf("Len = %d, want 1", a.Len())
	}
}

func TestAuthorizedKeys_Reload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	a, err := NewAuthorizedKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	_, pub := generateAuthKey(t)
	writeAuthorizedKeys(t, path, pub)
	if err := a.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !a.IsAuthorized(pub) {
		t.Error("reload did not pick up new key")
	}
}
