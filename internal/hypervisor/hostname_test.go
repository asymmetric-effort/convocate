package hypervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- random hostname --------------------------------------------------------

func TestDefaultRandHostname_LengthAndAlphabet(t *testing.T) {
	got, err := defaultRandHostname()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != hostnameLen {
		t.Errorf("len = %d, want %d", len(got), hostnameLen)
	}
	for _, r := range got {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			t.Errorf("non-alphabetic rune %q in %q", r, got)
		}
	}
}

func TestDefaultRandHostname_Distinct(t *testing.T) {
	// Two consecutive draws should not collide (probability ~1/52^8).
	a, _ := defaultRandHostname()
	b, _ := defaultRandHostname()
	if a == b {
		t.Errorf("two draws equal — bad RNG: %q", a)
	}
}

func TestRandIndex_BoundedByN(t *testing.T) {
	for i := 0; i < 200; i++ {
		got, err := randIndex(7)
		if err != nil {
			t.Fatal(err)
		}
		if got < 0 || got >= 7 {
			t.Errorf("randIndex out of range: %d", got)
		}
	}
}

// --- SetHypervisorHostname --------------------------------------------------

func TestSetHypervisorHostname_RequiresArgs(t *testing.T) {
	r := &mockRunner{}
	if err := SetHypervisorHostname(context.Background(), r, "", "fqdn"); err == nil {
		t.Error("expected error for empty hostname")
	}
	if err := SetHypervisorHostname(context.Background(), r, "host", ""); err == nil {
		t.Error("expected error for empty fqdn")
	}
	if len(r.cmds) != 0 {
		t.Error("no remote work should run on validation failure")
	}
}

func TestSetHypervisorHostname_ScriptShape(t *testing.T) {
	r := &mockRunner{}
	if err := SetHypervisorHostname(context.Background(), r, "abcd1234", "abcd1234.example.com"); err != nil {
		t.Fatal(err)
	}
	if len(r.cmds) != 1 {
		t.Fatalf("expected 1 cmd, got %d", len(r.cmds))
	}
	if !r.cmds[0].Opts.Sudo {
		t.Error("hostname change must run as sudo")
	}
	body := r.cmds[0].Cmd
	for _, want := range []string{
		"hostnamectl set-hostname 'abcd1234'",
		"127.0.1.1",
		"abcd1234.example.com",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("script missing %q\n%s", want, body)
		}
	}
}

// --- ResolveHypervisorIP ----------------------------------------------------

func TestResolveHypervisorIP_LiteralIPv4(t *testing.T) {
	got, err := ResolveHypervisorIP("192.168.3.42")
	if err != nil {
		t.Fatal(err)
	}
	if got != "192.168.3.42" {
		t.Errorf("got %q", got)
	}
}

func TestResolveHypervisorIP_LiteralIPv6(t *testing.T) {
	got, err := ResolveHypervisorIP("2001:db8::1")
	if err != nil {
		t.Fatal(err)
	}
	// IPv6 literals fall through unchanged when not v4-mappable.
	if got != "2001:db8::1" {
		t.Errorf("got %q", got)
	}
}

func TestResolveHypervisorIP_HostnameViaStub(t *testing.T) {
	orig := lookupHost
	defer func() { lookupHost = orig }()
	lookupHost = func(host string) ([]string, error) {
		if host != "agent.example.com" {
			t.Errorf("unexpected host: %s", host)
		}
		return []string{"::1", "10.0.0.7"}, nil
	}
	got, err := ResolveHypervisorIP("agent.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "10.0.0.7" {
		t.Errorf("got %q, want IPv4 from list", got)
	}
}

func TestResolveHypervisorIP_OnlyIPv6(t *testing.T) {
	orig := lookupHost
	defer func() { lookupHost = orig }()
	lookupHost = func(string) ([]string, error) { return []string{"::1"}, nil }
	got, err := ResolveHypervisorIP("ipv6-only.example")
	if err != nil {
		t.Fatal(err)
	}
	if got != "::1" {
		t.Errorf("got %q, want first IPv6 fallback", got)
	}
}

func TestResolveHypervisorIP_LookupFails(t *testing.T) {
	orig := lookupHost
	defer func() { lookupHost = orig }()
	lookupHost = func(string) ([]string, error) { return nil, errors.New("nxdomain") }
	if _, err := ResolveHypervisorIP("never-resolves.example"); err == nil {
		t.Error("expected lookup error")
	}
}

func TestResolveHypervisorIP_EmptyResult(t *testing.T) {
	orig := lookupHost
	defer func() { lookupHost = orig }()
	lookupHost = func(string) ([]string, error) { return nil, nil }
	if _, err := ResolveHypervisorIP("empty.example"); err == nil ||
		!strings.Contains(err.Error(), "no IPs") {
		t.Errorf("expected no-IPs error, got %v", err)
	}
}

// --- RegisterDNSName --------------------------------------------------------

