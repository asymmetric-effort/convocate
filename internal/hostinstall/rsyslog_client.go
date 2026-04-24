package hostinstall

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/asymmetric-effort/claude-shell/internal/tlsutil"
)

// RsyslogAgentTLSDir is where init-agent stashes the CA cert + client
// cert/key on the agent host. The rsyslog config references these paths
// verbatim, so changing it means rewriting the config template.
const RsyslogAgentTLSDir = "/etc/claude-agent/rsyslog-tls"

// rsyslogClientConfigTpl is the rsyslog drop-in that forwards every
// message to the shell host's TLS listener. %s placeholders are, in order:
//   1. agent-id    — stamped into $LocalHostName so the shell routes to
//                    /var/log/claude-agent/<agent-id>.log
//   2. shell-host  — DNS / IP of the claude-shell listener
const rsyslogClientConfigTpl = `# Managed by claude-host init-agent. Do not edit by hand.

# Stamp messages with our agent-id as hostname so the shell receiver
# routes them to /var/log/claude-agent/<agent-id>.log.
$LocalHostName %s

module(load="omfwd")

global(
    DefaultNetstreamDriver="gtls"
    DefaultNetstreamDriverCAFile="%s/ca.crt"
    DefaultNetstreamDriverCertFile="%s/client.crt"
    DefaultNetstreamDriverKeyFile="%s/client.key"
)

action(
    type="omfwd"
    target="%s"
    port="514"
    protocol="tcp"
    StreamDriver="gtls"
    StreamDriverMode="1"
    StreamDriverAuthMode="x509/certvalid"
    queue.type="LinkedList"
    queue.size="10000"
    queue.spoolDirectory="/var/spool/rsyslog"
    queue.filename="claude-shell-fwd"
    queue.saveOnShutdown="on"
    action.resumeRetryCount="-1"
)
`

// configureAgentRsyslogClient issues a client cert for agentID under the
// shell's CA (read from the local filesystem), uploads the TLS material +
// rsyslog config to the agent, installs rsyslog-gnutls, and restarts the
// daemon so the agent immediately starts shipping logs.
//
// Returns a clear error if init-shell hasn't run yet (no CA on the local
// machine) because issuing a client cert without the CA is impossible.
func configureAgentRsyslogClient(ctx context.Context, r Runner, localShellEtcDir, agentID, shellHost string, log io.Writer) error {
	caDir := filepath.Join(localShellEtcDir, "rsyslog-ca")
	caCertPEM, err := os.ReadFile(filepath.Join(caDir, "ca.crt"))
	if err != nil {
		return fmt.Errorf("read CA cert from %s (run 'claude-host init-shell' first): %w", caDir, err)
	}
	caKeyPEM, err := os.ReadFile(filepath.Join(caDir, "ca.key"))
	if err != nil {
		return fmt.Errorf("read CA key from %s: %w", caDir, err)
	}
	ca, err := tlsutil.ParseKeyMaterial(caCertPEM, caKeyPEM)
	if err != nil {
		return fmt.Errorf("parse CA material: %w", err)
	}

	client, err := tlsutil.SignCert(ca, tlsutil.SignOptions{
		CommonName: agentID,
		// Client cert — no DNS SANs, ClientAuth usage.
	})
	if err != nil {
		return fmt.Errorf("sign client cert: %w", err)
	}
	fmt.Fprintf(log, "  issued client cert for agent=%s (valid 1y)\n", agentID)

	// Stage the tls dir on the agent.
	if err := r.Run(ctx, "mkdir -p "+RsyslogAgentTLSDir,
		RunOptions{Sudo: true, Stdout: log, Stderr: log}); err != nil {
		return fmt.Errorf("mkdir %s: %w", RsyslogAgentTLSDir, err)
	}

	pushes := []struct {
		name    string
		path    string
		content []byte
		mode    os.FileMode
	}{
		{"CA cert", RsyslogAgentTLSDir + "/ca.crt", caCertPEM, 0644},
		{"client cert", RsyslogAgentTLSDir + "/client.crt", client.CertPEM, 0644},
		{"client key", RsyslogAgentTLSDir + "/client.key", client.KeyPEM, 0600},
	}
	for _, p := range pushes {
		if err := writeRemoteContent(ctx, r, log, p.content, p.path, p.mode, "root:root"); err != nil {
			return fmt.Errorf("upload %s: %w", p.name, err)
		}
	}

	// Install rsyslog-gnutls so GnuTLS driver is available.
	if err := r.Run(ctx, "DEBIAN_FRONTEND=noninteractive apt-get install -y rsyslog-gnutls",
		RunOptions{Sudo: true, Stdout: log, Stderr: log}); err != nil {
		return fmt.Errorf("install rsyslog-gnutls: %w", err)
	}

	// Rendered config — trim shell-host to drop any trailing whitespace
	// callers might have passed in.
	shellHost = strings.TrimSpace(shellHost)
	cfg := fmt.Sprintf(rsyslogClientConfigTpl,
		agentID,
		RsyslogAgentTLSDir, RsyslogAgentTLSDir, RsyslogAgentTLSDir,
		shellHost,
	)
	if err := writeRemoteContent(ctx, r, log, []byte(cfg),
		"/etc/rsyslog.d/10-claude-shell-client.conf", 0644, "root:root"); err != nil {
		return err
	}

	// /var/spool/rsyslog must exist before the daemon restarts for the
	// disk-assisted queue configured in the template.
	if err := r.Run(ctx, "mkdir -p /var/spool/rsyslog && chmod 0755 /var/spool/rsyslog",
		RunOptions{Sudo: true, Stdout: log, Stderr: log}); err != nil {
		return err
	}

	if err := r.Run(ctx, "systemctl restart rsyslog",
		RunOptions{Sudo: true, Stdout: log, Stderr: log}); err != nil {
		return err
	}
	return nil
}
