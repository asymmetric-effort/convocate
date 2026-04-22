package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupTestDir(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	skelDir := filepath.Join(base, "skel")
	if err := os.MkdirAll(skelDir, 0750); err != nil {
		t.Fatal(err)
	}
	// Create a CLAUDE.md in skel
	if err := os.WriteFile(filepath.Join(skelDir, "CLAUDE.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}
	return base, skelDir
}

func TestCreateWithPort_Specific(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	meta, err := mgr.CreateWithPort("svc", 8080)
	if err != nil {
		t.Fatalf("CreateWithPort failed: %v", err)
	}
	if meta.Port != 8080 {
		t.Errorf("Port = %d, want 8080", meta.Port)
	}

	// Second session using the same port should fail
	_, err = mgr.CreateWithPort("other", 8080)
	if err == nil {
		t.Error("expected error when reusing a port assigned to another session")
	}
}

func TestCreateWithPort_NoPort(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	meta, err := mgr.CreateWithPort("svc", 0)
	if err != nil {
		t.Fatalf("CreateWithPort failed: %v", err)
	}
	if meta.Port != 0 {
		t.Errorf("Port = %d, want 0", meta.Port)
	}
}

func TestCreateWithPort_Auto(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	meta, err := mgr.CreateWithPort("svc", PortAuto)
	if err != nil {
		t.Fatalf("CreateWithPort(PortAuto) failed: %v", err)
	}
	if meta.Port < PortAutoMin {
		t.Errorf("Port = %d, want >= %d", meta.Port, PortAutoMin)
	}

	// Next auto pick should land on the next free port
	meta2, err := mgr.CreateWithPort("svc2", PortAuto)
	if err != nil {
		t.Fatalf("second CreateWithPort(PortAuto) failed: %v", err)
	}
	if meta2.Port == meta.Port {
		t.Errorf("two auto-assigned ports collided: %d", meta.Port)
	}
}

func TestValidateProtocol(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "tcp", false},
		{"tcp", "tcp", false},
		{"TCP", "tcp", false},
		{"udp", "udp", false},
		{"UDP", "udp", false},
		{"  tcp  ", "tcp", false},
		{"sctp", "", true},
		{"icmp", "", true},
	}
	for _, tc := range tests {
		got, err := ValidateProtocol(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ValidateProtocol(%q) = %q, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ValidateProtocol(%q) failed: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ValidateProtocol(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMetadata_EffectiveProtocol(t *testing.T) {
	if got := (Metadata{}).EffectiveProtocol(); got != "tcp" {
		t.Errorf("empty protocol -> %q, want 'tcp'", got)
	}
	if got := (Metadata{Protocol: "udp"}).EffectiveProtocol(); got != "udp" {
		t.Errorf("udp -> %q, want 'udp'", got)
	}
}

func TestCreateWithPortProtocol_UDP(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	meta, err := mgr.CreateWithPortProtocol("dns", 53, "udp")
	if err != nil {
		t.Fatalf("CreateWithPortProtocol failed: %v", err)
	}
	if meta.Port != 53 || meta.Protocol != "udp" {
		t.Errorf("got port=%d proto=%q, want 53/udp", meta.Port, meta.Protocol)
	}

	data, err := os.ReadFile(filepath.Join(base, meta.UUID, "session.json"))
	if err != nil {
		t.Fatalf("read session.json: %v", err)
	}
	if !strings.Contains(string(data), `"protocol": "udp"`) {
		t.Errorf("session.json does not contain protocol:\n%s", string(data))
	}
}

func TestCreateWithPortProtocol_SamePortDifferentProtocolsCoexist(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	if _, err := mgr.CreateWithPortProtocol("dns-tcp", 53, "tcp"); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	// Same port, different protocol — should NOT collide.
	if _, err := mgr.CreateWithPortProtocol("dns-udp", 53, "udp"); err != nil {
		t.Errorf("tcp:53 and udp:53 should coexist, got: %v", err)
	}
}

func TestCreateWithPortProtocol_SameTupleCollides(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	if _, err := mgr.CreateWithPortProtocol("a", 53, "udp"); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	if _, err := mgr.CreateWithPortProtocol("b", 53, "udp"); err == nil {
		t.Error("udp:53 twice should collide")
	}
}

func TestCreateWithPortProtocol_InvalidProtocol(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	_, err := mgr.CreateWithPortProtocol("x", 8080, "sctp")
	if err == nil {
		t.Error("expected error for invalid protocol")
	}
}

func TestUpdate_ChangeProtocolOnly(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.CreateWithPortProtocol("dns", 53, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := mgr.Update(meta.UUID, "dns", 53, "udp")
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Protocol != "udp" {
		t.Errorf("protocol = %q, want 'udp'", updated.Protocol)
	}
}

func TestUpdate_ProtocolCollisionWithOther(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	if _, err := mgr.CreateWithPortProtocol("a", 53, "udp"); err != nil {
		t.Fatal(err)
	}
	mb, err := mgr.CreateWithPortProtocol("b", 53, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	// Changing b to udp should collide with a.
	_, err = mgr.Update(mb.UUID, "b", 53, "udp")
	if err == nil {
		t.Error("expected collision when switching to an in-use (port, protocol) tuple")
	}
}

// --- DNS name validation ---

func TestValidateDNSName(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"dns.local", "dns.local", false},
		{"DNS.Local", "dns.local", false},
		{"svc-1", "svc-1", false},
		{"a.b.c.d", "a.b.c.d", false},
		{"-bad", "", true},
		{"bad-", "", true},
		{"bad..dot", "", true},
		{".leading", "", true},
		{"trailing.", "", true},
		{"spaces here", "", true},
		{"under_score", "", true},
		{strings.Repeat("a", 64) + ".local", "", true},
		{strings.Repeat("a.", 130) + "x", "", true},
	}
	for _, tc := range tests {
		got, err := ValidateDNSName(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ValidateDNSName(%q) = %q, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ValidateDNSName(%q) failed: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ValidateDNSName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- DNS via options ---

func TestCreateWithOptions_PersistsDNSName(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	meta, err := mgr.CreateWithOptions("svc", CreateOptions{
		Port:     8080,
		Protocol: "tcp",
		DNSName:  "svc.claude.local",
	})
	if err != nil {
		t.Fatalf("CreateWithOptions: %v", err)
	}
	if meta.DNSName != "svc.claude.local" {
		t.Errorf("DNSName = %q, want 'svc.claude.local'", meta.DNSName)
	}
	data, err := os.ReadFile(filepath.Join(base, meta.UUID, "session.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"dns_name": "svc.claude.local"`) {
		t.Errorf("dns_name not persisted:\n%s", string(data))
	}
}

func TestCreateWithOptions_DNSCollision(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	if _, err := mgr.CreateWithOptions("a", CreateOptions{DNSName: "dup.local"}); err != nil {
		t.Fatal(err)
	}
	_, err := mgr.CreateWithOptions("b", CreateOptions{DNSName: "dup.local"})
	if err == nil {
		t.Error("expected DNS name collision error")
	}
}

func TestCreateWithOptions_InvalidDNS(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	_, err := mgr.CreateWithOptions("x", CreateOptions{DNSName: "bad name"})
	if err == nil {
		t.Error("expected validation error for bad DNS name")
	}
}

func TestUpdateWithOptions_ChangeDNS(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.CreateWithOptions("x", CreateOptions{DNSName: "before.local"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := mgr.UpdateWithOptions(meta.UUID, UpdateOptions{
		Name:    "x",
		DNSName: "after.local",
	})
	if err != nil {
		t.Fatalf("UpdateWithOptions: %v", err)
	}
	if updated.DNSName != "after.local" {
		t.Errorf("DNSName = %q, want 'after.local'", updated.DNSName)
	}
}

func TestUpdateWithOptions_ClearDNS(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.CreateWithOptions("x", CreateOptions{DNSName: "gone.local"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := mgr.UpdateWithOptions(meta.UUID, UpdateOptions{Name: "x"})
	if err != nil {
		t.Fatalf("UpdateWithOptions: %v", err)
	}
	if updated.DNSName != "" {
		t.Errorf("DNSName = %q, want empty after clear", updated.DNSName)
	}
}

func TestUpdateWithOptions_DNSCollision(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	if _, err := mgr.CreateWithOptions("a", CreateOptions{DNSName: "a.local"}); err != nil {
		t.Fatal(err)
	}
	meta, err := mgr.CreateWithOptions("b", CreateOptions{DNSName: "b.local"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = mgr.UpdateWithOptions(meta.UUID, UpdateOptions{
		Name:    "b",
		DNSName: "a.local",
	})
	if err == nil {
		t.Error("expected DNS collision on Update")
	}
}

func TestUpdateWithOptions_KeepSameDNS(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.CreateWithOptions("x", CreateOptions{DNSName: "keep.local"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := mgr.UpdateWithOptions(meta.UUID, UpdateOptions{
		Name:    "x",
		DNSName: "keep.local",
	})
	if err != nil {
		t.Fatalf("keeping same DNS should not error: %v", err)
	}
	if updated.DNSName != "keep.local" {
		t.Errorf("DNSName = %q, want 'keep.local'", updated.DNSName)
	}
}

func TestCreateWithPort_PersistsToSessionJSON(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	meta, err := mgr.CreateWithPort("svc", 8080)
	if err != nil {
		t.Fatalf("CreateWithPort failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(base, meta.UUID, "session.json"))
	if err != nil {
		t.Fatalf("read session.json: %v", err)
	}
	if !strings.Contains(string(data), `"port": 8080`) {
		t.Errorf("session.json does not contain port 8080:\n%s", string(data))
	}

	// Re-read via Get and confirm the round-trip preserves the port.
	got, err := mgr.Get(meta.UUID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Port != 8080 {
		t.Errorf("Get returned Port = %d, want 8080", got.Port)
	}
}

func TestFindAvailablePort_SkipsUsed(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	_, err := mgr.CreateWithPort("a", 1001)
	if err != nil {
		t.Fatalf("CreateWithPort failed: %v", err)
	}

	p, err := mgr.FindAvailablePort(1001)
	if err != nil {
		t.Fatalf("FindAvailablePort failed: %v", err)
	}
	if p != 1002 {
		t.Errorf("FindAvailablePort = %d, want 1002 (1001 is taken)", p)
	}
}

func TestNewManager(t *testing.T) {
	mgr := NewManager("/tmp/test", "/tmp/skel")
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.basePath != "/tmp/test" {
		t.Errorf("basePath = %q, want %q", mgr.basePath, "/tmp/test")
	}
	if mgr.skelPath != "/tmp/skel" {
		t.Errorf("skelPath = %q, want %q", mgr.skelPath, "/tmp/skel")
	}
}

func TestCreate_Success(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	meta, err := mgr.Create("test-session")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if meta.Name != "test-session" {
		t.Errorf("Name = %q, want %q", meta.Name, "test-session")
	}
	if meta.UUID == "" {
		t.Error("UUID is empty")
	}
	if meta.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
	if meta.LastAccessed.IsZero() {
		t.Error("LastAccessed is zero")
	}

	// Verify session directory exists
	sessionDir := filepath.Join(base, meta.UUID)
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		t.Errorf("session directory not created: %s", sessionDir)
	}

	// Verify CLAUDE.md was copied from skel
	claudeMD := filepath.Join(sessionDir, "CLAUDE.md")
	if _, err := os.Stat(claudeMD); os.IsNotExist(err) {
		t.Error("CLAUDE.md not copied from skel")
	}

	// Verify metadata file exists
	metaFile := filepath.Join(sessionDir, "session.json")
	if _, err := os.Stat(metaFile); os.IsNotExist(err) {
		t.Error("session.json not created")
	}
}

func TestCreateWithUUID(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "12345678-1234-1234-1234-123456789abc"
	meta, err := mgr.CreateWithUUID(testUUID, "named-session", 0)
	if err != nil {
		t.Fatalf("CreateWithUUID failed: %v", err)
	}
	if meta.UUID != testUUID {
		t.Errorf("UUID = %q, want %q", meta.UUID, testUUID)
	}
	if meta.Name != "named-session" {
		t.Errorf("Name = %q, want %q", meta.Name, "named-session")
	}
}

func TestList_Empty(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "")

	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestList_NonExistentDir(t *testing.T) {
	mgr := NewManager("/nonexistent/path/12345", "")

	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List should not error for nonexistent dir: %v", err)
	}
	if sessions != nil {
		t.Errorf("expected nil sessions, got %v", sessions)
	}
}

func TestList_MultipleSessions(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	_, err := mgr.CreateWithUUID("aaaaaaaa-1111-1111-1111-111111111111", "first", 0)
	if err != nil {
		t.Fatal(err)
	}

	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	_, err = mgr.CreateWithUUID("bbbbbbbb-2222-2222-2222-222222222222", "second", 0)
	if err != nil {
		t.Fatal(err)
	}

	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// Should be sorted by last accessed (most recent first)
	if sessions[0].Name != "second" {
		t.Errorf("first session should be 'second' (most recent), got %q", sessions[0].Name)
	}
}

func TestList_IgnoresNonUUIDDirs(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	// Create a non-UUID directory
	if err := os.MkdirAll(filepath.Join(base, "not-a-uuid"), 0750); err != nil {
		t.Fatal(err)
	}

	_, err := mgr.CreateWithUUID("cccccccc-3333-3333-3333-333333333333", "valid", 0)
	if err != nil {
		t.Fatal(err)
	}

	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session (ignoring non-UUID dirs), got %d", len(sessions))
	}
}

func TestGet_Success(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "dddddddd-4444-4444-4444-444444444444"
	_, err := mgr.CreateWithUUID(testUUID, "get-test", 0)
	if err != nil {
		t.Fatal(err)
	}

	meta, err := mgr.Get(testUUID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if meta.Name != "get-test" {
		t.Errorf("Name = %q, want %q", meta.Name, "get-test")
	}
}

func TestGet_NotFound(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "")

	_, err := mgr.Get("nonexistent-uuid")
	if err == nil {
		t.Error("expected error for nonexistent session, got nil")
	}
}

func TestDelete_Success(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "eeeeeeee-5555-5555-5555-555555555555"
	_, err := mgr.CreateWithUUID(testUUID, "delete-test", 0)
	if err != nil {
		t.Fatal(err)
	}

	if err := mgr.Delete(testUUID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	sessionDir := filepath.Join(base, testUUID)
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Error("session directory still exists after delete")
	}
}

func TestDelete_NotFound(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "")

	err := mgr.Delete("nonexistent-uuid")
	if err == nil {
		t.Error("expected error for nonexistent session, got nil")
	}
}

func TestDelete_Locked(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "ffffffff-6666-6666-6666-666666666666"
	_, err := mgr.CreateWithUUID(testUUID, "locked-test", 0)
	if err != nil {
		t.Fatal(err)
	}

	// Create a lock file
	lockPath := filepath.Join(base, testUUID+".lock")
	if err := os.WriteFile(lockPath, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}

	err = mgr.Delete(testUUID)
	if err == nil {
		t.Error("expected error when deleting locked session, got nil")
	}
}

func TestTouch(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "11111111-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	_, err := mgr.CreateWithUUID(testUUID, "touch-test", 0)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(10 * time.Millisecond)

	if err := mgr.Touch(testUUID); err != nil {
		t.Fatalf("Touch failed: %v", err)
	}

	meta, err := mgr.Get(testUUID)
	if err != nil {
		t.Fatal(err)
	}

	if !meta.LastAccessed.After(meta.CreatedAt) {
		t.Error("LastAccessed should be after CreatedAt after Touch")
	}
}

func TestLock_Success(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "22222222-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	_, err := mgr.CreateWithUUID(testUUID, "lock-test", 0)
	if err != nil {
		t.Fatal(err)
	}

	unlock, err := mgr.Lock(testUUID)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	// Verify lock file exists
	lockPath := filepath.Join(base, testUUID+".lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file not created")
	}

	// Unlock
	unlock()

	// Verify lock file removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file not removed after unlock")
	}
}

func TestLock_AlreadyLocked(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "33333333-cccc-cccc-cccc-cccccccccccc"
	_, err := mgr.CreateWithUUID(testUUID, "double-lock-test", 0)
	if err != nil {
		t.Fatal(err)
	}

	unlock, err := mgr.Lock(testUUID)
	if err != nil {
		t.Fatalf("first Lock failed: %v", err)
	}
	defer unlock()

	_, err = mgr.Lock(testUUID)
	if err == nil {
		t.Error("expected error for double lock, got nil")
	}
}

func TestIsLocked_NotLocked(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "44444444-dddd-dddd-dddd-dddddddddddd"
	_, err := mgr.CreateWithUUID(testUUID, "islocked-test", 0)
	if err != nil {
		t.Fatal(err)
	}

	if mgr.IsLocked(testUUID) {
		t.Error("session should not be locked")
	}
}

func TestIsLocked_Locked(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "55555555-eeee-eeee-eeee-eeeeeeeeeeee"
	_, err := mgr.CreateWithUUID(testUUID, "islocked-locked-test", 0)
	if err != nil {
		t.Fatal(err)
	}

	unlock, err := mgr.Lock(testUUID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	if !mgr.IsLocked(testUUID) {
		t.Error("session should be locked")
	}
}

func TestIsLocked_StaleLock(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "66666666-ffff-ffff-ffff-ffffffffffff"
	_, err := mgr.CreateWithUUID(testUUID, "stale-lock-test", 0)
	if err != nil {
		t.Fatal(err)
	}

	// Create a lock file with invalid PID
	lockPath := filepath.Join(base, testUUID+".lock")
	if err := os.WriteFile(lockPath, []byte("99999999"), 0600); err != nil {
		t.Fatal(err)
	}

	// Lock should be considered stale (PID doesn't exist)
	if mgr.IsLocked(testUUID) {
		t.Error("stale lock should be cleaned up")
	}

	// Lock file should have been removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("stale lock file should have been removed")
	}
}

func TestSessionDir(t *testing.T) {
	mgr := NewManager("/base/path", "/skel/path")
	expected := "/base/path/test-uuid"
	got := mgr.SessionDir("test-uuid")
	if got != expected {
		t.Errorf("SessionDir = %q, want %q", got, expected)
	}
}

func TestValidateName_Valid(t *testing.T) {
	validNames := []string{
		"test",
		"my-session",
		"Session 1",
		"bug_fix_123",
		"a",
		"project.v2",
		"A-Z.0-9_test",
	}

	for _, name := range validNames {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) returned unexpected error: %v", name, err)
		}
	}
}

func TestValidateName_Empty(t *testing.T) {
	err := ValidateName("")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestValidateName_Whitespace(t *testing.T) {
	err := ValidateName("   ")
	if err == nil {
		t.Error("expected error for whitespace-only name")
	}
}

func TestValidateName_TooLong(t *testing.T) {
	longName := ""
	for i := 0; i < 65; i++ {
		longName += "a"
	}
	err := ValidateName(longName)
	if err == nil {
		t.Error("expected error for name exceeding 64 characters")
	}
}

func TestValidateName_ControlChars(t *testing.T) {
	err := ValidateName("test\x00name")
	if err == nil {
		t.Error("expected error for name with control characters")
	}
}

func TestValidateName_Punctuation(t *testing.T) {
	invalidNames := []string{
		"test!name",
		"hello@world",
		"foo#bar",
		"test/path",
		"name;drop",
		"test&name",
		"a,b",
		"test(1)",
	}

	for _, name := range invalidNames {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) should reject punctuation", name)
		}
	}
}

func TestIsValidNameRune(t *testing.T) {
	valid := []rune{'a', 'z', 'A', 'Z', '0', '9', ' ', '_', '-', '.'}
	for _, r := range valid {
		if !isValidNameRune(r) {
			t.Errorf("isValidNameRune(%q) = false, want true", r)
		}
	}

	invalid := []rune{'!', '@', '#', '$', '/', '\\', ';', ',', '(', ')', '\t', '\n', '\x00'}
	for _, r := range invalid {
		if isValidNameRune(r) {
			t.Errorf("isValidNameRune(%q) = true, want false", r)
		}
	}
}

func TestCreate_NoSkel(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "/nonexistent/skel")

	meta, err := mgr.Create("no-skel")
	if err != nil {
		t.Fatalf("Create should succeed without skel dir: %v", err)
	}

	sessionDir := filepath.Join(base, meta.UUID)
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		t.Error("session directory not created")
	}
}

func TestCopySkel_WithSymlink(t *testing.T) {
	base := t.TempDir()
	skelDir := filepath.Join(base, "skel")
	if err := os.MkdirAll(skelDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Create a regular file
	if err := os.WriteFile(filepath.Join(skelDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink
	targetFile := filepath.Join(base, "target.txt")
	if err := os.WriteFile(targetFile, []byte("target"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetFile, filepath.Join(skelDir, "link.txt")); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory with a file
	subDir := filepath.Join(skelDir, "subdir")
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "sub.txt"), []byte("sub"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(base, skelDir)
	destDir := filepath.Join(base, "dest")

	if err := mgr.copySkel(destDir); err != nil {
		t.Fatalf("copySkel failed: %v", err)
	}

	// Verify file copied
	if _, err := os.Stat(filepath.Join(destDir, "file.txt")); os.IsNotExist(err) {
		t.Error("file.txt not copied")
	}

	// Verify symlink copied
	link, err := os.Readlink(filepath.Join(destDir, "link.txt"))
	if err != nil {
		t.Errorf("link.txt not copied as symlink: %v", err)
	}
	if link != targetFile {
		t.Errorf("symlink target = %q, want %q", link, targetFile)
	}

	// Verify subdirectory and file copied
	if _, err := os.Stat(filepath.Join(destDir, "subdir", "sub.txt")); os.IsNotExist(err) {
		t.Error("subdir/sub.txt not copied")
	}
}

func TestSetupClaudeSymlinks(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "")
	sessionDir := filepath.Join(base, "test-session")
	if err := os.MkdirAll(sessionDir, 0750); err != nil {
		t.Fatal(err)
	}

	if err := mgr.setupClaudeSymlinks(sessionDir); err != nil {
		t.Fatalf("setupClaudeSymlinks failed: %v", err)
	}

	claudeDir := filepath.Join(sessionDir, ".claude")
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		t.Error(".claude directory not created")
	}

	// Check symlinks exist
	for _, name := range []string{"settings.json", "settings.local.json", ".credentials.json", "plugins"} {
		link := filepath.Join(claudeDir, name)
		target, err := os.Readlink(link)
		if err != nil {
			t.Errorf("symlink %s not created: %v", name, err)
			continue
		}
		expectedPrefix := "/home/claude/.claude-shared/"
		if !strings.HasPrefix(target, expectedPrefix) {
			t.Errorf("symlink %s target = %q, expected prefix %q", name, target, expectedPrefix)
		}
	}
}

func TestSetupClaudeSymlinks_Idempotent(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "")
	sessionDir := filepath.Join(base, "test-session")
	if err := os.MkdirAll(sessionDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Run twice - second run should not fail
	if err := mgr.setupClaudeSymlinks(sessionDir); err != nil {
		t.Fatal(err)
	}
	if err := mgr.setupClaudeSymlinks(sessionDir); err != nil {
		t.Fatalf("second setupClaudeSymlinks should be idempotent: %v", err)
	}
}

func TestReadMetadata_InvalidJSON(t *testing.T) {
	base := t.TempDir()
	sessionDir := filepath.Join(base, "bad-session")
	if err := os.MkdirAll(sessionDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "session.json"), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(base, "")
	_, err := mgr.Get("bad-session")
	if err == nil {
		t.Error("expected error for invalid JSON metadata")
	}
}

func TestList_SkipsBadMetadata(t *testing.T) {
	base := t.TempDir()

	// Create a UUID-named dir with invalid metadata
	badUUID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	badDir := filepath.Join(base, badUUID)
	if err := os.MkdirAll(badDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "session.json"), []byte("invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(base, "")
	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	// Should skip the bad entry
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (bad metadata skipped), got %d", len(sessions))
	}
}

func TestIsLocked_InvalidLockContent(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	testUUID := "77777777-aaaa-bbbb-cccc-dddddddddddd"
	_, err := mgr.CreateWithUUID(testUUID, "bad-lock-test", 0)
	if err != nil {
		t.Fatal(err)
	}

	// Create lock file with non-numeric content
	lockPath := filepath.Join(base, testUUID+".lock")
	if err := os.WriteFile(lockPath, []byte("not-a-pid"), 0600); err != nil {
		t.Fatal(err)
	}

	if mgr.IsLocked(testUUID) {
		t.Error("invalid lock content should result in not locked (cleaned up)")
	}
}

func TestTouch_NonexistentSession(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "")

	err := mgr.Touch("nonexistent")
	if err == nil {
		t.Error("expected error when touching nonexistent session")
	}
}

// --- Clone ---

func TestClone_Success(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)

	src, err := mgr.Create("original")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Write a marker file into the source session.
	markerPath := filepath.Join(base, src.UUID, "marker.txt")
	if err := os.WriteFile(markerPath, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	clone, err := mgr.Clone(src.UUID, "copy")
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}
	if clone.Name != "copy" {
		t.Errorf("Name = %q, want 'copy'", clone.Name)
	}
	if clone.UUID == src.UUID {
		t.Error("Clone should generate a new UUID")
	}

	// Marker file should be copied.
	data, err := os.ReadFile(filepath.Join(base, clone.UUID, "marker.txt"))
	if err != nil {
		t.Fatalf("marker not copied: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("marker content = %q, want 'hello'", data)
	}
}

func TestClone_Nonexistent(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "")
	_, err := mgr.Clone("nope", "new")
	if err == nil {
		t.Error("expected error cloning nonexistent session")
	}
}

// --- OverrideLock ---

func TestOverrideLock_NotLocked(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.Create("s")
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.OverrideLock(meta.UUID); err == nil {
		t.Error("expected error when overriding an unlocked session")
	}
}

func TestOverrideLock_StaleByDeadPID(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.Create("s")
	if err != nil {
		t.Fatal(err)
	}
	// Write a lock with a PID almost certainly not running.
	lockPath := filepath.Join(base, meta.UUID+".lock")
	if err := os.WriteFile(lockPath, []byte("999999"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.OverrideLock(meta.UUID); err != nil {
		t.Errorf("OverrideLock failed on stale lock: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("stale lock file was not removed")
	}
}

func TestOverrideLock_InvalidPIDContent(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.Create("s")
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(base, meta.UUID+".lock")
	if err := os.WriteFile(lockPath, []byte("not-a-pid"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.OverrideLock(meta.UUID); err != nil {
		t.Errorf("OverrideLock failed on invalid PID: %v", err)
	}
}

func TestOverrideLock_ActivelyHeld(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.Create("s")
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(base, meta.UUID+".lock")
	pid := os.Getpid()
	if err := os.WriteFile(lockPath, []byte(fmt.Sprintf("%d", pid)), 0600); err != nil {
		t.Fatal(err)
	}
	err = mgr.OverrideLock(meta.UUID)
	if err == nil {
		t.Error("expected error overriding an actively held lock")
	}
}

// --- IsLocked: stale (>24h) ---

func TestIsLocked_StaleFile(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.Create("s")
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(base, meta.UUID+".lock")
	if err := os.WriteFile(lockPath, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	// Age the mtime past the 24h staleness window.
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}
	if mgr.IsLocked(meta.UUID) {
		t.Error("stale lock file should be considered not locked")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("stale lock file should have been removed")
	}
}

func TestIsLocked_LivePID(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.Create("s")
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(base, meta.UUID+".lock")
	// Our own PID is alive → locked.
	if err := os.WriteFile(lockPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	if !mgr.IsLocked(meta.UUID) {
		t.Error("expected IsLocked=true for live PID")
	}
}

// --- Delete: locked session cannot be deleted ---

func TestDelete_LockedSessionRejected(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.Create("s")
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(base, meta.UUID+".lock")
	if err := os.WriteFile(lockPath, []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Delete(meta.UUID); err == nil {
		t.Error("expected error deleting a locked session")
	}
}

// --- resolvePort: invalid negative port that isn't PortAuto ---

func TestResolvePort_InvalidNegative(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	_, err := mgr.CreateWithPort("bad", -42)
	if err == nil {
		t.Error("expected error for invalid negative port")
	}
}

// --- Update ---

func TestUpdate_RenameOnly(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.CreateWithPort("before", 8080)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := mgr.Update(meta.UUID, "after", 8080, "tcp")
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name != "after" || updated.Port != 8080 {
		t.Errorf("updated = %+v, want name=after port=8080", updated)
	}
	// Round-trip read from disk.
	got, err := mgr.Get(meta.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "after" {
		t.Errorf("persisted name = %q, want 'after'", got.Name)
	}
}

func TestUpdate_ChangePortToAuto(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.CreateWithPort("s", 0)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := mgr.Update(meta.UUID, "s", PortAuto, "tcp")
	if err != nil {
		t.Fatalf("Update(PortAuto) failed: %v", err)
	}
	if updated.Port < PortAutoMin {
		t.Errorf("auto port = %d, want >= %d", updated.Port, PortAutoMin)
	}
}

func TestUpdate_ChangePortToZero(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.CreateWithPort("s", 8080)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := mgr.Update(meta.UUID, "s", 0, "tcp")
	if err != nil {
		t.Fatalf("Update port->0 failed: %v", err)
	}
	if updated.Port != 0 {
		t.Errorf("updated.Port = %d, want 0", updated.Port)
	}
}

func TestUpdate_ChangePortToSpecific(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.CreateWithPort("s", 8080)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := mgr.Update(meta.UUID, "s", 9090, "tcp")
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Port != 9090 {
		t.Errorf("updated.Port = %d, want 9090", updated.Port)
	}
}

func TestUpdate_PortCollisionWithOther(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	_, err := mgr.CreateWithPort("a", 8080)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := mgr.CreateWithPort("b", 8081)
	if err != nil {
		t.Fatal(err)
	}
	_, err = mgr.Update(meta.UUID, "b", 8080, "tcp")
	if err == nil {
		t.Error("expected collision error when updating to a port held by another session")
	}
}

func TestUpdate_KeepSamePort(t *testing.T) {
	// Saving without changing the port should not complain about self-collision.
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.CreateWithPort("s", 8080)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := mgr.Update(meta.UUID, "s-renamed", 8080, "tcp")
	if err != nil {
		t.Fatalf("Update with unchanged port failed: %v", err)
	}
	if updated.Port != 8080 {
		t.Errorf("port should remain 8080, got %d", updated.Port)
	}
}

func TestUpdate_InvalidName(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.Create("s")
	if err != nil {
		t.Fatal(err)
	}
	_, err = mgr.Update(meta.UUID, "", 0, "tcp")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestUpdate_InvalidPort(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	meta, err := mgr.Create("s")
	if err != nil {
		t.Fatal(err)
	}
	_, err = mgr.Update(meta.UUID, "s", -42, "tcp")
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestUpdate_Nonexistent(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base, "")
	_, err := mgr.Update("missing", "name", 0, "tcp")
	if err == nil {
		t.Error("expected error updating nonexistent session")
	}
}

func TestIsPortUsedByOther_NoMatch(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	_, err := mgr.CreateWithPort("x", 8080)
	if err != nil {
		t.Fatal(err)
	}
	if mgr.isPortUsedByOther("some-id", 9999, "tcp") {
		t.Error("expected false when no session uses port 9999")
	}
}

// --- Symlinks: re-create is idempotent ---

func TestSetupClaudeSymlinks_ExistingSymlinks(t *testing.T) {
	base, skelDir := setupTestDir(t)
	mgr := NewManager(base, skelDir)
	// Creating the same session twice with different UUIDs should both succeed;
	// setupClaudeSymlinks is idempotent against existing symlinks.
	_, err := mgr.Create("a")
	if err != nil {
		t.Fatal(err)
	}
	meta, err := mgr.Create("b")
	if err != nil {
		t.Fatal(err)
	}
	// Now call setupClaudeSymlinks again on the same directory to exercise the
	// "skip existing" branch.
	if err := mgr.setupClaudeSymlinks(filepath.Join(base, meta.UUID)); err != nil {
		t.Errorf("second setupClaudeSymlinks failed: %v", err)
	}
}
