package hypervisor

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// hostnameAlphabet is the character set used for random hypervisor
// hostnames. Mixed-case alphabetic per the create-vm spec — no
// digits, no symbols. 52^8 ≈ 5.3×10^13 distinct names so collisions
// in any realistic cluster are vanishingly unlikely.
const hostnameAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// hostnameLen is the random hostname length per the spec.
const hostnameLen = 8

// randHostname generates an 8-char [A-Za-z] string. Overridable in
// tests for determinism.
var randHostname = defaultRandHostname

func defaultRandHostname() (string, error) {
	buf := make([]byte, hostnameLen)
	for i := range buf {
		idx, err := randIndex(len(hostnameAlphabet))
		if err != nil {
			return "", err
		}
		buf[i] = hostnameAlphabet[idx]
	}
	return string(buf), nil
}

// randIndex returns a uniform-ish index into a slice of length n.
// Modulo bias against a single byte at n=52 is negligible (~1 in 2.6M
// extra weight on indices [0,3]); accepted.
func randIndex(n int) (int, error) {
	var b [1]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, fmt.Errorf("rand read: %w", err)
	}
	return int(b[0]) % n, nil
}

// SetHypervisorHostname writes the new hostname on the remote host
// via `hostnamectl` + a /etc/hosts append so root@<hostname> resolves
// before any further apt or libvirt commands run. fqdn is also
// stamped in the hosts file as 127.0.1.1 → <fqdn> <hostname>, which
// matches Ubuntu's stock resolution behavior.
func SetHypervisorHostname(ctx context.Context, r Runner, hostname, fqdn string) error {
	if hostname == "" || fqdn == "" {
		return errors.New("SetHypervisorHostname: hostname and fqdn are required")
	}
	cmd := fmt.Sprintf(`set -e
hostnamectl set-hostname %s
# Replace any existing 127.0.1.1 line with our managed value, or add
# one if it's missing — keeps the file canonical.
if grep -q '^127.0.1.1' /etc/hosts; then
  sed -i 's|^127\.0\.1\.1.*$|127.0.1.1\t%s %s|' /etc/hosts
else
  printf '127.0.1.1\t%%s %%s\n' %s %s >> /etc/hosts
fi
`, shellQuoteArg(hostname),
		fqdn, hostname,
		shellQuoteArg(fqdn), shellQuoteArg(hostname))

	return r.Run(ctx, cmd, RunOptions{Sudo: true})
}

// ResolveHypervisorIP figures out the IPv4 address claude-shell's
// dnsmasq should publish for the new hypervisor. If hypervisor is
// already an IP literal it's returned verbatim; otherwise we
// net.LookupHost it and pick the first IPv4 answer.
func ResolveHypervisorIP(hypervisor string) (string, error) {
	if ip := net.ParseIP(hypervisor); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return v4.String(), nil
		}
		return hypervisor, nil
	}
	ips, err := lookupHost(hypervisor)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", hypervisor, err)
	}
	for _, s := range ips {
		if ip := net.ParseIP(s); ip != nil && ip.To4() != nil {
			return s, nil
		}
	}
	if len(ips) > 0 {
		return ips[0], nil
	}
	return "", fmt.Errorf("no IPs for %s", hypervisor)
}

// lookupHost is exposed as a var so tests can stub DNS without making
// real network calls (or being subject to CI network flakes).
var lookupHost = net.LookupHost

// DefaultDnsmasqHostsFile is where claude-shell publishes records for
// the cluster. claude-host create-vm appends a record per new
// hypervisor so it resolves cluster-wide as <hostname>.<domain>.
const DefaultDnsmasqHostsFile = "/var/lib/claude-shell/dnsmasq-hosts"

// RegisterDNSName appends an A record (ip → fqdn) to the shell host's
// dnsmasq-hosts file. Idempotent: the function reads the file, looks
// for an existing line with the same fqdn, replaces it on a name
// match, and only writes back when content changed. Atomic rename
// avoids a torn file dnsmasq might race-load.
//
// hostsFile defaults to DefaultDnsmasqHostsFile when empty.
func RegisterDNSName(hostsFile, fqdn, ip string) error {
	if fqdn == "" || ip == "" {
		return errors.New("RegisterDNSName: fqdn and ip are required")
	}
	if hostsFile == "" {
		hostsFile = DefaultDnsmasqHostsFile
	}
	existing, err := os.ReadFile(hostsFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", hostsFile, err)
	}

	want := fmt.Sprintf("%s\t%s", ip, fqdn)
	lines := splitLines(string(existing))
	out := make([]string, 0, len(lines)+1)
	replaced := false
	for _, ln := range lines {
		if ln == "" {
			continue
		}
		// Match by trailing fqdn so we replace any old IP for this
		// host without piling on duplicates.
		fields := splitFields(ln)
		if len(fields) >= 2 && fields[len(fields)-1] == fqdn {
			out = append(out, want)
			replaced = true
			continue
		}
		out = append(out, ln)
	}
	if !replaced {
		out = append(out, want)
	}

	header := "# Managed by claude-shell. Do not edit by hand.\n"
	body := strings.Join(stripHeader(out), "\n") + "\n"

	tmp := hostsFile + ".tmp"
	if err := os.MkdirAll(filepath.Dir(hostsFile), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(hostsFile), err)
	}
	if err := os.WriteFile(tmp, []byte(header+body), 0644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, hostsFile); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s -> %s: %w", tmp, hostsFile, err)
	}
	return nil
}

// stripHeader drops the project's "# Managed by ..." comment and any
// blank lines from the input so RegisterDNSName can reapply it
// canonically. Operators editing the file by hand violate the
// "do not edit" comment anyway; their changes survive only if they
// look like data lines.
func stripHeader(lines []string) []string {
	out := lines[:0]
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		out = append(out, ln)
	}
	return out
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func splitFields(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
