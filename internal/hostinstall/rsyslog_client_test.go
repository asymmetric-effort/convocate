package hostinstall

import (
	"bytes"
	"context"
	"crypto/x509"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asymmetric-effort/claude-shell/internal/tlsutil"
)

func TestConfigureAgentRsyslogClient_MissingCA(t *testing.T) {
	dir := t.TempDir()
	m := &mockRunner{}
	err := configureAgentRsyslogClient(context.Background(), m, dir, "agent-x", "shell.example", "", "", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "run 'claude-host init-shell' first") {
		t.Errorf("expected CA-missing guidance, got %v", err)
	}
	if len(m.copies) != 0 || len(m.cmds) != 0 {
		t.Error("no remote work should happen when CA is missing")
	}
}

func TestConfigureAgentRsyslogClient_Happy(t *testing.T) {
	dir := t.TempDir()
	// Seed a CA on the local filesystem.
	ca, err := tlsutil.GenerateCA("ca", 2)
	if err != nil {
		t.Fatal(err)
	}
	caDir := filepath.Join(dir, "rsyslog-ca")
	if err := os.MkdirAll(caDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caDir, "ca.crt"), ca.CertPEM, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caDir, "ca.key"), ca.KeyPEM, 0600); err != nil {
		t.Fatal(err)
	}

	m := &mockRunner{}
	if err := configureAgentRsyslogClient(context.Background(), m, dir, "agent-yyy", "shell.example.com", "", "", &bytes.Buffer{}); err != nil {
		t.Fatalf("step: %v", err)
	}

	// Client cert must validate under the CA with ClientAuth usage.
	clientCopy := findCopy(m.copies, "/etc/claude-agent/rsyslog-tls/client.crt")
	keyCopy := findCopy(m.copies, "/etc/claude-agent/rsyslog-tls/client.key")
	if clientCopy == nil || keyCopy == nil {
		t.Fatal("client cert/key not uploaded")
	}
	leaf, err := tlsutil.ParseKeyMaterial(clientCopy.Content, keyCopy.Content)
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	if _, err := leaf.Cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Errorf("client cert doesn't validate under CA: %v", err)
	}
	if leaf.Cert.Subject.CommonName != "agent-yyy" {
		t.Errorf("client CN = %q, want agent-yyy", leaf.Cert.Subject.CommonName)
	}

	// rsyslog config embeds the agent-id and target host.
	cfg := findCopy(m.copies, "/etc/rsyslog.d/10-claude-shell-client.conf")
	if cfg == nil {
		t.Fatal("client config not uploaded")
	}
	body := string(cfg.Content)
	if !strings.Contains(body, "$LocalHostName agent-yyy") {
		t.Errorf("LocalHostName stamp missing or wrong in config:\n%s", body)
	}
	if !strings.Contains(body, `target="shell.example.com"`) {
		t.Errorf("target host missing in config")
	}
	if !strings.Contains(body, `port="514"`) {
		t.Errorf("port missing in config")
	}
	if !strings.Contains(body, `StreamDriver="gtls"`) {
		t.Errorf("gtls driver missing in config")
	}

	// CA cert must also have been uploaded to the agent so the daemon
	// can validate the server end of the TLS handshake.
	caCopy := findCopy(m.copies, "/etc/claude-agent/rsyslog-tls/ca.crt")
	if caCopy == nil {
		t.Fatal("CA cert not uploaded to agent")
	}
	if !bytes.Equal(caCopy.Content, ca.CertPEM) {
		t.Error("uploaded CA doesn't match source")
	}
}
