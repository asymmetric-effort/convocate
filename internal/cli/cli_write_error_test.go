package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCAGenerateCertWriteError tests when ca.crt can't be written.
func TestCAGenerateCertWriteError(t *testing.T) {
	dir := setTestDataDir(t)

	// Pre-create ca.crt as a directory so the cert write fails.
	os.MkdirAll(filepath.Join(dir, "ca.crt"), 0o700)

	exitCode := caGenerate()
	if exitCode != 1 {
		t.Errorf("expected exit 1 when cert write fails, got %d", exitCode)
	}

	// Clean up.
	os.RemoveAll(filepath.Join(dir, "ca.crt"))
}

// TestCAGenerateKeyWriteErrorOnly tests when ca.crt writes OK but ca.key fails.
func TestCAGenerateKeyWriteErrorOnly(t *testing.T) {
	dir := setTestDataDir(t)

	// Pre-create ca.key as a directory so the key write fails.
	os.MkdirAll(filepath.Join(dir, "ca.key"), 0o700)

	exitCode := caGenerate()
	if exitCode != 1 {
		t.Errorf("expected exit 1 when key write fails, got %d", exitCode)
	}

	// Clean up.
	os.RemoveAll(filepath.Join(dir, "ca.key"))
}

// TestHostIssueCertLoadCAError tests with valid PEM files but corrupt key content.
func TestHostIssueCertLoadCAError(t *testing.T) {
	dir := setTestDataDir(t)

	// Generate CA normally first.
	exitCode := caGenerate()
	if exitCode != 0 {
		t.Fatalf("ca generate: %d", exitCode)
	}

	// Corrupt the key file with valid PEM wrapping but bad content.
	badKey := "-----BEGIN EC PRIVATE KEY-----\nYmFk\n-----END EC PRIVATE KEY-----\n"
	os.WriteFile(filepath.Join(dir, "ca.key"), []byte(badKey), 0o600)

	exitCode = hostIssueCert("bad-key-host")
	if exitCode != 1 {
		t.Errorf("expected exit 1 with corrupt key, got %d", exitCode)
	}
}

// TestCAGenerateMkdirError tests when the data directory can't be created.
func TestCAGenerateMkdirError(t *testing.T) {
	// Point to a path under /proc which can't be created.
	os.Setenv("CONVOCATE_DATA_DIR", "/proc/nonexistent/ca-data")
	t.Cleanup(func() { os.Unsetenv("CONVOCATE_DATA_DIR") })

	exitCode := caGenerate()
	if exitCode != 1 {
		t.Errorf("expected exit 1, got %d", exitCode)
	}
}

// TestOpenbaoInitMkdirError tests when data dir can't be created.
func TestOpenbaoInitMkdirError(t *testing.T) {
	os.Setenv("CONVOCATE_DATA_DIR", "/proc/nonexistent/bao-data")
	t.Cleanup(func() { os.Unsetenv("CONVOCATE_DATA_DIR") })

	exitCode := openbaoInit()
	if exitCode != 1 {
		t.Errorf("expected exit 1, got %d", exitCode)
	}
}
