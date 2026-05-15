package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asymmetric-effort/convocate/internal/mtls"
)

func setTestDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.Setenv("CONVOCATE_DATA_DIR", dir)
	t.Cleanup(func() { os.Unsetenv("CONVOCATE_DATA_DIR") })
	return dir
}

func TestRunHelp(t *testing.T) {
	exitCode := Run([]string{"help"})
	if exitCode != 0 {
		t.Errorf("help exit code: got %d, want 0", exitCode)
	}
}

func TestRunNoArgs(t *testing.T) {
	exitCode := Run([]string{})
	if exitCode != 1 {
		t.Errorf("no args exit code: got %d, want 1", exitCode)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	exitCode := Run([]string{"foobar"})
	if exitCode != 1 {
		t.Errorf("unknown command exit code: got %d, want 1", exitCode)
	}
}

func TestCAGenerate(t *testing.T) {
	dir := setTestDataDir(t)

	exitCode := Run([]string{"ca", "generate"})
	if exitCode != 0 {
		t.Fatalf("ca generate exit code: got %d, want 0", exitCode)
	}

	// Check cert file.
	certPath := filepath.Join(dir, "ca.crt")
	data, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	if !strings.Contains(string(data), "CERTIFICATE") {
		t.Error("cert file doesn't contain CERTIFICATE")
	}

	// Check key file.
	keyPath := filepath.Join(dir, "ca.key")
	data, err = os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if !strings.Contains(string(data), "EC PRIVATE KEY") {
		t.Error("key file doesn't contain EC PRIVATE KEY")
	}

	// Check key permissions.
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("key permissions: got %o, want 600", info.Mode().Perm())
	}
}

func TestCAPrintBundle(t *testing.T) {
	dir := setTestDataDir(t)

	// Generate CA first.
	exitCode := Run([]string{"ca", "generate"})
	if exitCode != 0 {
		t.Fatalf("ca generate failed: %d", exitCode)
	}

	// Print bundle (output goes to stdout — we just check exit code).
	exitCode = Run([]string{"ca", "print-bundle"})
	if exitCode != 0 {
		t.Errorf("ca print-bundle exit code: got %d, want 0", exitCode)
	}

	_ = dir
}

func TestCAPrintBundleNoCA(t *testing.T) {
	setTestDataDir(t)
	exitCode := Run([]string{"ca", "print-bundle"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 when no CA exists, got %d", exitCode)
	}
}

func TestCANoSubcommand(t *testing.T) {
	exitCode := Run([]string{"ca"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for missing subcommand, got %d", exitCode)
	}
}

func TestCAUnknownSubcommand(t *testing.T) {
	exitCode := Run([]string{"ca", "foobar"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for unknown subcommand, got %d", exitCode)
	}
}

func TestHostIssueCert(t *testing.T) {
	setTestDataDir(t)

	// Generate CA first.
	exitCode := Run([]string{"ca", "generate"})
	if exitCode != 0 {
		t.Fatalf("ca generate failed: %d", exitCode)
	}

	// Issue host cert (output goes to stdout).
	exitCode = Run([]string{"host", "issue-cert", "agent-host-1"})
	if exitCode != 0 {
		t.Errorf("host issue-cert exit code: got %d, want 0", exitCode)
	}
}

func TestHostIssueCertNoCA(t *testing.T) {
	setTestDataDir(t)
	exitCode := Run([]string{"host", "issue-cert", "agent-host-1"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 when no CA exists, got %d", exitCode)
	}
}

func TestHostIssueCertNoHostID(t *testing.T) {
	exitCode := Run([]string{"host", "issue-cert"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for missing host ID, got %d", exitCode)
	}
}

func TestHostNoSubcommand(t *testing.T) {
	exitCode := Run([]string{"host"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for missing subcommand, got %d", exitCode)
	}
}

func TestHostUnknownSubcommand(t *testing.T) {
	exitCode := Run([]string{"host", "foobar"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for unknown subcommand, got %d", exitCode)
	}
}

func TestOpenBaoInit(t *testing.T) {
	dir := setTestDataDir(t)

	exitCode := Run([]string{"openbao", "init"})
	if exitCode != 0 {
		t.Fatalf("openbao init exit code: got %d, want 0", exitCode)
	}

	keyPath := filepath.Join(dir, "openbao-unseal.key")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}

	// Should be 64 hex chars + newline.
	keyHex := strings.TrimSpace(string(data))
	if len(keyHex) != 64 {
		t.Errorf("key length: got %d, want 64 hex chars", len(keyHex))
	}

	// Check permissions.
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key: %v", err)
	}
	if info.Mode().Perm() != 0o400 {
		t.Errorf("key permissions: got %o, want 400", info.Mode().Perm())
	}
}

func TestOpenBaoNoSubcommand(t *testing.T) {
	exitCode := Run([]string{"openbao"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for missing subcommand, got %d", exitCode)
	}
}

func TestOpenBaoUnknownSubcommand(t *testing.T) {
	exitCode := Run([]string{"openbao", "foobar"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for unknown subcommand, got %d", exitCode)
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://router.example.com", "router.example.com"},
		{"https://router.example.com:8443", "router.example.com"},
		{"http://localhost:8443", "localhost"},
		{"https://192.168.1.1:443", "192.168.1.1"},
		{"router.example.com", "router.example.com"},
		{"router.example.com/path", "router.example.com"},
	}
	for _, testCase := range tests {
		t.Run(testCase.input, func(t *testing.T) {
			got := extractHost(testCase.input)
			if got != testCase.want {
				t.Errorf("extractHost(%q) = %q, want %q", testCase.input, got, testCase.want)
			}
		})
	}
}

func TestIssueServerCert(t *testing.T) {
	ca, err := mtls.GenerateCA("test-ca", DefaultCAValidity)
	if err != nil {
		t.Fatalf("GenerateCA error: %v", err)
	}

	pair, err := IssueServerCert(ca, "https://router.example.com:8443")
	if err != nil {
		t.Fatalf("IssueServerCert error: %v", err)
	}
	if len(pair.CertPEM) == 0 {
		t.Error("CertPEM is empty")
	}
	if len(pair.KeyPEM) == 0 {
		t.Error("KeyPEM is empty")
	}
}

func TestIssueServerCertLocalhost(t *testing.T) {
	ca, err := mtls.GenerateCA("test-ca", DefaultCAValidity)
	if err != nil {
		t.Fatalf("GenerateCA error: %v", err)
	}

	pair, err := IssueServerCert(ca, "https://localhost:8443")
	if err != nil {
		t.Fatalf("IssueServerCert error: %v", err)
	}
	if len(pair.CertPEM) == 0 {
		t.Error("CertPEM is empty")
	}
}
