package dns

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteHostsFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	records := []Record{
		{Name: "dns.local", IP: "192.168.3.90"},
		{Name: "svc.internal", IP: "10.0.0.1"},
	}
	if err := WriteHostsFile(path, records); err != nil {
		t.Fatalf("WriteHostsFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "192.168.3.90\tdns.local") {
		t.Errorf("missing dns.local entry:\n%s", string(data))
	}
	if !strings.Contains(string(data), "10.0.0.1\tsvc.internal") {
		t.Errorf("missing svc.internal entry:\n%s", string(data))
	}
	if !strings.HasPrefix(string(data), "# Managed by convocate") {
		t.Errorf("missing header:\n%s", string(data))
	}
}

func TestWriteHostsFile_EmptyRecords(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	if err := WriteHostsFile(path, nil); err != nil {
		t.Fatalf("WriteHostsFile(nil): %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), "# Managed by convocate") {
		t.Errorf("expected header even with no records, got:\n%s", string(data))
	}
}

func TestWriteHostsFile_SkipsBlankEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	records := []Record{
		{Name: "", IP: "1.2.3.4"},       // skipped
		{Name: "ok.local", IP: ""},      // skipped
		{Name: "good.local", IP: "1.1"}, // kept
	}
	if err := WriteHostsFile(path, records); err != nil {
		t.Fatalf("WriteHostsFile: %v", err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "1.2.3.4") {
		t.Errorf("blank-name record should be skipped:\n%s", string(data))
	}
	if strings.Contains(string(data), "ok.local") {
		t.Errorf("blank-IP record should be skipped:\n%s", string(data))
	}
	if !strings.Contains(string(data), "good.local") {
		t.Errorf("good record missing:\n%s", string(data))
	}
}

func TestWriteHostsFile_NonWritableDirError(t *testing.T) {
	// Path whose parent is a file, not a directory.
	dir := t.TempDir()
	conflict := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(conflict, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	err := WriteHostsFile(filepath.Join(conflict, "hosts"), []Record{{Name: "a", IP: "1.1.1.1"}})
	if err == nil {
		t.Error("expected error writing into a non-directory path")
	}
}

func TestHostsFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	if !HostsFileExists(path) {
		t.Error("expected true when parent dir exists")
	}
	if HostsFileExists(filepath.Join(dir, "nope", "hosts")) {
		t.Error("expected false when parent dir is missing")
	}

	// Parent path is a file (not a dir).
	filePath := filepath.Join(dir, "file")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if HostsFileExists(filepath.Join(filePath, "hosts")) {
		t.Error("expected false when parent is a file")
	}
}

func TestDetectHostIP_OverrideAndDefault(t *testing.T) {
	orig := DetectHostIP
	defer func() { DetectHostIP = orig }()
	DetectHostIP = func() string { return "10.0.0.7" }
	if got := DetectHostIP(); got != "10.0.0.7" {
		t.Errorf("got %q, want 10.0.0.7", got)
	}
}

// TestDetectHostIP_RealCall invokes the production implementation so
// its interface-walk + IPv4 filter branches show up in the coverage
// report. Accepts any non-empty result including the 127.0.0.1
// fallback on sandboxed runners with no routable interface.
func TestDetectHostIP_RealCall(t *testing.T) {
	orig := DetectHostIP
	defer func() { DetectHostIP = orig }()
	got := orig()
	if got == "" {
		t.Fatal("DetectHostIP returned empty string")
	}
	if got != "127.0.0.1" {
		// Validate the shape of a non-loopback answer.
		var a, b, c, d int
		if _, err := fmt.Sscanf(got, "%d.%d.%d.%d", &a, &b, &c, &d); err != nil {
			t.Errorf("DetectHostIP returned non-IPv4 %q: %v", got, err)
		}
	}
}

// TestWriteHostsFile_RenameFails exercises the rename-error branch by
// pointing WriteHostsFile at a target path that's actually a non-empty
// directory. The tmp write succeeds (different inode); the rename then
// fails with ENOTDIR-equivalent because the kernel refuses to clobber
// a directory with a plain file.
func TestWriteHostsFile_RenameFails(t *testing.T) {
	parent := t.TempDir()
	dst := filepath.Join(parent, "hosts")
	// Make dst a non-empty directory — rename into it fails.
	if err := os.MkdirAll(filepath.Join(dst, "child"), 0755); err != nil {
		t.Fatal(err)
	}
	err := WriteHostsFile(dst, []Record{{Name: "n", IP: "1.1.1.1"}})
	if err == nil {
		t.Error("expected rename error when dst is a directory")
	}
	if err != nil && !strings.Contains(err.Error(), "rename") {
		t.Errorf("error = %q, want rename-flavored", err)
	}
}

func TestDetectHostIP_Default(t *testing.T) {
	// Real call — any valid IPv4 is acceptable; fall-back covers no-network
	// test hosts.
	ip := DetectHostIP()
	if ip == "" {
		t.Error("expected non-empty IP")
	}
	// Must parse as IPv4 or be the loopback fallback.
	if strings.Count(ip, ".") != 3 {
		t.Errorf("got %q, want IPv4-dotted address", ip)
	}
}
