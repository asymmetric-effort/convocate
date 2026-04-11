// Package menu provides the interactive session selection menu for claude-shell.
package menu

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/asymmetric-effort/claude-shell/internal/session"
)

const (
	// ActionNewSession indicates the user wants to create a new session.
	ActionNewSession = "new"
	// ActionDeleteSession indicates the user wants to delete a session.
	ActionDeleteSession = "delete"
)

// Selection represents the user's menu choice.
type Selection struct {
	Action    string
	SessionID string
}

// Display renders the session menu and returns the user's selection.
func Display(sessions []session.Metadata, reader io.Reader, writer io.Writer) (Selection, error) {
	printMenu(sessions, writer)

	scanner := bufio.NewScanner(reader)
	fmt.Fprint(writer, "\nSelect option: ")

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return Selection{}, fmt.Errorf("failed to read input: %w", err)
		}
		return Selection{}, fmt.Errorf("no input received")
	}

	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		input = "1"
	}

	return parseSelection(input, sessions)
}

// PromptSessionName asks the user for a session name.
func PromptSessionName(reader io.Reader, writer io.Writer) (string, error) {
	fmt.Fprint(writer, "Session name: ")
	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}
		return "", fmt.Errorf("no input received")
	}
	return strings.TrimSpace(scanner.Text()), nil
}

// PromptDeleteSession asks which session to delete.
func PromptDeleteSession(sessions []session.Metadata, reader io.Reader, writer io.Writer) (string, error) {
	fmt.Fprintln(writer, "\nSelect session to delete:")
	for i, s := range sessions {
		fmt.Fprintf(writer, "  %d | %-20s | %s\n", i+1, s.Name, s.UUID)
	}
	fmt.Fprint(writer, "\nSession number (or 'c' to cancel): ")

	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}
		return "", fmt.Errorf("no input received")
	}

	input := strings.TrimSpace(scanner.Text())
	if strings.ToLower(input) == "c" {
		return "", nil
	}

	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(sessions) {
		return "", fmt.Errorf("invalid selection: %s", input)
	}

	return sessions[idx-1].UUID, nil
}

// ConfirmDelete asks the user to confirm session deletion.
func ConfirmDelete(name, id string, reader io.Reader, writer io.Writer) (bool, error) {
	fmt.Fprintf(writer, "Delete session %q (%s)? [y/N]: ", name, id[:8])
	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		return false, nil
	}
	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return input == "y" || input == "yes", nil
}

func printMenu(sessions []session.Metadata, writer io.Writer) {
	fmt.Fprintln(writer, "")
	fmt.Fprintln(writer, "claude-shell - Session Manager")
	fmt.Fprintln(writer, strings.Repeat("-", 80))
	fmt.Fprintf(writer, "  %-4s| %-20s | %-36s | %-12s | %s\n", "#", "Name", "Session ID", "Created", "Last Accessed")
	fmt.Fprintln(writer, strings.Repeat("-", 80))
	fmt.Fprintf(writer, "  %-4s| %-20s | %-36s | %-12s | %s\n", "1", "+ New Session", "", "", "")

	for i, s := range sessions {
		status := ""
		if s.UUID != "" {
			status = ""
		}
		_ = status
		fmt.Fprintf(writer, "  %-4s| %-20s | %s | %-12s | %s\n",
			strconv.Itoa(i+2),
			truncate(s.Name, 20),
			s.UUID,
			s.CreatedAt.Format("2006-01-02"),
			s.LastAccessed.Format("2006-01-02"),
		)
	}

	fmt.Fprintln(writer, strings.Repeat("-", 80))
	fmt.Fprintf(writer, "  %-4s| %-20s |\n", "D", "Delete a session")
	fmt.Fprintln(writer, strings.Repeat("-", 80))
}

func parseSelection(input string, sessions []session.Metadata) (Selection, error) {
	if strings.ToLower(input) == "d" {
		return Selection{Action: ActionDeleteSession}, nil
	}

	idx, err := strconv.Atoi(input)
	if err != nil {
		return Selection{}, fmt.Errorf("invalid selection: %q", input)
	}

	if idx == 1 {
		return Selection{Action: ActionNewSession}, nil
	}

	sessionIdx := idx - 2
	if sessionIdx < 0 || sessionIdx >= len(sessions) {
		return Selection{}, fmt.Errorf("invalid selection: %d (valid range: 1-%d, D)", idx, len(sessions)+1)
	}

	return Selection{
		Action:    sessions[sessionIdx].UUID,
		SessionID: sessions[sessionIdx].UUID,
	}, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
