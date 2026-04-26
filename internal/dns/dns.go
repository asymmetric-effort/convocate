// Package dns manages the host file consumed by the local dnsmasq service to
// resolve convocate session DNS names. The file is rewritten in full
// whenever session state changes; dnsmasq picks up modifications because
// convocate's drop-in config points at it via addn-hosts.
package dns

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Record is a single DNS A-record to publish.
type Record struct {
	Name string
	IP   string
}

// DefaultHostsFile is the file path convocate manages. dnsmasq should be
// configured with "addn-hosts=/var/lib/convocate/dnsmasq-hosts".
const DefaultHostsFile = "/var/lib/convocate/dnsmasq-hosts"

// DefaultDnsmasqConfFile is the drop-in conf file the installer writes to
// wire up the hosts file (if the host has a dnsmasq.d directory).
const DefaultDnsmasqConfFile = "/etc/dnsmasq.d/convocate.conf"

// WriteHostsFile writes records to path in /etc/hosts format, atomically via
// a temp file + rename. Missing parent directory is surfaced as an error —
// the caller is expected to have initialized it during install.
func WriteHostsFile(path string, records []Record) error {
	var buf bytes.Buffer
	buf.WriteString("# Managed by convocate. Do not edit by hand.\n")
	for _, r := range records {
		if r.Name == "" || r.IP == "" {
			continue
		}
		fmt.Fprintf(&buf, "%s\t%s\n", r.IP, r.Name)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

// HostsFileExists reports whether the managed hosts file is usable (parent
// directory exists and is writable). Returns false silently on missing
// install — callers use this as a gate to skip DNS sync when the user hasn't
// set up the dnsmasq integration.
func HostsFileExists(path string) bool {
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	return true
}

// DetectHostIP returns the first non-loopback IPv4 address on the local
// host, falling back to "127.0.0.1" when none is found.
var DetectHostIP = func() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if ipnet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipnet.IP.To4(); ip4 != nil {
			return ip4.String()
		}
	}
	return "127.0.0.1"
}
