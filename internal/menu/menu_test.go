package menu

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/asymmetric-effort/claude-shell/internal/session"
)

// errReader is an io.Reader that always returns an error.
type errReader struct{}

func (e *errReader) Read([]byte) (int, error) {
	return 0, errors.New("read error")
}

// Ensure errReader implements io.Reader.
var _ io.Reader = (*errReader)(nil)

func testSessions() []session.Metadata {
	return []session.Metadata{
		{
			UUID:         "aaaaaaaa-1111-1111-1111-111111111111",
			Name:         "first-session",
			CreatedAt:    time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
			LastAccessed: time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		},
		{
			UUID:         "bbbbbbbb-2222-2222-2222-222222222222",
			Name:         "second-session",
			CreatedAt:    time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC),
			LastAccessed: time.Date(2026, 4, 9, 0, 0, 0, 0, time.UTC),
		},
	}
}

// --- parseSelection tests ---

func TestParseSelection_Create(t *testing.T) {
	for _, input := range []string{"c", "C"} {
		sel, err := parseSelection(input, testSessions())
		if err != nil {
			t.Fatalf("parseSelection(%q) failed: %v", input, err)
		}
		if sel.Action != ActionNewSession {
			t.Errorf("parseSelection(%q).Action = %q, want %q", input, sel.Action, ActionNewSession)
		}
	}
}

func TestParseSelection_Delete(t *testing.T) {
	for _, input := range []string{"d", "D"} {
		sel, err := parseSelection(input, testSessions())
		if err != nil {
			t.Fatalf("parseSelection(%q) failed: %v", input, err)
		}
		if sel.Action != ActionDeleteSession {
			t.Errorf("parseSelection(%q).Action = %q, want %q", input, sel.Action, ActionDeleteSession)
		}
	}
}

func TestParseSelection_Reload(t *testing.T) {
	for _, input := range []string{"r", "R"} {
		sel, err := parseSelection(input, testSessions())
		if err != nil {
			t.Fatalf("parseSelection(%q) failed: %v", input, err)
		}
		if sel.Action != ActionReload {
			t.Errorf("parseSelection(%q).Action = %q, want %q", input, sel.Action, ActionReload)
		}
	}
}

func TestParseSelection_Quit(t *testing.T) {
	for _, input := range []string{"q", "Q"} {
		sel, err := parseSelection(input, testSessions())
		if err != nil {
			t.Fatalf("parseSelection(%q) failed: %v", input, err)
		}
		if sel.Action != ActionQuit {
			t.Errorf("parseSelection(%q).Action = %q, want %q", input, sel.Action, ActionQuit)
		}
	}
}

func TestParseSelection_SessionByNumber(t *testing.T) {
	sessions := testSessions()
	sel, err := parseSelection("1", sessions)
	if err != nil {
		t.Fatalf("parseSelection(1) failed: %v", err)
	}
	if sel.SessionID != sessions[0].UUID {
		t.Errorf("SessionID = %q, want %q", sel.SessionID, sessions[0].UUID)
	}

	sel, err = parseSelection("2", sessions)
	if err != nil {
		t.Fatalf("parseSelection(2) failed: %v", err)
	}
	if sel.SessionID != sessions[1].UUID {
		t.Errorf("SessionID = %q, want %q", sel.SessionID, sessions[1].UUID)
	}
}

func TestParseSelection_InvalidNumber(t *testing.T) {
	_, err := parseSelection("99", testSessions())
	if err == nil {
		t.Error("expected error for out-of-range selection")
	}
}

func TestParseSelection_InvalidText(t *testing.T) {
	_, err := parseSelection("xyz", testSessions())
	if err == nil {
		t.Error("expected error for non-numeric input")
	}
}

// --- Prompt tests ---

func TestPromptSessionName_Success(t *testing.T) {
	reader := strings.NewReader("my-session\n")
	writer := &bytes.Buffer{}

	name, err := PromptSessionName(reader, writer)
	if err != nil {
		t.Fatalf("PromptSessionName failed: %v", err)
	}
	if name != "my-session" {
		t.Errorf("name = %q, want %q", name, "my-session")
	}
}

func TestPromptSessionName_Trimmed(t *testing.T) {
	reader := strings.NewReader("  spaced name  \n")
	writer := &bytes.Buffer{}

	name, err := PromptSessionName(reader, writer)
	if err != nil {
		t.Fatalf("PromptSessionName failed: %v", err)
	}
	if name != "spaced name" {
		t.Errorf("name = %q, want %q", name, "spaced name")
	}
}

func TestPromptSessionName_NoInput(t *testing.T) {
	reader := strings.NewReader("")
	writer := &bytes.Buffer{}

	_, err := PromptSessionName(reader, writer)
	if err == nil {
		t.Error("expected error for no input, got nil")
	}
}

