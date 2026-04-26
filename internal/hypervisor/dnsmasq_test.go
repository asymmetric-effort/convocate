package hypervisor

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestConfigureHypervisorDnsmasq_RequiresShellIP(t *testing.T) {
	err := ConfigureHypervisorDnsmasq(context.Background(), &mockRunner{}, "")
	if err == nil || !strings.Contains(err.Error(), "shellIP") {
		t.Errorf("expected shellIP error, got %v", err)
	}
}

func TestConfigureHypervisorDnsmasq_WiresShellAndGateway(t *testing.T) {
	m := &mockRunner{
		cmdStdout: map[string]string{
			"ip -4 route show default": "default via 192.168.3.1 dev enp0s31f6 proto static\n",
		},
	}
	err := ConfigureHypervisorDnsmasq(context.Background(), m, "192.168.3.90")
	if err != nil {
		t.Fatalf("ConfigureHypervisorDnsmasq: %v", err)
	}

	// Three remote scripts: detect gateway, disable stub, install
	// dnsmasq, write config. Plus the inner detect = 4 total cmds.
	// First cmd is the gateway detection.
	if !strings.Contains(m.cmds[0].Cmd, "ip -4 route show default") {
		t.Errorf("cmd 0 should detect gateway, got %q", m.cmds[0].Cmd)
	}

	// Find the dnsmasq config write — should embed both server lines.
	confSeen := false
	for _, c := range m.cmds {
		if strings.Contains(c.Cmd, "/etc/dnsmasq.d/10-convocate.conf") {
			confSeen = true
			for _, want := range []string{
				"server=192.168.3.90",
				"server=192.168.3.1",
				"cache-size=1000",
				"domain-needed",
				"bogus-priv",
				"systemctl restart dnsmasq",
			} {
				if !strings.Contains(c.Cmd, want) {
					t.Errorf("dnsmasq config missing %q\n%s", want, c.Cmd)
				}
			}
		}
	}
	if !confSeen {
		t.Error("dnsmasq config write step never ran")
	}

	// Resolved stub-disable + dnsmasq install both surface.
	joined := allHypervisorCmds(m.cmds)
	for _, want := range []string{
		"DNSStubListener=no",
		"apt-get install -y dnsmasq",
		"systemctl restart systemd-resolved",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("phase missing %q", want)
		}
	}
}

func TestConfigureHypervisorDnsmasq_DefaultGatewayFails(t *testing.T) {
	m := &mockRunner{
		failOn: map[string]error{
			"ip -4 route show default": errors.New("network down"),
		},
	}
	err := ConfigureHypervisorDnsmasq(context.Background(), m, "1.1.1.1")
	if err == nil || !strings.Contains(err.Error(), "default gateway") {
		t.Errorf("expected gateway error, got %v", err)
	}
}

func TestConfigureHypervisorDnsmasq_NoDefaultRoute(t *testing.T) {
	m := &mockRunner{
		cmdStdout: map[string]string{"ip -4 route show default": ""},
	}
	err := ConfigureHypervisorDnsmasq(context.Background(), m, "1.1.1.1")
	if err == nil || !strings.Contains(err.Error(), "no default route") {
		t.Errorf("expected no-default-route, got %v", err)
	}
}

func TestConfigureHypervisorDnsmasq_AptFails(t *testing.T) {
	m := &mockRunner{
		cmdStdout: map[string]string{
			"ip -4 route show default": "default via 10.0.0.1 dev eth0\n",
		},
		failOn: map[string]error{
			"DEBIAN_FRONTEND=noninteractive apt-get install -y dnsmasq": errors.New("apt locked"),
		},
	}
	err := ConfigureHypervisorDnsmasq(context.Background(), m, "10.0.0.99")
	if err == nil || !strings.Contains(err.Error(), "install dnsmasq") {
		t.Errorf("expected install dnsmasq error, got %v", err)
	}
}

func TestConfigureHypervisorDnsmasq_ResolvedFails(t *testing.T) {
	m := &mockRunner{
		cmdStdout: map[string]string{
			"ip -4 route show default": "default via 10.0.0.1 dev eth0\n",
		},
		failOn: map[string]error{
			// The resolved-disable script starts with `set -e\nmkdir`.
			"set -e\nmkdir -p /etc/systemd/resolved.conf.d": errors.New("read-only fs"),
		},
	}
	err := ConfigureHypervisorDnsmasq(context.Background(), m, "10.0.0.99")
	if err == nil || !strings.Contains(err.Error(), "resolved stub") {
		t.Errorf("expected resolved stub error, got %v", err)
	}
}

func TestDetectDefaultGateway_MultipleRoutes(t *testing.T) {
	m := &mockRunner{
		cmdStdout: map[string]string{
			"ip -4 route show default": "default via 10.0.0.1 dev eth0 metric 100\ndefault via 10.0.0.2 dev eth1 metric 200\n",
		},
	}
	got, err := detectDefaultGateway(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	// First match wins.
	if got != "10.0.0.1" {
		t.Errorf("got %q, want 10.0.0.1", got)
	}
}

func TestDetectDefaultGateway_MalformedOutput(t *testing.T) {
	m := &mockRunner{
		cmdStdout: map[string]string{
			"ip -4 route show default": "weird line without via token\n",
		},
	}
	_, err := detectDefaultGateway(context.Background(), m)
	if err == nil || !strings.Contains(err.Error(), "no `via` token") {
		t.Errorf("expected malformed-output error, got %v", err)
	}
}

// allHypervisorCmds joins every recorded Run cmd into a single
// multi-line string for substring searches in tests.
func allHypervisorCmds(cs []mockCall) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteString(c.Cmd)
		b.WriteByte('\n')
	}
	return b.String()
}
