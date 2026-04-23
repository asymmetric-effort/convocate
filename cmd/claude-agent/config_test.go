package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestGenerateAgentID_FormatAndLength(t *testing.T) {
	id, err := generateAgentID()
	if err != nil {
		t.Fatalf("generateAgentID: %v", err)
	}
	if len(id) != agentIDLength {
		t.Errorf("len = %d, want %d", len(id), agentIDLength)
	}
	// All characters must come from the defined alphabet — no 0/1/o/l/i.
	for i, r := range id {
		if !strings.ContainsRune(agentIDAlphabet, r) {
			t.Errorf("char %d = %q outside alphabet", i, r)
		}
	}
	// Lowercase alphanumeric invariant.
	if matched, _ := regexp.MatchString(`^[a-z0-9]+$`, id); !matched {
		t.Errorf("id %q is not lowercase alphanumeric", id)
	}
}

func TestGenerateAgentID_Uniqueness(t *testing.T) {
	// Not a cryptographic test — just a sanity check that we're not
	// deterministically emitting the same ID every call.
	seen := map[string]bool{}
	for i := 0; i < 32; i++ {
		id, err := generateAgentID()
		if err != nil {
			t.Fatal(err)
		}
		if seen[id] {
			t.Fatalf("duplicate ID after %d iterations: %s", i, id)
		}
		seen[id] = true
	}
}

func TestLoadOrCreateAgentID_CreatesThenReuses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-id")
	first, err := loadOrCreateAgentID(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != agentIDLength {
		t.Errorf("unexpected len %d", len(first))
	}
	// Second call should re-read the same file.
	second, err := loadOrCreateAgentID(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("id not stable: %q then %q", first, second)
	}
	// File exists with expected mode.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", info.Mode().Perm())
	}
}

func TestLoadOrCreateAgentID_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-id")
	if err := os.WriteFile(path, []byte("  fixedabc012 \n"), 0644); err != nil {
		t.Fatal(err)
	}
	id, err := loadOrCreateAgentID(path)
	if err != nil {
		t.Fatal(err)
	}
	if id != "fixedabc012" {
		t.Errorf("id = %q, want 'fixedabc012'", id)
	}
}

func TestLoadOrCreateAgentID_EmptyFileRegenerates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-id")
	if err := os.WriteFile(path, []byte("   \n"), 0644); err != nil {
		t.Fatal(err)
	}
	id, err := loadOrCreateAgentID(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != agentIDLength {
		t.Errorf("expected regenerated id of length %d, got %q", agentIDLength, id)
	}
}

func TestLoadOrCreateAgentID_UnwritableDir(t *testing.T) {
	dir := t.TempDir()
	// Write to a path whose parent is a file — triggers the MkdirAll error.
	bad := filepath.Join(dir, "notadir")
	if err := os.WriteFile(bad, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadOrCreateAgentID(filepath.Join(bad, "nested", "agent-id"))
	if err == nil {
		t.Error("expected error when parent path is not a directory")
	}
}