func TestPromptDeleteSession_Success(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("1\n")
	writer := &bytes.Buffer{}

	id, err := PromptDeleteSession(sessions, reader, writer)
	if err != nil {
		t.Fatalf("PromptDeleteSession failed: %v", err)
	}
	if id != sessions[0].UUID {
		t.Errorf("id = %q, want %q", id, sessions[0].UUID)
	}
}

func TestPromptDeleteSession_Cancel(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("c\n")
	writer := &bytes.Buffer{}

	id, err := PromptDeleteSession(sessions, reader, writer)
	if err != nil {
		t.Fatalf("PromptDeleteSession failed: %v", err)
	}
	if id != "" {
		t.Errorf("expected empty id for cancel, got %q", id)
	}
}

func TestPromptDeleteSession_InvalidSelection(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("99\n")
	writer := &bytes.Buffer{}

	_, err := PromptDeleteSession(sessions, reader, writer)
	if err == nil {
		t.Error("expected error for invalid selection, got nil")
	}
}

func TestPromptDeleteSession_NonNumeric(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("xyz\n")
	writer := &bytes.Buffer{}

	_, err := PromptDeleteSession(sessions, reader, writer)
	if err == nil {
		t.Error("expected error for non-numeric input, got nil")
	}
}

func TestPromptDeleteSession_NoInput(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("")
	writer := &bytes.Buffer{}

	_, err := PromptDeleteSession(sessions, reader, writer)
	if err == nil {
		t.Error("expected error for no input, got nil")
	}
}

func TestConfirmDelete_Yes(t *testing.T) {
	reader := strings.NewReader("y\n")
	writer := &bytes.Buffer{}

	confirmed, err := ConfirmDelete("test", "aaaaaaaa-1111-1111-1111-111111111111", reader, writer)
	if err != nil {
		t.Fatalf("ConfirmDelete failed: %v", err)
	}
	if !confirmed {
		t.Error("expected confirmed=true for 'y'")
	}
}

func TestConfirmDelete_YesFull(t *testing.T) {
	reader := strings.NewReader("yes\n")
	writer := &bytes.Buffer{}

	confirmed, err := ConfirmDelete("test", "aaaaaaaa-1111-1111-1111-111111111111", reader, writer)
	if err != nil {
		t.Fatalf("ConfirmDelete failed: %v", err)
	}
	if !confirmed {
		t.Error("expected confirmed=true for 'yes'")
	}
}

func TestConfirmDelete_No(t *testing.T) {
	reader := strings.NewReader("n\n")
	writer := &bytes.Buffer{}

	confirmed, err := ConfirmDelete("test", "aaaaaaaa-1111-1111-1111-111111111111", reader, writer)
	if err != nil {
		t.Fatalf("ConfirmDelete failed: %v", err)
	}
	if confirmed {
		t.Error("expected confirmed=false for 'n'")
	}
}

func TestConfirmDelete_DefaultNo(t *testing.T) {
	reader := strings.NewReader("\n")
	writer := &bytes.Buffer{}

	confirmed, err := ConfirmDelete("test", "aaaaaaaa-1111-1111-1111-111111111111", reader, writer)
	if err != nil {
		t.Fatalf("ConfirmDelete failed: %v", err)
	}
	if confirmed {
		t.Error("expected confirmed=false for empty input (default N)")
	}
}

// --- Scanner error path tests ---

func TestPromptSessionName_ReaderError(t *testing.T) {
	writer := &bytes.Buffer{}
	_, err := PromptSessionName(&errReader{}, writer)
	if err == nil {
		t.Error("expected error from reader error")
	}
	if !strings.Contains(err.Error(), "failed to read input") {
		t.Errorf("error = %q, want 'failed to read input'", err.Error())
	}
}

func TestPromptDeleteSession_ReaderError(t *testing.T) {
	writer := &bytes.Buffer{}
	_, err := PromptDeleteSession(testSessions(), &errReader{}, writer)
	if err == nil {
		t.Error("expected error from reader error")
	}
	if !strings.Contains(err.Error(), "failed to read input") {
		t.Errorf("error = %q, want 'failed to read input'", err.Error())
	}
}

func TestConfirmDelete_NoInput(t *testing.T) {
	reader := strings.NewReader("")
	writer := &bytes.Buffer{}

	confirmed, err := ConfirmDelete("test", "aaaaaaaa-1111-1111-1111-111111111111", reader, writer)
	if err != nil {
		t.Fatalf("ConfirmDelete failed: %v", err)
	}
	if confirmed {
		t.Error("expected confirmed=false for no input (EOF)")
	}
}

// --- truncate tests ---

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is too long", 10, "this is..."},
		{"a", 1, "a"},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.max)
		if got != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.expected)
		}
	}
}
