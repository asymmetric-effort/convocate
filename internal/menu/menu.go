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
	// ActionCloneSession indicates the user wants to clone an existing session.
	ActionCloneSession = "clone"
	// ActionDeleteSession indicates the user wants to delete a session.
	ActionDeleteSession = "delete"
	// ActionReload indicates the UI should refresh the session list.
	// It is emitted by the periodic auto-refresh timer, not by a user key.
	ActionReload = "reload"
	// ActionRestart indicates the user wants to start the selected session
	// container in detached/background mode so it runs autonomously.
	ActionRestart = "restart"
	// ActionOverrideLock indicates the user wants to override a stale session lock.
	ActionOverrideLock = "override-lock"
	// ActionBackground indicates the user wants to disconnect an attached
	// terminal from a connected session while leaving the container running.
	ActionBackground = "background"
	// ActionQuit indicates the user wants to quit the shell.
	ActionQuit = "quit"
)

// Selection represents the user's menu choice.
type Selection struct {
	Action    string
	SessionID string
	Name      string
	// Port is the port to publish for a new session. 0 means no port,
	// session.PortAuto (-1) means auto-assign, and any positive value is a
	// specific port to publish.
	Port int
	// Protocol is the transport protocol for the published port: "tcp" or
	// "udp". Empty is treated as "tcp" by callers.
	Protocol string
}

// parsePortInput validates the raw port field entered by the user in the
// create dialog. An empty string maps to 0 (no port published); "0" maps to
// session.PortAuto requesting auto-assignment; anything else must be a
// decimal integer in the range 1-65535.
func parsePortInput(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("port must be a number")
	}
	if n == 0 {
		return session.PortAuto, nil
	}
	if n < 1 || n > 65535 {
		return 0, fmt.Errorf("port must be between 0 and 65535")
	}
	return n, nil
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

func parseSelection(input string, sessions []session.Metadata) (Selection, error) {
	switch strings.ToLower(input) {
	case "c":
		return Selection{Action: ActionNewSession}, nil
	case "d":
		return Selection{Action: ActionDeleteSession}, nil
	case "r":
		return Selection{Action: ActionReload}, nil
	case "q":
		return Selection{Action: ActionQuit}, nil
	}

	idx, err := strconv.Atoi(input)
	if err != nil {
		return Selection{}, fmt.Errorf("invalid selection: %q", input)
	}

	sessionIdx := idx - 1
	if sessionIdx < 0 || sessionIdx >= len(sessions) {
		return Selection{}, fmt.Errorf("invalid selection: %d (valid range: 1-%d, C, D, Q)", idx, len(sessions))
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
