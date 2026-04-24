package sshutil

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestGenerateKeypair_RoundTrip(t *testing.T) {
	kp, err := GenerateKeypair("agent=abc123")
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	// Private key must parse via ssh.ParsePrivateKey — that's what the
	// status emitter + shell CRUD client both use.
	signer, err := ssh.ParsePrivateKey(kp.PrivatePEM)
	if err != nil {
		t.Fatalf("parse private: %v", err)
	}
	// The signer's public key must match the authorized_keys line we emit —
	// agent/shell would otherwise fail auth on first connect.
	if !bytes.Equal(signer.PublicKey().Marshal(), kp.PublicKey.Marshal()) {
		t.Error("signer pub != generated pub")
	}
	// Comment must be baked into the authorized-key line.
	if !bytes.Contains(kp.AuthorizedKey, []byte("agent=abc123")) {
		t.Errorf("comment missing from authorized_keys line: %q", kp.AuthorizedKey)
	}
	// Must be parseable back by ssh.ParseAuthorizedKey.
	out, comment, _, _, err := ssh.ParseAuthorizedKey(kp.AuthorizedKey)
	if err != nil {
		t.Fatalf("parse authorized: %v", err)
	}
	if !bytes.Equal(out.Marshal(), kp.PublicKey.Marshal()) {
		t.Error("re-parsed authorized key doesn't match")
	}
	if comment != "agent=abc123" {
		t.Errorf("comment = %q, want agent=abc123", comment)
	}
}

func TestGenerateKeypair_EmptyComment(t *testing.T) {
	kp, err := GenerateKeypair("")
	if err != nil {
		t.Fatal(err)
	}
	// No trailing space when no comment.
	line := string(kp.AuthorizedKey)
	if strings.HasSuffix(strings.TrimRight(line, "\n"), " ") {
		t.Errorf("trailing space with empty comment: %q", line)
	}
	if _, _, _, _, err := ssh.ParseAuthorizedKey(kp.AuthorizedKey); err != nil {
		t.Errorf("parse: %v", err)
	}
}

func TestGenerateKeypair_ProducesDistinctKeys(t *testing.T) {
	a, _ := GenerateKeypair("a")
	b, _ := GenerateKeypair("b")
	if bytes.Equal(a.PrivatePEM, b.PrivatePEM) {
		t.Error("two calls produced identical keys")
	}
}