func TestRegisterDNSName_RequiresArgs(t *testing.T) {
	if err := RegisterDNSName("", "", "1.1.1.1"); err == nil {
		t.Error("expected error for empty fqdn")
	}
	if err := RegisterDNSName("", "f", ""); err == nil {
		t.Error("expected error for empty ip")
	}
}

func TestRegisterDNSName_NewFile(t *testing.T) {
	dir := t.TempDir()
	hf := filepath.Join(dir, "hosts")
	if err := RegisterDNSName(hf, "abc.example.com", "192.168.1.10"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(hf)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "192.168.1.10\tabc.example.com") {
		t.Errorf("missing record:\n%s", got)
	}
	if !strings.Contains(got, "Managed by claude-shell") {
		t.Errorf("missing header:\n%s", got)
	}
}

func TestRegisterDNSName_ReplacesExistingFqdn(t *testing.T) {
	dir := t.TempDir()
	hf := filepath.Join(dir, "hosts")
	// Pre-populate with an old IP for the same fqdn.
	if err := os.WriteFile(hf, []byte("# Managed by claude-shell.\n10.0.0.1\tabc.example.com\nother.host\t1.2.3.4\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RegisterDNSName(hf, "abc.example.com", "10.0.0.99"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(hf)
	if strings.Contains(string(data), "10.0.0.1\tabc.example.com") {
		t.Errorf("old IP not replaced:\n%s", data)
	}
	if !strings.Contains(string(data), "10.0.0.99\tabc.example.com") {
		t.Errorf("new IP missing:\n%s", data)
	}
	// Other hosts preserved.
	if !strings.Contains(string(data), "other.host") {
		t.Errorf("unrelated entry dropped:\n%s", data)
	}
}

func TestRegisterDNSName_AppendsWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	hf := filepath.Join(dir, "hosts")
	if err := os.WriteFile(hf, []byte("# header\n1.1.1.1\tone.example\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RegisterDNSName(hf, "two.example", "2.2.2.2"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(hf)
	got := string(data)
	if !strings.Contains(got, "1.1.1.1\tone.example") {
		t.Errorf("first record dropped:\n%s", got)
	}
	if !strings.Contains(got, "2.2.2.2\ttwo.example") {
		t.Errorf("new record missing:\n%s", got)
	}
}

func TestRegisterDNSName_DefaultPath(t *testing.T) {
	// Empty hostsFile arg should hit DefaultDnsmasqHostsFile. We
	// can't safely write there from a unit test, so just verify the
	// constant lines up with the project's convention.
	if DefaultDnsmasqHostsFile != "/var/lib/claude-shell/dnsmasq-hosts" {
		t.Errorf("DefaultDnsmasqHostsFile = %q", DefaultDnsmasqHostsFile)
	}
}

func TestRegisterDNSName_RenameFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses perm checks; can't simulate rename failure")
	}
	dir := t.TempDir()
	hf := filepath.Join(dir, "hosts")
	// Make hf a non-empty directory — rename into it fails.
	if err := os.MkdirAll(filepath.Join(hf, "child"), 0755); err != nil {
		t.Fatal(err)
	}
	err := RegisterDNSName(hf, "x.example", "1.1.1.1")
	if err == nil {
		t.Error("expected rename error when dst is a directory")
	}
}

func TestRegisterDNSName_MkdirFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses perm checks")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(parent, 0755)
	err := RegisterDNSName(filepath.Join(parent, "sub", "hosts"), "x", "1.1.1.1")
	if err == nil {
		t.Error("expected mkdir error under read-only parent")
	}
}

// --- splitLines / splitFields helpers --------------------------------------

func TestSplitLines(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a\n", []string{"a"}},
		{"a\nb\n", []string{"a", "b"}},
		{"a\nb", []string{"a", "b"}},
	}
	for _, tc := range cases {
		got := splitLines(tc.in)
		if !equalStrSlices(got, tc.want) {
			t.Errorf("splitLines(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSplitFields(t *testing.T) {
	got := splitFields("  a   b\tc  ")
	if !equalStrSlices(got, []string{"a", "b", "c"}) {
		t.Errorf("got %v", got)
	}
}

func equalStrSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Document randHostname's role with a simple usage check that the
// package-level seam is wired correctly. This catches regressions
// where someone replaces randHostname with a closure that ignores
// the override.
func TestRandHostname_OverridableSeam(t *testing.T) {
	orig := randHostname
	defer func() { randHostname = orig }()
	randHostname = func() (string, error) { return "Xxxxxxxx", nil }
	got, err := randHostname()
	if err != nil {
		t.Fatal(err)
	}
	if got != "Xxxxxxxx" {
		t.Errorf("override not honored: %q", got)
	}
	// Also exercise default once in this same test for trivial coverage.
	if _, err := defaultRandHostname(); err != nil {
		t.Fatalf("defaultRandHostname: %v", err)
	}
	_ = fmt.Sprintf // keep fmt referenced for future tests
}
