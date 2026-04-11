package menu

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/asymmetric-effort/claude-shell/internal/session"
)

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

func TestDisplay_NewSession(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("1\n")
	writer := &bytes.Buffer{}

	sel, err := Display(sessions, reader, writer)
	if err != nil {
		t.Fatalf("Display failed: %v", err)
	}
	if sel.Action != ActionNewSession {
		t.Errorf("Action = %q, want %q", sel.Action, ActionNewSession)
	}
}

func TestDisplay_DefaultSelection(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("\n")
	writer := &bytes.Buffer{}

	sel, err := Display(sessions, reader, writer)
	if err != nil {
		t.Fatalf("Display failed: %v", err)
	}
	if sel.Action != ActionNewSession {
		t.Errorf("default should be new session, got %q", sel.Action)
	}
}

func TestDisplay_ResumeSession(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("2\n")
	writer := &bytes.Buffer{}

	sel, err := Display(sessions, reader, writer)
	if err != nil {
		t.Fatalf("Display failed: %v", err)
	}
	if sel.SessionID != "aaaaaaaa-1111-1111-1111-111111111111" {
		t.Errorf("SessionID = %q, want first session UUID", sel.SessionID)
	}
}

func TestDisplay_ResumeSecondSession(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("3\n")
	writer := &bytes.Buffer{}

	sel, err := Display(sessions, reader, writer)
	if err != nil {
		t.Fatalf("Display failed: %v", err)
	}
	if sel.SessionID != "bbbbbbbb-2222-2222-2222-222222222222" {
		t.Errorf("SessionID = %q, want second session UUID", sel.SessionID)
	}
}

func TestDisplay_DeleteAction(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("d\n")
	writer := &bytes.Buffer{}

	sel, err := Display(sessions, reader, writer)
	if err != nil {
		t.Fatalf("Display failed: %v", err)
	}
	if sel.Action != ActionDeleteSession {
		t.Errorf("Action = %q, want %q", sel.Action, ActionDeleteSession)
	}
}

func TestDisplay_DeleteUppercase(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("D\n")
	writer := &bytes.Buffer{}

	sel, err := Display(sessions, reader, writer)
	if err != nil {
		t.Fatalf("Display failed: %v", err)
	}
	if sel.Action != ActionDeleteSession {
		t.Errorf("Action = %q, want %q", sel.Action, ActionDeleteSession)
	}
}

func TestDisplay_InvalidSelection(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("99\n")
	writer := &bytes.Buffer{}

	_, err := Display(sessions, reader, writer)
	if err == nil {
		t.Error("expected error for invalid selection, got nil")
	}
}

func TestDisplay_NonNumericInput(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("xyz\n")
	writer := &bytes.Buffer{}

	_, err := Display(sessions, reader, writer)
	if err == nil {
		t.Error("expected error for non-numeric input, got nil")
	}
}

func TestDisplay_NoInput(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("")
	writer := &bytes.Buffer{}

	_, err := Display(sessions, reader, writer)
	if err == nil {
		t.Error("expected error for no input, got nil")
	}
}

func TestDisplay_EmptySessions(t *testing.T) {
	reader := strings.NewReader("1\n")
	writer := &bytes.Buffer{}

	sel, err := Display(nil, reader, writer)
	if err != nil {
		t.Fatalf("Display failed: %v", err)
	}
	if sel.Action != ActionNewSession {
		t.Errorf("Action = %q, want %q", sel.Action, ActionNewSession)
	}
}

func TestDisplay_MenuOutput(t *testing.T) {
	sessions := testSessions()
	reader := strings.NewReader("1\n")
	writer := &bytes.Buffer{}

	_, _ = Display(sessions, reader, writer)

	output := writer.String()
	if !strings.Contains(output, "Session Manager") {
		t.Error("menu output missing header")
	}
	if !strings.Contains(output, "New Session") {
		t.Error("menu output missing New Session option")
	}
	if !strings.Contains(output, "first-session") {
		t.Error("menu output missing first session")
	}
	if !strings.Contains(output, "second-session") {
		t.Error("menu output missing second session")
	}
	if !strings.Contains(output, "Delete") {
		t.Error("menu output missing delete option")
	}
}

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
