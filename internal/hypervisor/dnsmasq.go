package hypervisor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
)

// hypervisorDnsmasqConfTpl is the /etc/dnsmasq.d drop-in installed on
// the hypervisor. Two `server=` lines define the upstream resolution
// chain: queries miss the local cache → primary (shell host's
// dnsmasq) → fallback (the box's default gateway). cache-size keeps
// frequently-resolved cluster names hot. bogus-priv + domain-needed
// suppress upstream lookups for RFC1918 reverse names that would
// otherwise leak.
//
// %s placeholders, in order: shell-host IP, default-gateway IP.
const hypervisorDnsmasqConfTpl = `# Managed by claude-host create-vm. Do not edit by hand.
# Forward all queries to the claude-shell host's dnsmasq first; fall
# back to the LAN gateway when the primary is unreachable.
server=%s
server=%s
cache-size=1000
domain-needed
bogus-priv
`

// resolvedStubDropIn disables systemd-resolved's :53 stub listener so
// dnsmasq can bind that port. Identical to the drop-in claude-host
// install lays down on the shell host — keeping them aligned means
// operators only have one shape of file to recognize.
const resolvedStubDropIn = `# Managed by claude-host create-vm. Frees port 53 for dnsmasq.
[Resolve]
DNSStubListener=no
`

// ConfigureHypervisorDnsmasq installs dnsmasq on the hypervisor and
// configures it to forward queries to the shell host first, with the
// hypervisor's own default gateway as fallback. systemd-resolved's
// stub listener is disabled so dnsmasq can bind :53.
//
// shellIP must be the IPv4 the new hypervisor will use to reach
// the shell host's dnsmasq. The default gateway is auto-detected on
// the remote.
func ConfigureHypervisorDnsmasq(ctx context.Context, r Runner, shellIP string) error {
	if shellIP == "" {
		return errors.New("ConfigureHypervisorDnsmasq: shellIP required")
	}
	gateway, err := detectDefaultGateway(ctx, r)
	if err != nil {
		return fmt.Errorf("detect default gateway: %w", err)
	}

	// 1. Disable systemd-resolved's stub listener (idempotent — the
	//    drop-in is overwritten on each create-vm).
	if err := r.Run(ctx, `set -e
mkdir -p /etc/systemd/resolved.conf.d
cat >/etc/systemd/resolved.conf.d/00-claude-dnsmasq.conf <<'RESOLV_EOF'
`+resolvedStubDropIn+`RESOLV_EOF
systemctl restart systemd-resolved || true
`, RunOptions{Sudo: true}); err != nil {
		return fmt.Errorf("disable resolved stub: %w", err)
	}

	// 2. Install dnsmasq.
	if err := r.Run(ctx,
		`DEBIAN_FRONTEND=noninteractive apt-get install -y dnsmasq`,
		RunOptions{Sudo: true},
	); err != nil {
		return fmt.Errorf("install dnsmasq: %w", err)
	}

	// 3. Write the forwarder config drop-in + (re)start dnsmasq.
	conf := fmt.Sprintf(hypervisorDnsmasqConfTpl, shellIP, gateway)
	cmd := fmt.Sprintf(`set -e
mkdir -p /etc/dnsmasq.d
cat >/etc/dnsmasq.d/10-claude-shell.conf <<'DNS_EOF'
%sDNS_EOF
systemctl restart dnsmasq
`, conf)
	if err := r.Run(ctx, cmd, RunOptions{Sudo: true}); err != nil {
		return fmt.Errorf("install dnsmasq config: %w", err)
	}
	return nil
}

// detectDefaultGateway parses `ip route show default` on the remote
// and extracts the gateway IP. Errors when the host has no default
// route — that's a misconfigured box we shouldn't try to provision.
func detectDefaultGateway(ctx context.Context, r Runner) (string, error) {
	var buf bytes.Buffer
	err := r.Run(ctx, `ip -4 route show default`, RunOptions{Stdout: &buf})
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(buf.String())
	if out == "" {
		return "", errors.New("no default route on hypervisor")
	}
	// Expected format: "default via 192.168.3.1 dev enp0s31f6 proto static".
	// We require the line start with "default" so a stray output that
	// happens to mention "via" elsewhere doesn't poison the parse.
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "default" || fields[1] != "via" {
			continue
		}
		return fields[2], nil
	}
	return "", fmt.Errorf("no `via` token in default route: %q", out)
}
