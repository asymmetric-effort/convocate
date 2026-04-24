package hostinstall

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/asymmetric-effort/claude-shell/internal/tlsutil"
)

// RsyslogCADir is the on-host directory where init-shell stores the CA +
// server material. init-agent reads the CA cert/key back out of here when
// issuing client certs for a new agent.
const RsyslogCADir = "/etc/claude-shell/rsyslog-ca"

// File names inside RsyslogCADir. Kept separate from the path so tests can
// exercise them without dragging in the full dir prefix.
const (
	rsyslogCACertName     = "ca.crt"
	rsyslogCAKeyName      = "ca.key"
	rsyslogServerCertName = "server.crt"
	rsyslogServerKeyName  = "server.key"
)

// rsyslogServerConfig is the /etc/rsyslog.d drop-in that turns on a TLS
// imtcp listener on 514 and routes incoming messages to per-agent log
// files under /var/log/claude-agent/. The template keys off the authenticated
// hostname so an agent's forward-config ($LocalHostName <agent-id>) lands
// its messages under <agent-id>.log.
const rsyslogServerConfig = `# Managed by claude-host init-shell. Do not edit by hand.
module(load="imtcp"
       StreamDriver.Name="gtls"
       StreamDriver.Mode="1"
       StreamDriver.Authmode="x509/certvalid")

global(
    DefaultNetstreamDriver="gtls"
    DefaultNetstreamDriverCAFile="/etc/claude-shell/rsyslog-ca/ca.crt"
    DefaultNetstreamDriverCertFile="/etc/claude-shell/rsyslog-ca/server.crt"
    DefaultNetstreamDriverKeyFile="/etc/claude-shell/rsyslog-ca/server.key"
)

input(type="imtcp" port="514")

template(name="claudeAgentPerHost" type="list") {
    constant(value="/var/log/claude-agent/")
    property(name="hostname")
    constant(value=".log")
}

# Route any message arriving on the TLS listener (RemoteAddress non-empty)
# into the per-agent file and stop further processing so it doesn't also
# hit /var/log/syslog.
if ($inputname == "imtcp") then {
    action(type="omfile" dynaFile="claudeAgentPerHost"
           dirCreateMode="0755" fileCreateMode="0640")
    stop
}
`

// rsyslogLogrotateConfig keeps the per-agent logs bounded — daily rotation,
// 14-day retention, gzip after one cycle. copytruncate avoids needing
// to HUP rsyslog on every rotation.
const rsyslogLogrotateConfig = `# Managed by claude-host init-shell. Do not edit by hand.
/var/log/claude-agent/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
    dateext
}
`

// stepInstallRsyslogServer provisions the TLS CA + server cert on the
// target and drops the rsyslog + logrotate config. Idempotent: if a CA
// already exists on the target it's reused, so previously-issued client
// certs stay valid across init-shell reruns.
func stepInstallRsyslogServer(ctx context.Context, r Runner, log io.Writer) error {
	hostname, err := remoteHostname(ctx, r, log)
	if err != nil {
		return fmt.Errorf("determine shell hostname: %w", err)
	}
	fmt.Fprintf(log, "  shell hostname for SAN: %s\n", hostname)

	ca, err := loadOrGenerateRsyslogCA(ctx, r, log)
	if err != nil {
		return err
	}

	// Server cert is cheap to regenerate and picks up hostname changes.
	server, err := tlsutil.SignCert(ca, tlsutil.SignOptions{
		CommonName: hostname,
		DNSNames:   []string{hostname, "localhost"},
		IsServer:   true,
	})
	if err != nil {
		return fmt.Errorf("sign server cert: %w", err)
	}
	if err := r.Run(ctx, "mkdir -p "+RsyslogCADir, RunOptions{Sudo: true, Stdout: log, Stderr: log}); err != nil {
		return fmt.Errorf("mkdir %s: %w", RsyslogCADir, err)
	}
	writes := []struct {
		path    string
		content []byte
		mode    os.FileMode
	}{
		{RsyslogCADir + "/" + rsyslogServerCertName, server.CertPEM, 0644},
		{RsyslogCADir + "/" + rsyslogServerKeyName, server.KeyPEM, 0600},
	}
	for _, w := range writes {
		if err := writeRemoteContent(ctx, r, log, w.content, w.path, w.mode, "root:root"); err != nil {
			return err
		}
	}

	// Install rsyslog-gnutls so GnuTLS driver is available; on Ubuntu
	// 22.04 the default rsyslog package uses it but the TLS helpers
	// live in a separate package.
	if err := r.Run(ctx, "DEBIAN_FRONTEND=noninteractive apt-get install -y rsyslog-gnutls",
		RunOptions{Sudo: true, Stdout: log, Stderr: log}); err != nil {
		return fmt.Errorf("install rsyslog-gnutls: %w", err)
	}

	// Config + logrotate + log dir.
	if err := writeRemoteContent(ctx, r, log,
		[]byte(rsyslogServerConfig),
		"/etc/rsyslog.d/10-claude-shell-server.conf", 0644, "root:root"); err != nil {
		return err
	}
	if err := writeRemoteContent(ctx, r, log,
		[]byte(rsyslogLogrotateConfig),
		"/etc/logrotate.d/claude-shell-agent-logs", 0644, "root:root"); err != nil {
		return err
	}
	if err := r.Run(ctx, "mkdir -p /var/log/claude-agent && chmod 0755 /var/log/claude-agent",
		RunOptions{Sudo: true, Stdout: log, Stderr: log}); err != nil {
		return err
	}

	// Firewall + restart rsyslog so the new listener comes up.
	if err := r.Run(ctx, `command -v ufw >/dev/null 2>&1 && ufw allow 514/tcp || true`,
		RunOptions{Sudo: true, Stdout: log, Stderr: log}); err != nil {
		return err
	}
	if err := r.Run(ctx, "systemctl restart rsyslog",
		RunOptions{Sudo: true, Stdout: log, Stderr: log}); err != nil {
		return err
	}
	return nil
}

