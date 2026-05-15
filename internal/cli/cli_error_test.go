package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/mtls"
)

func testCA(t *testing.T) *mtls.CA {
	t.Helper()
	ca, err := mtls.GenerateCA("test-ca", 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	return ca
}

// TestCAGenerateWriteError tests the error path when ca.crt/ca.key
// cannot be written (e.g., to a read-only directory).
func TestCAGenerateWriteError(t *testing.T) {
	dir := t.TempDir()
	readOnlyDir := filepath.Join(dir, "readonly")
	err := os.MkdirAll(readOnlyDir, 0o500)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Make it writable for cleanup.
	t.Cleanup(func() { os.Chmod(readOnlyDir, 0o700) })

	// Point data dir to a subdirectory of the read-only dir (cannot be created).
	os.Setenv("CONVOCATE_DATA_DIR", filepath.Join(readOnlyDir, "subdir"))
	t.Cleanup(func() { os.Unsetenv("CONVOCATE_DATA_DIR") })

	exitCode := caGenerate()
	if exitCode != 1 {
		t.Errorf("caGenerate should fail, got exit code %d", exitCode)
	}
}

// TestHostIssueCertNoKeyFile tests when CA key file is missing.
func TestHostIssueCertNoKeyFile(t *testing.T) {
	dir := setTestDataDir(t)

	// Generate CA first.
	exitCode := caGenerate()
	if exitCode != 0 {
		t.Fatalf("caGenerate: %d", exitCode)
	}

	// Remove the key file.
	os.Remove(filepath.Join(dir, "ca.key"))

	exitCode = hostIssueCert("host-no-key")
	if exitCode != 1 {
		t.Errorf("hostIssueCert should fail without key, got %d", exitCode)
	}
}

// TestHostIssueCertCorruptCA tests when CA files are corrupt.
func TestHostIssueCertCorruptCA(t *testing.T) {
	dir := setTestDataDir(t)

	// Write corrupt CA files.
	os.WriteFile(filepath.Join(dir, "ca.crt"), []byte("not a cert"), 0o600)
	os.WriteFile(filepath.Join(dir, "ca.key"), []byte("not a key"), 0o600)

	exitCode := hostIssueCert("host-corrupt")
	if exitCode != 1 {
		t.Errorf("hostIssueCert should fail with corrupt CA, got %d", exitCode)
	}
}

// TestOpenbaoInitWriteError tests when the bootstrap key cannot be written.
func TestOpenbaoInitWriteError(t *testing.T) {
	dir := t.TempDir()
	readOnlyDir := filepath.Join(dir, "readonly")
	err := os.MkdirAll(readOnlyDir, 0o500)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { os.Chmod(readOnlyDir, 0o700) })

	os.Setenv("CONVOCATE_DATA_DIR", filepath.Join(readOnlyDir, "subdir"))
	t.Cleanup(func() { os.Unsetenv("CONVOCATE_DATA_DIR") })

	exitCode := openbaoInit()
	if exitCode != 1 {
		t.Errorf("openbaoInit should fail, got exit code %d", exitCode)
	}
}

// TestCaDataDirFromEnv tests the CONVOCATE_DATA_DIR env var.
func TestCaDataDirFromEnv(t *testing.T) {
	os.Setenv("CONVOCATE_DATA_DIR", "/custom/path")
	defer os.Unsetenv("CONVOCATE_DATA_DIR")

	dir := caDataDir()
	if dir != "/custom/path" {
		t.Errorf("caDataDir: got %q, want /custom/path", dir)
	}
}

// TestCaDataDirDefault tests the default path.
func TestCaDataDirDefault(t *testing.T) {
	os.Unsetenv("CONVOCATE_DATA_DIR")
	dir := caDataDir()
	if dir != "/var/lib/convocate" {
		t.Errorf("caDataDir: got %q, want /var/lib/convocate", dir)
	}
}

// TestIssueServerCertEmpty tests IssueServerCert with empty URL.
func TestIssueServerCertEmpty(t *testing.T) {
	dir := setTestDataDir(t)
	_ = dir
	// Generate a CA inline.
	ca := testCA(t)
	pair, err := IssueServerCert(ca, "")
	if err != nil {
		t.Fatalf("IssueServerCert: %v", err)
	}
	if len(pair.CertPEM) == 0 {
		t.Error("CertPEM is empty")
	}
}

// TestIssueServerCertWithIP tests IssueServerCert with an IP URL.
func TestIssueServerCertWithIP(t *testing.T) {
	ca := testCA(t)
	pair, err := IssueServerCert(ca, "https://192.168.1.100:8443")
	if err != nil {
		t.Fatalf("IssueServerCert: %v", err)
	}
	if len(pair.CertPEM) == 0 {
		t.Error("CertPEM is empty")
	}
}

// TestCAGenerateKeyWriteError tests when ca.crt can be created but ca.key fails.
func TestCAGenerateKeyWriteError(t *testing.T) {
	dir := t.TempDir()
	// Create dir writable, then make a subdirectory for data.
	dataDir := filepath.Join(dir, "data")
	os.MkdirAll(dataDir, 0o700)
	os.Setenv("CONVOCATE_DATA_DIR", dataDir)
	t.Cleanup(func() { os.Unsetenv("CONVOCATE_DATA_DIR") })

	// Write ca.crt as a directory to make the second write fail.
	os.MkdirAll(filepath.Join(dataDir, "ca.key"), 0o700)

	exitCode := caGenerate()
	if exitCode != 1 {
		t.Errorf("expected exit 1 when key write fails, got %d", exitCode)
	}
	// Clean up the directory.
	os.RemoveAll(filepath.Join(dataDir, "ca.key"))
}

// TestOpenbaoInitKeyWriteError tests when key file path is a directory.
func TestOpenbaoInitKeyWriteError(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	os.MkdirAll(dataDir, 0o700)
	os.Setenv("CONVOCATE_DATA_DIR", dataDir)
	t.Cleanup(func() { os.Unsetenv("CONVOCATE_DATA_DIR") })

	// Make the key path a directory so write fails.
	os.MkdirAll(filepath.Join(dataDir, "openbao-unseal.key"), 0o700)

	exitCode := openbaoInit()
	if exitCode != 1 {
		t.Errorf("expected exit 1, got %d", exitCode)
	}
	os.RemoveAll(filepath.Join(dataDir, "openbao-unseal.key"))
}

// TestRunHelpAliases tests --help and -h aliases.
func TestRunHelpAliases(t *testing.T) {
	if Run([]string{"--help"}) != 0 {
		t.Error("--help should return 0")
	}
	if Run([]string{"-h"}) != 0 {
		t.Error("-h should return 0")
	}
}
