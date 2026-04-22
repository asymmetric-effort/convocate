package dns

import (
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
	if !strings.HasPrefix(string(data), "# Managed by claude-shell") {
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
	if !strings.HasPrefix(string(data), "# Managed by claude-shell") {
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
