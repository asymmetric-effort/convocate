package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/asymmetric-effort/convocate/internal/mtls"
)

// DefaultCAValidity is the default validity period for the private CA (10 years).
const DefaultCAValidity = 10 * 365 * 24 * time.Hour

// DefaultCertValidity is the default validity period for host certs (1 year).
const DefaultCertValidity = 365 * 24 * time.Hour

// Run executes a convocate-cli subcommand. Returns 0 on success, 1 on error.
func Run(args []string) int {
	if len(args) < 1 {
		printUsage()
		return 1
	}

	switch args[0] {
	case "ca":
		return runCA(args[1:])
	case "host":
		return runHost(args[1:])
	case "openbao":
		return runOpenBao(args[1:])
	case "help", "--help", "-h":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		printUsage()
		return 1
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: convocate-cli <command> [args]

Commands:
  ca print-bundle                 Print the CA trust bundle PEM
  ca generate                     Generate a new private CA
  host issue-cert <host-id>       Issue an mTLS client cert for an agent host
  openbao init                    Generate the sealed bootstrap key file`)
}

// --- CA subcommands ---

func runCA(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: convocate-cli ca <print-bundle|generate> [args]")
		return 1
	}

	switch args[0] {
	case "print-bundle":
		return caPrintBundle()
	case "generate":
		return caGenerate()
	default:
		fmt.Fprintf(os.Stderr, "unknown ca command: %s\n", args[0])
		return 1
	}
}

// caPrintBundle reads the CA cert from /var/lib/convocate/ca.crt and prints it.
func caPrintBundle() int {
	caPath := caDataDir() + "/ca.crt"
	data, err := os.ReadFile(caPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading CA certificate: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: run 'convocate-cli ca generate' first\n")
		return 1
	}
	fmt.Print(string(data))
	return 0
}

// caGenerate creates a new private CA and writes the cert+key to the data dir.
func caGenerate() int {
	dir := caDataDir()
	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating data dir: %v\n", err)
		return 1
	}

	ca, err := mtls.GenerateCA("convocate-private-ca", DefaultCAValidity)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating CA: %v\n", err)
		return 1
	}

	certPath := dir + "/ca.crt"
	keyPath := dir + "/ca.key"

	err = os.WriteFile(certPath, ca.CertPEM, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing CA cert: %v\n", err)
		return 1
	}

	err = os.WriteFile(keyPath, ca.KeyPEM, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing CA key: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stderr, "CA generated:\n  cert: %s\n  key:  %s\n", certPath, keyPath)
	return 0
}

// --- Host subcommands ---

func runHost(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: convocate-cli host <issue-cert> <host-id>")
		return 1
	}

	switch args[0] {
	case "issue-cert":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: convocate-cli host issue-cert <host-id>")
			return 1
		}
		return hostIssueCert(args[1])
	default:
		fmt.Fprintf(os.Stderr, "unknown host command: %s\n", args[0])
		return 1
	}
}

// hostIssueCert issues a client cert for an agent host. The CA must
// already exist in the data dir.
func hostIssueCert(hostID string) int {
	dir := caDataDir()
	certPEM, err := os.ReadFile(dir + "/ca.crt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading CA cert: %v\nhint: run 'convocate-cli ca generate' first\n", err)
		return 1
	}
	keyPEM, err := os.ReadFile(dir + "/ca.key")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading CA key: %v\n", err)
		return 1
	}

	ca, err := mtls.LoadCA(certPEM, keyPEM)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading CA: %v\n", err)
		return 1
	}

	pair, err := ca.IssueClientCert(hostID, DefaultCertValidity)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error issuing cert: %v\n", err)
		return 1
	}

	// Output the combined cert+key PEM to stdout.
	fmt.Print(string(pair.CertPEM))
	fmt.Print(string(pair.KeyPEM))

	fmt.Fprintf(os.Stderr, "host cert issued for %q (validity: %s)\n", hostID, DefaultCertValidity)
	return 0
}

// IssueServerCert generates a server certificate for the Router API.
// This is called by the Router API at first start, not by the CLI directly.
func IssueServerCert(ca *mtls.CA, publicURL string) (*mtls.CertKeyPair, error) {
	dnsNames := []string{"localhost"}
	var ips []net.IP

	// Extract hostname from publicURL if present.
	if publicURL != "" {
		host := extractHost(publicURL)
		if host != "" && host != "localhost" {
			dnsNames = append(dnsNames, host)
		}
		ip := net.ParseIP(host)
		if ip != nil {
			ips = append(ips, ip)
		}
	}
	ips = append(ips, net.ParseIP("127.0.0.1"))

	return ca.IssueServerCert("convocate-router", dnsNames, ips, DefaultCertValidity)
}

// --- OpenBao subcommands ---

func runOpenBao(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: convocate-cli openbao <init>")
		return 1
	}

	switch args[0] {
	case "init":
		return openbaoInit()
	default:
		fmt.Fprintf(os.Stderr, "unknown openbao command: %s\n", args[0])
		return 1
	}
}

// openbaoInit generates a sealed bootstrap key file.
func openbaoInit() int {
	dir := caDataDir()
	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating data dir: %v\n", err)
		return 1
	}

	// Generate a 256-bit random key.
	key := make([]byte, 32)
	_, err = rand.Read(key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating bootstrap key: %v\n", err)
		return 1
	}

	keyHex := hex.EncodeToString(key)
	keyPath := dir + "/openbao-unseal.key"

	err = os.WriteFile(keyPath, []byte(keyHex+"\n"), 0o400)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing bootstrap key: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stderr, "OpenBao bootstrap key written to %s (mode 0400)\n", keyPath)
	fmt.Fprintf(os.Stderr, "Store this file securely out-of-band.\n")
	return 0
}

// --- Helpers ---

// caDataDir returns the data directory for CA files.
func caDataDir() string {
	dir := os.Getenv("CONVOCATE_DATA_DIR")
	if dir != "" {
		return dir
	}
	return "/var/lib/convocate"
}

// extractHost extracts the hostname from a URL.
func extractHost(rawURL string) string {
	// Simple extraction: strip scheme, strip path.
	host := rawURL
	for _, prefix := range []string{"https://", "http://"} {
		if len(host) > len(prefix) && host[:len(prefix)] == prefix {
			host = host[len(prefix):]
			break
		}
	}
	// Strip port.
	for i := range len(host) {
		if host[i] == ':' || host[i] == '/' {
			return host[:i]
		}
	}
	return host
}
