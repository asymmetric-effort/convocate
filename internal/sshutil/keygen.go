package sshutil

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// Keypair bundles the artifacts you need after generating a fresh ed25519
// SSH key: the OpenSSH private-key PEM, the single-line OpenSSH public key
// (authorized_keys format, newline-terminated), and the parsed public key.
type Keypair struct {
	PrivatePEM       []byte        // OpenSSH PEM encoded private key
	AuthorizedKey    []byte        // "ssh-ed25519 AAAA... comment\n"
	PublicKey        ssh.PublicKey // parsed public key
	PublicKeyComment string        // comment baked into AuthorizedKey
}

// GenerateKeypair creates a fresh ed25519 SSH keypair with the given comment
// appended to the public-key line (useful for tagging keys by agent id).
// Nothing is written to disk; the caller decides where the material lives.
func GenerateKeypair(comment string) (*Keypair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519: %w", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	privPEM := pem.EncodeToMemory(block)

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("derive public key: %w", err)
	}
	authLine := ssh.MarshalAuthorizedKey(sshPub)
	// MarshalAuthorizedKey emits "algo base64\n" with no comment. Fold the
	// comment in before the trailing newline so authorized_keys readers can
	// identify the key later.
	if comment != "" && len(authLine) > 0 && authLine[len(authLine)-1] == '\n' {
		authLine = append(authLine[:len(authLine)-1], ' ')
		authLine = append(authLine, []byte(comment)...)
		authLine = append(authLine, '\n')
	}
	return &Keypair{
		PrivatePEM:       privPEM,
		AuthorizedKey:    authLine,
		PublicKey:        sshPub,
		PublicKeyComment: comment,
	}, nil
}