// loadOrGenerateRsyslogCA probes the target for an existing CA; reuses it
// when present so issued agent certs remain valid across reruns. Otherwise
// mints a fresh 10-year CA and uploads the material.
func loadOrGenerateRsyslogCA(ctx context.Context, r Runner, log io.Writer) (*tlsutil.KeyMaterial, error) {
	caCertPath := RsyslogCADir + "/" + rsyslogCACertName
	caKeyPath := RsyslogCADir + "/" + rsyslogCAKeyName

	exists, err := remoteFileExists(ctx, r, caCertPath, log)
	if err != nil {
		return nil, err
	}
	if exists {
		fmt.Fprintln(log, "  CA already present — reusing")
		certPEM, err := readRemoteFile(ctx, r, caCertPath, log)
		if err != nil {
			return nil, fmt.Errorf("read existing CA cert: %w", err)
		}
		keyPEM, err := readRemoteFile(ctx, r, caKeyPath, log)
		if err != nil {
			return nil, fmt.Errorf("read existing CA key: %w", err)
		}
		return tlsutil.ParseKeyMaterial(certPEM, keyPEM)
	}

	fmt.Fprintln(log, "  minting new rsyslog CA")
	ca, err := tlsutil.GenerateCA("claude-shell rsyslog CA", 10)
	if err != nil {
		return nil, err
	}
	if err := r.Run(ctx, "mkdir -p "+RsyslogCADir,
		RunOptions{Sudo: true, Stdout: log, Stderr: log}); err != nil {
		return nil, err
	}
	if err := writeRemoteContent(ctx, r, log, ca.CertPEM, caCertPath, 0644, "root:root"); err != nil {
		return nil, err
	}
	if err := writeRemoteContent(ctx, r, log, ca.KeyPEM, caKeyPath, 0600, "root:root"); err != nil {
		return nil, err
	}
	return ca, nil
}

// readRemoteFile pulls content via `sudo cat <path>`. Intended for small
// files (cert/key material) — don't use on a multi-megabyte log.
func readRemoteFile(ctx context.Context, r Runner, path string, log io.Writer) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.Run(ctx, "cat "+shellQuoteArg(path), RunOptions{
		Sudo:   true,
		Stdout: &buf,
		Stderr: log,
	}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// remoteHostname asks the target for its fully-qualified hostname; falls
// back to the short form if FQDN resolution fails.
func remoteHostname(ctx context.Context, r Runner, log io.Writer) (string, error) {
	var buf bytes.Buffer
	if err := r.Run(ctx, "hostname -f 2>/dev/null || hostname",
		RunOptions{Stdout: &buf, Stderr: log}); err != nil {
		return "", err
	}
	name := strings.TrimSpace(buf.String())
	if name == "" {
		return "", fmt.Errorf("empty hostname")
	}
	return name, nil
}

