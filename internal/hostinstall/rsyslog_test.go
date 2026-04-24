package hostinstall

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/asymmetric-effort/claude-shell/internal/tlsutil"
)

func TestRsyslogServerStep_GeneratesCA_WhenMissing(t *testing.T) {
	m := &mockRunner{
		cmdStdout: map[string]string{
			"hostname -f":                                         "shell.example\n",
			"test -f '/etc/claude-shell/rsyslog-ca/ca.crt'":       "NO\n",
		},
	}
	var log bytes.Buffer
	if err := stepInstallRsyslogServer(context.Background(), m, &log); err != nil {
		t.Fatalf("step: %v\nlog:\n%s", err, log.String())
	}

	// CA material must have been uploaded — and must round-trip through
	// ParseKeyMaterial, proving we wrote real PEM.
	caCert := findCopy(m.copies, "/etc/claude-shell/rsyslog-ca/ca.crt")
	caKey := findCopy(m.copies, "/etc/claude-shell/rsyslog-ca/ca.key")
	if caCert == nil || caKey == nil {
		t.Fatalf("CA material not uploaded: cert=%v key=%v", caCert != nil, caKey != nil)
	}
	if _, err := tlsutil.ParseKeyMaterial(caCert.Content, caKey.Content); err != nil {
		t.Errorf("uploaded CA doesn't parse: %v", err)
	}

	// Server cert present + signed under the fresh CA (so it chains).
	srvCert := findCopy(m.copies, "/etc/claude-shell/rsyslog-ca/server.crt")
	srvKey := findCopy(m.copies, "/etc/claude-shell/rsyslog-ca/server.key")
	if srvCert == nil || srvKey == nil {
		t.Fatalf("server material not uploaded")
	}
	if srvCert.Mode != 0644 {
		t.Errorf("server.crt mode = %o, want 0644", srvCert.Mode)
	}
	if srvKey.Mode != 0600 {
		t.Errorf("server.key mode = %o, want 0600", srvKey.Mode)
	}

	// Config + logrotate written.
	if cfg := findCopy(m.copies, "/etc/rsyslog.d/10-claude-shell-server.conf"); cfg == nil {
		t.Error("rsyslog server config not uploaded")
	} else if !bytes.Contains(cfg.Content, []byte("imtcp")) {
		t.Errorf("rsyslog config missing imtcp module: %q", cfg.Content)
	}
	if lr := findCopy(m.copies, "/etc/logrotate.d/claude-shell-agent-logs"); lr == nil {
		t.Error("logrotate config not uploaded")
	}

	// Expected command sequence highlights.
	joined := allCmds(m.cmds)
	for _, want := range []string{
		"hostname -f",
		"test -f '/etc/claude-shell/rsyslog-ca/ca.crt'",
		"mkdir -p /etc/claude-shell/rsyslog-ca",
		"rsyslog-gnutls",
		"mkdir -p /var/log/claude-agent",
		"ufw allow 514/tcp",
		"systemctl restart rsyslog",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing cmd substring %q", want)
		}
	}
}

func TestRsyslogServerStep_ReusesExistingCA(t *testing.T) {
	// Pre-mint a CA and hand its PEM back via the mock runner's `cat`
	// stdout so the step's "already present" branch takes over.
	existing, err := tlsutil.GenerateCA("existing", 5)
	if err != nil {
		t.Fatal(err)
	}
	m := &mockRunner{
		cmdStdout: map[string]string{
			"hostname -f":                                   "shell.example\n",
			"test -f '/etc/claude-shell/rsyslog-ca/ca.crt'": "YES\n",
			"cat '/etc/claude-shell/rsyslog-ca/ca.crt'":     string(existing.CertPEM),
			"cat '/etc/claude-shell/rsyslog-ca/ca.key'":     string(existing.KeyPEM),
		},
	}
	if err := stepInstallRsyslogServer(context.Background(), m, &bytes.Buffer{}); err != nil {
		t.Fatalf("step: %v", err)
	}
	// CA material must NOT have been re-uploaded.
	if findCopy(m.copies, "/etc/claude-shell/rsyslog-ca/ca.crt") != nil {
		t.Error("existing CA was overwritten")
	}
	if findCopy(m.copies, "/etc/claude-shell/rsyslog-ca/ca.key") != nil {
		t.Error("existing CA key was overwritten")
	}
	// But a new server cert WAS issued.
	if findCopy(m.copies, "/etc/claude-shell/rsyslog-ca/server.crt") == nil {
		t.Error("server cert should still be regenerated when reusing CA")
	}
}

func TestReadRemoteFile(t *testing.T) {
	m := &mockRunner{cmdStdout: map[string]string{
		"cat '/x/y'": "hello\n",
	}}
	out, err := readRemoteFile(context.Background(), m, "/x/y", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello\n" {
		t.Errorf("got %q", out)
	}
}

func TestRemoteHostname_FallbackOnEmpty(t *testing.T) {
	m := &mockRunner{} // no stdout stubbed
	_, err := remoteHostname(context.Background(), m, &bytes.Buffer{})
	if err == nil {
		t.Error("expected error for empty hostname")
	}
}
