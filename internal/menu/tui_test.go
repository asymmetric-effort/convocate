package menu

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/asymmetric-effort/convocate/internal/session"
)

const (
	testScreenWidth  = 110
	testScreenHeight = 30
)

// newTestScreen creates and initializes a simulation screen.
func newTestScreen(t *testing.T) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("failed to init simulation screen: %v", err)
	}
	screen.SetSize(testScreenWidth, testScreenHeight)
	return screen
}

// runWithKey posts a key event and runs DisplayWithScreen, returning the selection.
func runWithKey(t *testing.T, sessions []session.Metadata, key tcell.Key, ch rune) Selection {
	t.Helper()
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		// Small delay to let the event loop start
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(key, ch, tcell.ModNone)
	}()

	// Pretend every session is running — item #7 intercepts Enter on
	// a remote-but-stopped session to show the not-running dialog;
	// without this stub, tests that press Enter hang in that dialog.
	sel, err := DisplayWithScreen(sessions, screen, DisplayOptions{
		Agents:    []string{"test-agent"},
		IsRunning: func(string) bool { return true },
	})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	return sel
}

// getScreenText reads all cells in a row and returns the string content.
func getScreenText(screen tcell.SimulationScreen, row, width int) string {
	var sb strings.Builder
	for x := 0; x < width; x++ {
		ch, _, _, _ := screen.GetContent(x, row)
		sb.WriteRune(ch)
	}
	return sb.String()
}

// --- Display function tests ---

func TestDisplay_ScreenFactoryError(t *testing.T) {
	orig := screenFactory
	defer func() { screenFactory = orig }()

	screenFactory = func() (tcell.Screen, error) {
		return nil, errors.New("no terminal")
	}

	_, err := Display(testSessions(), DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err == nil {
		t.Error("expected error when screen factory fails")
	}
	if !strings.Contains(err.Error(), "no terminal") {
		t.Errorf("error = %q, want 'no terminal'", err.Error())
	}
}

func TestDisplay_FullPath(t *testing.T) {
	orig := screenFactory
	defer func() { screenFactory = orig }()

	screenCh := make(chan tcell.SimulationScreen, 1)
	screenFactory = func() (tcell.Screen, error) {
		s := tcell.NewSimulationScreen("")
		if err := s.Init(); err != nil {
			return nil, err
		}
		screenCh <- s
		return s, nil
	}

	go func() {
		simScreen := <-screenCh
		time.Sleep(20 * time.Millisecond)
		simScreen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	sel, err := Display(testSessions(), DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("Display failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

// --- Key handling tests ---

func TestTUI_QuitOnQ(t *testing.T) {
	sel := runWithKey(t, testSessions(), tcell.KeyRune, 'q')
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

func TestTUI_QuitOnUpperQ(t *testing.T) {
	sel := runWithKey(t, testSessions(), tcell.KeyRune, 'Q')
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

func TestTUI_QuitOnEscape(t *testing.T) {
	sel := runWithKey(t, testSessions(), tcell.KeyEscape, 0)
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

func TestTUI_CreateDialog_TypeAndEnter(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'n', tcell.ModNone) // open dialog
		time.Sleep(10 * time.Millisecond)
		for _, ch := range "my-session" {
			screen.InjectKey(tcell.KeyRune, ch, tcell.ModNone)
			time.Sleep(5 * time.Millisecond)
		}
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone) // confirm
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionNewSession {
		t.Errorf("Action = %q, want %q", sel.Action, ActionNewSession)
	}
	if sel.Name != "my-session" {
		t.Errorf("Name = %q, want %q", sel.Name, "my-session")
	}
}

func TestTUI_CreateDialog_UpperN(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'N', tcell.ModNone) // open dialog
		time.Sleep(10 * time.Millisecond)
		for _, ch := range "test" {
			screen.InjectKey(tcell.KeyRune, ch, tcell.ModNone)
			time.Sleep(5 * time.Millisecond)
		}
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionNewSession {
		t.Errorf("Action = %q, want %q", sel.Action, ActionNewSession)
	}
	if sel.Name != "test" {
		t.Errorf("Name = %q, want %q", sel.Name, "test")
	}
}

func TestTUI_CreateDialog_EscapeCancels(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'n', tcell.ModNone) // open dialog
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone) // cancel
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone) // quit
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q (escape should cancel dialog)", sel.Action, ActionQuit)
	}
}

func TestTUI_CreateDialog_EmptyNameShowsError(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'n', tcell.ModNone) // open dialog
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone) // empty name
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone) // cancel
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone) // quit
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

func TestTUI_CreateDialog_InvalidCharShowsError(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'n', tcell.ModNone)
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, '!', tcell.ModNone) // invalid char
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone) // try to submit
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

func TestTUI_CreateDialog_Backspace(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'n', tcell.ModNone)
		time.Sleep(10 * time.Millisecond)
		for _, ch := range "testx" {
			screen.InjectKey(tcell.KeyRune, ch, tcell.ModNone)
			time.Sleep(5 * time.Millisecond)
		}
		screen.InjectKey(tcell.KeyBackspace2, 0, tcell.ModNone) // delete 'x'
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Name != "test" {
		t.Errorf("Name = %q, want %q", sel.Name, "test")
	}
}

func TestTUI_DeleteDialog_ConfirmYes(t *testing.T) {
	sessions := testSessions()
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'd', tcell.ModNone) // open delete dialog
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'y', tcell.ModNone) // confirm
	}()

	sel, err := DisplayWithScreen(sessions, screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionDeleteSession {
		t.Errorf("Action = %q, want %q", sel.Action, ActionDeleteSession)
	}
	if sel.SessionID != sessions[0].UUID {
		t.Errorf("SessionID = %q, want %q", sel.SessionID, sessions[0].UUID)
	}
}

func TestTUI_DeleteDialog_ConfirmNo(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'd', tcell.ModNone)
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'n', tcell.ModNone) // decline
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone) // quit
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

func TestTUI_DeleteDialog_EscapeCancels(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'd', tcell.ModNone)
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone) // cancel
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

func TestTUI_DeleteDialog_EmptySessionsIgnored(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'd', tcell.ModNone) // no sessions, should be ignored
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(nil, screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

func TestTUI_RestartOnR_OpensDialog(t *testing.T) {
	// 'R' on a non-running session opens the restart confirm dialog; since the
	// dialog doesn't finish the loop, we also need to quit afterward.
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'r', tcell.ModNone)
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone) // cancel dialog
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q (restart dialog should be cancellable)", sel.Action, ActionQuit)
	}
}

func TestTUI_SelectSessionWithEnter(t *testing.T) {
	sessions := testSessions()
	// Cursor starts at 0, so Enter selects first session
	sel := runWithKey(t, sessions, tcell.KeyEnter, 0)
	if sel.SessionID != sessions[0].UUID {
		t.Errorf("SessionID = %q, want %q", sel.SessionID, sessions[0].UUID)
	}
}

func TestTUI_SelectSessionWithNumber(t *testing.T) {
	sessions := testSessions()
	sel := runWithKey(t, sessions, tcell.KeyRune, '2')
	if sel.SessionID != sessions[1].UUID {
		t.Errorf("SessionID = %q, want %q", sel.SessionID, sessions[1].UUID)
	}
}

func TestTUI_NumberOutOfRange(t *testing.T) {
	// Pressing '9' with only 2 sessions should not select anything
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, '9', tcell.ModNone) // ignored — out of range
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone) // quit
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q (out-of-range number should be ignored)", sel.Action, ActionQuit)
	}
}

func TestTUI_ArrowDownThenEnter(t *testing.T) {
	sessions := testSessions()
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyDown, 0, tcell.ModNone)
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(sessions, screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.SessionID != sessions[1].UUID {
		t.Errorf("SessionID = %q, want %q (second session)", sel.SessionID, sessions[1].UUID)
	}
}

func TestTUI_ArrowUpClampsAtZero(t *testing.T) {
	sessions := testSessions()
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyUp, 0, tcell.ModNone) // already at 0, should stay
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(sessions, screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.SessionID != sessions[0].UUID {
		t.Errorf("SessionID = %q, want %q (first session)", sel.SessionID, sessions[0].UUID)
	}
}

func TestTUI_EmptySessions_EnterIgnored(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone) // no sessions, should be ignored
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(nil, screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

// --- Screen content tests ---

func TestTUI_TitleBarContent(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	row := getScreenText(screen, 0, testScreenWidth)
	if !strings.Contains(row, "convocate") {
		t.Errorf("title bar missing 'convocate', got: %q", row)
	}
}

func TestTUI_TitleBarStyle(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	_, _, style, _ := screen.GetContent(1, 0)
	fg, bg, _ := style.Decompose()
	if fg != tcell.ColorWhite {
		t.Errorf("title bar fg = %v, want White", fg)
	}
	if bg != tcell.ColorBlue {
		t.Errorf("title bar bg = %v, want Blue", bg)
	}
}

func TestTUI_MenuBarContent(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	row := getScreenText(screen, testScreenHeight-1, testScreenWidth)
	for _, label := range []string{"(N)ew", "(D)elete", "(R)estart", "(Q)uit"} {
		if !strings.Contains(row, label) {
			t.Errorf("menu bar missing %q, got: %q", label, row)
		}
	}
}

func TestTUI_MenuBarStyle(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	_, _, style, _ := screen.GetContent(1, testScreenHeight-1)
	fg, bg, _ := style.Decompose()
	if fg != tcell.ColorYellow {
		t.Errorf("menu bar fg = %v, want Yellow", fg)
	}
	if bg != tcell.ColorBlue {
		t.Errorf("menu bar bg = %v, want Blue", bg)
	}
}

func TestTUI_SessionsDisplayed(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	// Check that session names appear in the session area (rows 4+)
	found := false
	for row := 4; row < testScreenHeight-1; row++ {
		text := getScreenText(screen, row, testScreenWidth)
		if strings.Contains(text, "first-session") {
			found = true
			break
		}
	}
	if !found {
		t.Error("session list missing 'first-session'")
	}
}

func TestTUI_EmptySessionsMessage(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(nil, screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	found := false
	for row := 4; row < testScreenHeight-1; row++ {
		text := getScreenText(screen, row, testScreenWidth)
		if strings.Contains(text, "No sessions") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'No sessions' message when session list is empty")
	}
}

func TestTUI_ColumnHeaders(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	headerRow := getScreenText(screen, 2, testScreenWidth)
	for _, col := range []string{"#", "Name", "Session ID", "Created", "Last Accessed"} {
		if !strings.Contains(headerRow, col) {
			t.Errorf("header row missing %q, got: %q", col, headerRow)
		}
	}
}

// --- Scrolling tests ---

func TestTUI_ScrollWithManySessions(t *testing.T) {
	// Create more sessions than can fit on screen
	sessions := make([]session.Metadata, 50)
	for i := range sessions {
		sessions[i] = session.Metadata{
			UUID:         strings.Replace("aaaaaaaa-0000-0000-0000-000000000000", "0000", strings.Repeat("0", 4), -1),
			Name:         "session-" + strings.Repeat("0", 2-len(string(rune('0'+i%10)))) + string(rune('0'+i%10)),
			CreatedAt:    time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			LastAccessed: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		}
	}

	screen := newTestScreen(t)
	defer screen.Fini()

	// Navigate down past the visible area then quit
	go func() {
		time.Sleep(10 * time.Millisecond)
		for i := 0; i < 30; i++ {
			screen.InjectKey(tcell.KeyDown, 0, tcell.ModNone)
			time.Sleep(5 * time.Millisecond)
		}
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(sessions, screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

func TestTUI_HandleKeyUnit(t *testing.T) {
	sessions := testSessions()
	// IsRunning=true so the Enter test below doesn't hit the
	// not-running redirect added for item #7.
	ui := &tui{sessions: sessions, cursor: 0, isRunningFunc: func(string) bool { return true }}

	// Test Down arrow
	sel, done := ui.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if done {
		t.Error("Down arrow should not finish")
	}
	_ = sel
	if ui.cursor != 1 {
		t.Errorf("cursor after Down = %d, want 1", ui.cursor)
	}

	// Test Down at end (clamp)
	ui.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if ui.cursor != 1 {
		t.Errorf("cursor should clamp at %d, got %d", len(sessions)-1, ui.cursor)
	}

	// Test Up
	ui.handleKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if ui.cursor != 0 {
		t.Errorf("cursor after Up = %d, want 0", ui.cursor)
	}

	// Test Up at start (clamp)
	ui.handleKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if ui.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", ui.cursor)
	}

	// Test Enter
	sel, done = ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !done {
		t.Error("Enter should finish")
	}
	if sel.SessionID != sessions[0].UUID {
		t.Errorf("SessionID = %q, want %q", sel.SessionID, sessions[0].UUID)
	}
}

func TestTUI_HandleKeyRunes_ImmediateReturn(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0}

	tests := []struct {
		rune   rune
		action string
	}{
		{'q', ActionQuit},
		{'Q', ActionQuit},
	}

	for _, tt := range tests {
		ui.mode = modeMenu
		sel, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, tt.rune, tcell.ModNone))
		if !done {
			t.Errorf("key %q should finish", tt.rune)
		}
		if sel.Action != tt.action {
			t.Errorf("key %q: Action = %q, want %q", tt.rune, sel.Action, tt.action)
		}
	}
}

func TestTUI_HandleKeyRunes_OpensDialog(t *testing.T) {
	// One agent registered so the N handler uses the single-agent
	// shortcut straight into modeCreateDialog.
	ui := &tui{sessions: testSessions(), cursor: 0, agents: []string{"a1"}}

	// 'n' opens create dialog
	ui.mode = modeMenu
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	if done {
		t.Error("'n' should not finish (opens dialog)")
	}
	if ui.mode != modeCreateDialog {
		t.Errorf("mode = %d, want modeCreateDialog", ui.mode)
	}

	// 'd' opens delete dialog
	ui.mode = modeMenu
	_, done = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))
	if done {
		t.Error("'d' should not finish (opens dialog)")
	}
	if ui.mode != modeDeleteConfirm {
		t.Errorf("mode = %d, want modeDeleteConfirm", ui.mode)
	}
}

func TestTUI_HandleKeyRunes_DIgnoredNoSessions(t *testing.T) {
	ui := &tui{sessions: nil, cursor: 0}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))
	if done {
		t.Error("'d' with no sessions should not finish")
	}
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu (should stay in menu)", ui.mode)
	}
}

func TestTUI_HandleKeyNumberSelect(t *testing.T) {
	sessions := testSessions()
	ui := &tui{sessions: sessions, cursor: 0}

	sel, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, '1', tcell.ModNone))
	if !done {
		t.Error("number key should finish")
	}
	if sel.SessionID != sessions[0].UUID {
		t.Errorf("SessionID = %q, want %q", sel.SessionID, sessions[0].UUID)
	}
}

// --- Resize event test ---

func TestTUI_ResizeEvent(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.SetSize(120, 40)
		ev := tcell.NewEventResize(120, 40)
		if err := screen.PostEvent(ev); err != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

// --- Tiny screen test (early return in draw) ---

func TestTUI_TinyScreen(t *testing.T) {
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("failed to init screen: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(10, 3) // too small: height < 4

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

func TestTUI_NarrowScreen(t *testing.T) {
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("failed to init screen: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(15, 3) // width < 20

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

// --- drawSessionTable: visibleRows < 1 ---

func TestTUI_MinimalHeight(t *testing.T) {
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("failed to init screen: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 5) // menuBarRow=4, sessionsStart=4, visibleRows=0

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

// --- drawSessionTable: cursor clamping ---

func TestTUI_CursorClampHigh(t *testing.T) {
	sessions := testSessions()
	screen := newTestScreen(t)
	ui := &tui{
		screen:   screen,
		sessions: sessions,
		cursor:   99, // way past end
	}
	_, height := screen.Size()
	ui.drawSessionTable(testScreenWidth, height)
	if ui.cursor != len(sessions)-1 {
		t.Errorf("cursor = %d, want %d", ui.cursor, len(sessions)-1)
	}
	screen.Fini()
}

func TestTUI_CursorClampNegative(t *testing.T) {
	sessions := testSessions()
	screen := newTestScreen(t)
	ui := &tui{
		screen:   screen,
		sessions: sessions,
		cursor:   -5, // negative
	}
	_, height := screen.Size()
	ui.drawSessionTable(testScreenWidth, height)
	if ui.cursor != 0 {
		t.Errorf("cursor = %d, want 0", ui.cursor)
	}
	screen.Fini()
}

// --- drawTitleBar: narrow width hides clock ---

func TestTUI_TitleBarNarrowHidesClock(t *testing.T) {
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("failed to init screen: %v", err)
	}
	defer screen.Fini()
	// width=30: clock is 19 chars + 1 padding = 20, x = 30-20 = 10, but need x > 13
	// so clock should be hidden
	screen.SetSize(30, 10)

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	row := getScreenText(screen, 0, 30)
	if !strings.Contains(row, "convocate") {
		t.Error("title bar should still show 'convocate'")
	}
}

// --- handleKey: unrecognized rune ---

func TestTUI_UnrecognizedRuneIgnored(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModNone))
	if done {
		t.Error("unrecognized rune 'z' should not finish")
	}
}

// --- handleKey: Enter with empty sessions ---

func TestTUI_EnterEmptySessions(t *testing.T) {
	ui := &tui{sessions: nil, cursor: 0}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if done {
		t.Error("Enter with no sessions should not finish")
	}
}

// --- handleKey: Down with empty sessions ---

func TestTUI_DownEmptySessions(t *testing.T) {
	ui := &tui{sessions: nil, cursor: 0}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if done {
		t.Error("Down with no sessions should not finish")
	}
	if ui.cursor != 0 {
		t.Errorf("cursor = %d, want 0", ui.cursor)
	}
}

// --- scroll offset adjusts when cursor above offset ---

func TestTUI_ScrollOffsetAdjustsUp(t *testing.T) {
	sessions := testSessions()
	screen := newTestScreen(t)
	ui := &tui{
		screen:   screen,
		sessions: sessions,
		cursor:   0,
		offset:   1, // offset past cursor
	}
	_, height := screen.Size()
	ui.drawSessionTable(testScreenWidth, height)
	if ui.offset != 0 {
		t.Errorf("offset = %d, want 0 (should adjust to cursor)", ui.offset)
	}
	screen.Fini()
}

// --- Clock ticker test ---

func TestTUI_ClockTicker(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		// Wait for at least one tick (50ms interval) then quit
		time.Sleep(120 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	sel, err := displayWithOptions(testSessions(), screen, 50*time.Millisecond, 10*time.Second, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("displayWithOptions failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

// --- Create dialog: Name + Protocol + Port field ---

func TestTUI_CreateDialog_TabCyclesFourFields(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 0}
	want := []int{1, 2, 3, 0}
	for i, w := range want {
		_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
		if ui.activeField != w {
			t.Errorf("Tab %d: activeField = %d, want %d", i+1, ui.activeField, w)
		}
	}
}

func TestTUI_CreateDialog_ShiftTabCyclesBackward(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 0}
	want := []int{3, 2, 1, 0}
	for i, w := range want {
		_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone))
		if ui.activeField != w {
			t.Errorf("Shift+Tab %d: activeField = %d, want %d", i+1, ui.activeField, w)
		}
	}
}

func TestTUI_CreateDialog_PortFieldDigitsOnly(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 2}
	for _, ch := range "80ab80" {
		_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	got := string(ui.inputBufPort)
	if got != "8080" {
		t.Errorf("port buffer = %q, want %q (non-digits should be ignored)", got, "8080")
	}
}

func TestTUI_CreateDialog_EnterReturnsPort(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'n', tcell.ModNone)
		time.Sleep(10 * time.Millisecond)
		for _, ch := range "my-session" {
			screen.InjectKey(tcell.KeyRune, ch, tcell.ModNone)
			time.Sleep(2 * time.Millisecond)
		}
		time.Sleep(5 * time.Millisecond)
		// Tab past Protocol onto Port.
		screen.InjectKey(tcell.KeyTab, 0, tcell.ModNone)
		time.Sleep(5 * time.Millisecond)
		screen.InjectKey(tcell.KeyTab, 0, tcell.ModNone)
		time.Sleep(5 * time.Millisecond)
		for _, ch := range "8080" {
			screen.InjectKey(tcell.KeyRune, ch, tcell.ModNone)
			time.Sleep(2 * time.Millisecond)
		}
		time.Sleep(5 * time.Millisecond)
		screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionNewSession {
		t.Errorf("Action = %q, want %q", sel.Action, ActionNewSession)
	}
	if sel.Name != "my-session" {
		t.Errorf("Name = %q, want %q", sel.Name, "my-session")
	}
	if sel.Port != 8080 {
		t.Errorf("Port = %d, want 8080", sel.Port)
	}
	if sel.Protocol != "tcp" {
		t.Errorf("Protocol = %q, want 'tcp' (default)", sel.Protocol)
	}
}

func TestTUI_CreateDialog_InvalidPortShowsError(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 2}
	// Populate name so name validation passes
	ui.inputBuf = []rune("test")
	ui.inputProtocol = "tcp"
	ui.inputBufPort = []rune("70000")
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if done {
		t.Error("Enter with invalid port should not finish")
	}
	if ui.dialogErr == "" {
		t.Error("expected dialog error for out-of-range port")
	}
	if ui.activeField != 2 {
		t.Errorf("expected focus to return to Port field, activeField=%d", ui.activeField)
	}
}

func TestTUI_SessionTable_PortColumn(t *testing.T) {
	sessions := []session.Metadata{
		{
			UUID:         "aaaaaaaa-1111-1111-1111-111111111111",
			Name:         "with-port",
			CreatedAt:    time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
			LastAccessed: time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
			Port:         8080,
		},
		{
			UUID:         "bbbbbbbb-2222-2222-2222-222222222222",
			Name:         "no-port",
			CreatedAt:    time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC),
			LastAccessed: time.Date(2026, 4, 9, 0, 0, 0, 0, time.UTC),
		},
	}

	screen := newWideTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(sessions, screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	// Header row must include "Port"
	header := getScreenText(screen, 2, 120)
	if !strings.Contains(header, "Port") {
		t.Errorf("header missing 'Port' column, got: %q", header)
	}

	// Row 0: 8080 visible
	row0 := getScreenText(screen, 4, 120)
	if !strings.Contains(row0, "8080") {
		t.Errorf("row 0 missing '8080', got: %q", row0)
	}

	// Row 1: port dash placeholder
	row1 := getScreenText(screen, 5, 120)
	if !strings.Contains(row1, " -     ") {
		t.Errorf("row 1 expected '-' placeholder in port column, got: %q", row1)
	}
}

// --- Title bar load averages ---

func TestTUI_TitleBarLoadAverages(t *testing.T) {
	orig := loadAverageReader
	defer func() { loadAverageReader = orig }()
	loadAverageReader = func() (string, bool) { return "0.10 0.20 0.30", true }

	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	row := getScreenText(screen, 0, testScreenWidth)
	if !strings.Contains(row, "0.10 0.20 0.30") {
		t.Errorf("title bar missing load averages, got: %q", row)
	}
	clock := time.Now().Format("2006-01-02 15:04:05")
	// Two spaces between load averages and the timestamp.
	idxLoad := strings.Index(row, "0.10 0.20 0.30")
	idxClock := strings.Index(row, clock[:10]) // match at least the date
	if idxLoad < 0 || idxClock < 0 {
		t.Fatalf("load/clock not found in row: %q", row)
	}
	gap := row[idxLoad+len("0.10 0.20 0.30") : idxClock]
	if gap != "  " {
		t.Errorf("expected two-space gap between load and clock, got: %q", gap)
	}
}

func TestTUI_TitleBarLoadAverages_Unavailable(t *testing.T) {
	orig := loadAverageReader
	defer func() { loadAverageReader = orig }()
	loadAverageReader = func() (string, bool) { return "", false }

	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	row := getScreenText(screen, 0, testScreenWidth)
	if !strings.Contains(row, "convocate") {
		t.Errorf("title bar missing app name, got: %q", row)
	}
}

// --- Status indicator + (B)ackground tests ---

// newWideTestScreen creates a simulation screen wide enough to include the
// trailing status column, which gets clipped at the default test width.
func newWideTestScreen(t *testing.T) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("failed to init simulation screen: %v", err)
	}
	screen.SetSize(120, testScreenHeight)
	return screen
}

func TestTUI_StatusIndicator_Connected(t *testing.T) {
	sessions := testSessions()
	connected := sessions[0].UUID
	opts := DisplayOptions{
		IsRunning: func(id string) bool { return id == connected },
		IsLocked:  func(id string) bool { return id == connected },
	}

	screen := newWideTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(sessions, screen, opts)

	row := getScreenText(screen, 4, 120)
	trimmed := strings.TrimRight(row, " ")
	if !strings.HasSuffix(trimmed, " C") {
		t.Errorf("expected 'C' status at end of row, got: %q", row)
	}
}

func TestTUI_StatusIndicator_RunningNotLocked(t *testing.T) {
	sessions := testSessions()
	target := sessions[0].UUID
	opts := DisplayOptions{
		IsRunning: func(id string) bool { return id == target },
		IsLocked:  func(string) bool { return false },
	}

	screen := newWideTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(sessions, screen, opts)

	row := getScreenText(screen, 4, 120)
	trimmed := strings.TrimRight(row, " ")
	if !strings.HasSuffix(trimmed, " R") {
		t.Errorf("expected 'R' status at end of row, got: %q", row)
	}
}

func TestTUI_StatusIndicator_OrphanMarker(t *testing.T) {
	orphan := []session.Metadata{{
		UUID:         "orphan11-1111-1111-1111-111111111111",
		Name:         "orphan-session",
		CreatedAt:    time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		LastAccessed: time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
	}}
	opts := DisplayOptions{
		IsRunning: func(string) bool { return true },
		IsLocked:  func(string) bool { return true },
	}
	screen := newWideTestScreen(t)
	defer screen.Fini()
	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()
	_, _ = DisplayWithScreen(orphan, screen, opts)
	row := getScreenText(screen, 4, 120)
	trimmed := strings.TrimRight(row, " ")
	if !strings.HasSuffix(trimmed, " O") {
		t.Errorf("expected 'O' orphan status, got: %q", row)
	}
}

func TestTUI_BackgroundKey_OpensDialog(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModNone))
	if done {
		t.Error("'b' should not finish (opens dialog)")
	}
	if ui.mode != modeBackgroundConfirm {
		t.Errorf("mode = %d, want modeBackgroundConfirm", ui.mode)
	}
}

func TestTUI_BackgroundDialog_ConfirmConnected(t *testing.T) {
	sessions := testSessions()
	target := sessions[0].UUID
	var backgroundCalledWith string
	ui := &tui{
		sessions:      sessions,
		cursor:        0,
		mode:          modeBackgroundConfirm,
		isRunningFunc: func(id string) bool { return id == target },
		isLockedFunc:  func(id string) bool { return id == target },
		backgroundFunc: func(id string) error {
			backgroundCalledWith = id
			return nil
		},
	}

	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if done {
		t.Error("confirm should not finish the menu loop")
	}
	if backgroundCalledWith != target {
		t.Errorf("backgroundFunc called with %q, want %q", backgroundCalledWith, target)
	}
	if ui.mode != modeBackgroundInitiated {
		t.Errorf("mode = %d, want modeBackgroundInitiated", ui.mode)
	}
}

func TestTUI_BackgroundDialog_NotConnected(t *testing.T) {
	sessions := testSessions()
	var backgroundCalled bool
	ui := &tui{
		sessions:      sessions,
		cursor:        0,
		mode:          modeBackgroundConfirm,
		isRunningFunc: func(string) bool { return false },
		isLockedFunc:  func(string) bool { return false },
		backgroundFunc: func(string) error {
			backgroundCalled = true
			return nil
		},
	}

	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if done {
		t.Error("confirm should not finish the menu loop")
	}
	if backgroundCalled {
		t.Error("backgroundFunc should not be called for non-connected session")
	}
	if ui.mode != modeNotConnectedDialog {
		t.Errorf("mode = %d, want modeNotConnectedDialog", ui.mode)
	}
}

func TestTUI_BackgroundDialog_Decline(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeBackgroundConfirm}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_SettingsKey_OpensDialog(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone))
	if done {
		t.Error("'s' should not finish (opens dialog)")
	}
	if ui.mode != modeSettingsDialog {
		t.Errorf("mode = %d, want modeSettingsDialog", ui.mode)
	}
}

func TestTUI_SettingsKey_UpperS(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'S', tcell.ModNone))
	if done {
		t.Error("'S' should not finish (opens dialog)")
	}
	if ui.mode != modeSettingsDialog {
		t.Errorf("mode = %d, want modeSettingsDialog", ui.mode)
	}
}

func TestTUI_SettingsKey_NoSessionsOK(t *testing.T) {
	ui := &tui{sessions: nil, cursor: 0}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone))
	if done {
		t.Error("'s' with no sessions should not finish")
	}
	if ui.mode != modeSettingsDialog {
		t.Errorf("mode = %d, want modeSettingsDialog (settings should open regardless of sessions)", ui.mode)
	}
}

func TestTUI_SettingsDialog_EscClosesDialog(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeSettingsDialog}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_SettingsDialog_EnterClosesDialog(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeSettingsDialog}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_SettingsDialog_TabIsNoop(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeSettingsDialog}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if ui.mode != modeSettingsDialog {
		t.Errorf("mode = %d, want modeSettingsDialog (Tab should not exit dialog)", ui.mode)
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone))
	if ui.mode != modeSettingsDialog {
		t.Errorf("mode = %d, want modeSettingsDialog (Shift+Tab should not exit dialog)", ui.mode)
	}
}

func TestTUI_MenuBarIncludesSettings(t *testing.T) {
	screen := newWideTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	row := getScreenText(screen, testScreenHeight-1, 120)
	if !strings.Contains(row, "(S)ettings") {
		t.Errorf("menu bar missing '(S)ettings', got: %q", row)
	}
}

func TestTUI_MenuBarIncludesBackground(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	row := getScreenText(screen, testScreenHeight-1, testScreenWidth)
	if !strings.Contains(row, "(B)ackground") {
		t.Errorf("menu bar missing '(B)ackground', got: %q", row)
	}
}

// --- Restart dialog flow ---

func TestTUI_RestartKey_NoSessionsIgnored(t *testing.T) {
	ui := &tui{sessions: nil, cursor: 0}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone))
	if done {
		t.Error("'r' with no sessions should not finish")
	}
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_RestartKey_OpensConfirmForStopped(t *testing.T) {
	ui := &tui{
		sessions:      testSessions(),
		cursor:        0,
		isRunningFunc: func(string) bool { return false },
	}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone))
	if done {
		t.Error("'r' should not finish (opens dialog)")
	}
	if ui.mode != modeRestartConfirm {
		t.Errorf("mode = %d, want modeRestartConfirm", ui.mode)
	}
}

func TestTUI_RestartKey_AlreadyRunningDialog(t *testing.T) {
	sessions := testSessions()
	ui := &tui{
		sessions:      sessions,
		cursor:        0,
		isRunningFunc: func(id string) bool { return id == sessions[0].UUID },
	}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'R', tcell.ModNone))
	if done {
		t.Error("'R' on running session should not finish")
	}
	if ui.mode != modeAlreadyRunningDialog {
		t.Errorf("mode = %d, want modeAlreadyRunningDialog", ui.mode)
	}
}

func TestTUI_RestartDialog_ConfirmYes_CallsCallback(t *testing.T) {
	sessions := testSessions()
	var called string
	ui := &tui{
		sessions:    sessions,
		cursor:      0,
		mode:        modeRestartConfirm,
		restartFunc: func(id string) error { called = id; return nil },
	}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if done {
		t.Error("confirm-yes should not finish the menu loop")
	}
	if called != sessions[0].UUID {
		t.Errorf("restartFunc called with %q, want %q", called, sessions[0].UUID)
	}
	if ui.mode != modeRestartInitiated {
		t.Errorf("mode = %d, want modeRestartInitiated", ui.mode)
	}
}

func TestTUI_RestartDialog_ConfirmYes_CallbackError(t *testing.T) {
	ui := &tui{
		sessions:    testSessions(),
		cursor:      0,
		mode:        modeRestartConfirm,
		restartFunc: func(string) error { return errors.New("boom") },
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'Y', tcell.ModNone))
	if ui.mode != modeRestartConfirm {
		t.Errorf("mode = %d, want modeRestartConfirm (stay in dialog on error)", ui.mode)
	}
	if ui.dialogErr != "boom" {
		t.Errorf("dialogErr = %q, want %q", ui.dialogErr, "boom")
	}
}

func TestTUI_RestartDialog_ConfirmYes_NoCallback(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeRestartConfirm}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if ui.mode != modeRestartInitiated {
		t.Errorf("mode = %d, want modeRestartInitiated", ui.mode)
	}
}

func TestTUI_RestartDialog_ConfirmYes_OutOfRangeCursor(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 99, mode: modeRestartConfirm}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu (bad cursor falls back to menu)", ui.mode)
	}
}

func TestTUI_RestartDialog_DeclineN(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeRestartConfirm}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'N', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_RestartDialog_Escape(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeRestartConfirm, dialogErr: "x"}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
	if ui.dialogErr != "" {
		t.Errorf("dialogErr = %q, want empty after escape", ui.dialogErr)
	}
}

func TestTUI_RestartInitiated_EnterDismisses(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeRestartInitiated}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu after Enter", ui.mode)
	}
}

func TestTUI_RestartInitiated_NonEnterKeyHolds(t *testing.T) {
	// Any key other than Enter must NOT dismiss — the dialog is a notification
	// that the user has to acknowledge with Enter.
	for _, ev := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone),
	} {
		ui := &tui{sessions: testSessions(), cursor: 0, mode: modeRestartInitiated}
		_, _ = ui.handleKey(ev)
		if ui.mode != modeRestartInitiated {
			t.Errorf("key %v: mode = %d, want modeRestartInitiated (only Enter dismisses)", ev.Key(), ui.mode)
		}
	}
}

func TestTUI_AlreadyRunning_AnyKeyDismisses(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeAlreadyRunningDialog}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_MenuBarShowsRestart(t *testing.T) {
	screen := newWideTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	row := getScreenText(screen, testScreenHeight-1, 120)
	if !strings.Contains(row, "(R)estart") {
		t.Errorf("menu bar missing '(R)estart', got: %q", row)
	}
	if strings.Contains(row, "(R)eload") {
		t.Errorf("menu bar should not contain '(R)eload', got: %q", row)
	}
}

// --- Draw tests for dialogs (exercise render paths) ---

func newDrawTUI(t *testing.T, mode tuiMode) (*tui, tcell.SimulationScreen) {
	t.Helper()
	screen := newWideTestScreen(t)
	return &tui{
		screen:   screen,
		sessions: testSessions(),
		cursor:   0,
		mode:     mode,
	}, screen
}

func TestTUI_DrawAllDialogs(t *testing.T) {
	modes := []struct {
		mode tuiMode
		err  string
	}{
		{modeCreateDialog, ""},
		{modeCreateDialog, "name error"},
		{modeCloneDialog, ""},
		{modeCloneDialog, "err"},
		{modeDeleteConfirm, ""},
		{modeLockedDialog, ""},
		{modeOverrideConfirm, ""},
		{modeOverrideConfirm, "err"},
		{modeKillConfirm, ""},
		{modeKillConfirm, "err"},
		{modeNotRunningDialog, ""},
		{modeKillInitiated, ""},
		{modeBackgroundConfirm, ""},
		{modeBackgroundConfirm, "err"},
		{modeBackgroundInitiated, ""},
		{modeNotConnectedDialog, ""},
		{modeSettingsDialog, ""},
		{modeRestartConfirm, ""},
		{modeRestartConfirm, "boom"},
		{modeRestartInitiated, ""},
		{modeAlreadyRunningDialog, ""},
	}
	for _, tc := range modes {
		ui, screen := newDrawTUI(t, tc.mode)
		ui.dialogErr = tc.err
		ui.draw()
		screen.Fini()
	}
}

func TestTUI_DrawDialogs_BadCursor(t *testing.T) {
	// Dialogs that dereference t.sessions[t.cursor] must early-return cleanly.
	modes := []tuiMode{
		modeCloneDialog,
		modeDeleteConfirm,
		modeLockedDialog,
		modeOverrideConfirm,
		modeKillConfirm,
		modeNotRunningDialog,
		modeKillInitiated,
		modeBackgroundConfirm,
		modeBackgroundInitiated,
		modeNotConnectedDialog,
		modeRestartConfirm,
		modeRestartInitiated,
		modeAlreadyRunningDialog,
	}
	for _, m := range modes {
		screen := newWideTestScreen(t)
		ui := &tui{screen: screen, sessions: nil, cursor: 0, mode: m}
		ui.draw() // should not panic
		screen.Fini()
	}
}

// --- Handler tests that were previously uncovered ---

func TestTUI_HandleLockedDialogKey(t *testing.T) {
	ui := &tui{mode: modeLockedDialog}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_HandleNotRunningDialogKey(t *testing.T) {
	ui := &tui{mode: modeNotRunningDialog}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_HandleKillInitiatedDialogKey(t *testing.T) {
	ui := &tui{mode: modeKillInitiated}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_HandleBackgroundInitiatedDialogKey(t *testing.T) {
	ui := &tui{mode: modeBackgroundInitiated}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_HandleNotConnectedDialogKey(t *testing.T) {
	ui := &tui{mode: modeNotConnectedDialog}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_HandleBackgroundDialog_Escape(t *testing.T) {
	ui := &tui{mode: modeBackgroundConfirm, dialogErr: "x"}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.mode != modeMenu || ui.dialogErr != "" {
		t.Errorf("escape did not clear dialog state")
	}
}

func TestTUI_HandleBackgroundDialog_OutOfRangeCursor(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 99, mode: modeBackgroundConfirm}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_HandleBackgroundDialog_CallbackError(t *testing.T) {
	sessions := testSessions()
	ui := &tui{
		sessions:       sessions,
		cursor:         0,
		mode:           modeBackgroundConfirm,
		isRunningFunc:  func(string) bool { return true },
		isLockedFunc:   func(string) bool { return true },
		backgroundFunc: func(string) error { return errors.New("nope") },
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if ui.mode != modeBackgroundConfirm {
		t.Errorf("mode = %d, want modeBackgroundConfirm (error should keep dialog open)", ui.mode)
	}
	if ui.dialogErr == "" {
		t.Error("expected dialogErr to be set on callback error")
	}
}

// --- Kill dialog / Override dialog handler tests ---

func TestTUI_HandleKillDialog_ConfirmRuns(t *testing.T) {
	sessions := testSessions()
	var killed string
	ui := &tui{
		sessions:      sessions,
		cursor:        0,
		mode:          modeKillConfirm,
		isRunningFunc: func(string) bool { return true },
		killFunc:      func(id string) error { killed = id; return nil },
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if killed != sessions[0].UUID {
		t.Errorf("killFunc called with %q, want %q", killed, sessions[0].UUID)
	}
	if ui.mode != modeKillInitiated {
		t.Errorf("mode = %d, want modeKillInitiated", ui.mode)
	}
}

func TestTUI_HandleKillDialog_NotRunning(t *testing.T) {
	ui := &tui{
		sessions:      testSessions(),
		cursor:        0,
		mode:          modeKillConfirm,
		isRunningFunc: func(string) bool { return false },
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if ui.mode != modeNotRunningDialog {
		t.Errorf("mode = %d, want modeNotRunningDialog", ui.mode)
	}
}

func TestTUI_HandleKillDialog_CallbackError(t *testing.T) {
	ui := &tui{
		sessions:      testSessions(),
		cursor:        0,
		mode:          modeKillConfirm,
		isRunningFunc: func(string) bool { return true },
		killFunc:      func(string) error { return errors.New("no") },
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'Y', tcell.ModNone))
	if ui.mode != modeKillConfirm {
		t.Errorf("mode = %d, want modeKillConfirm (stay on error)", ui.mode)
	}
	if ui.dialogErr == "" {
		t.Error("expected dialogErr")
	}
}

func TestTUI_HandleKillDialog_Decline(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeKillConfirm}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_HandleKillDialog_OutOfRange(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 99, mode: modeKillConfirm}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_HandleKillDialog_Escape(t *testing.T) {
	ui := &tui{mode: modeKillConfirm, dialogErr: "x"}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.mode != modeMenu || ui.dialogErr != "" {
		t.Errorf("escape did not reset")
	}
}

func TestTUI_HandleOverrideDialog_ConfirmYes(t *testing.T) {
	sessions := testSessions()
	var overrode string
	ui := &tui{
		sessions:         sessions,
		cursor:           0,
		mode:             modeOverrideConfirm,
		overrideLockFunc: func(id string) error { overrode = id; return nil },
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if overrode != sessions[0].UUID {
		t.Errorf("overrideLockFunc called with %q, want %q", overrode, sessions[0].UUID)
	}
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_HandleOverrideDialog_CallbackError(t *testing.T) {
	ui := &tui{
		sessions:         testSessions(),
		cursor:           0,
		mode:             modeOverrideConfirm,
		overrideLockFunc: func(string) error { return errors.New("x") },
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'Y', tcell.ModNone))
	if ui.mode != modeOverrideConfirm {
		t.Errorf("mode = %d, want modeOverrideConfirm (stay on error)", ui.mode)
	}
	if ui.dialogErr == "" {
		t.Error("expected dialogErr")
	}
}

func TestTUI_HandleOverrideDialog_NoCursor(t *testing.T) {
	ui := &tui{sessions: nil, cursor: 0, mode: modeOverrideConfirm}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_HandleOverrideDialog_Decline(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeOverrideConfirm}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_HandleOverrideDialog_Escape(t *testing.T) {
	ui := &tui{mode: modeOverrideConfirm, dialogErr: "x"}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.mode != modeMenu || ui.dialogErr != "" {
		t.Errorf("escape did not reset state")
	}
}

// --- handleMenuKey remaining branches (kill, override, enter-locked) ---

func TestTUI_HandleMenu_KillOpensDialog(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone))
	if ui.mode != modeKillConfirm {
		t.Errorf("mode = %d, want modeKillConfirm", ui.mode)
	}
}

func TestTUI_HandleMenu_OverrideOnlyWhenLocked(t *testing.T) {
	sessions := testSessions()
	ui := &tui{
		sessions:     sessions,
		cursor:       0,
		isLockedFunc: func(id string) bool { return id == sessions[0].UUID },
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone))
	if ui.mode != modeOverrideConfirm {
		t.Errorf("mode = %d, want modeOverrideConfirm", ui.mode)
	}

	// Unlocked: O is a no-op.
	ui2 := &tui{sessions: sessions, cursor: 0, isLockedFunc: func(string) bool { return false }}
	_, _ = ui2.handleKey(tcell.NewEventKey(tcell.KeyRune, 'O', tcell.ModNone))
	if ui2.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui2.mode)
	}
}

func TestTUI_HandleMenu_EnterOnLockedOpensLockDialog(t *testing.T) {
	sessions := testSessions()
	ui := &tui{
		sessions:     sessions,
		cursor:       0,
		isLockedFunc: func(id string) bool { return id == sessions[0].UUID },
	}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if done {
		t.Error("Enter on locked session should not finish")
	}
	if ui.mode != modeLockedDialog {
		t.Errorf("mode = %d, want modeLockedDialog", ui.mode)
	}
}

func TestTUI_HandleMenu_NumberOnLockedOpensLockDialog(t *testing.T) {
	sessions := testSessions()
	ui := &tui{
		sessions:     sessions,
		cursor:       0,
		isLockedFunc: func(id string) bool { return id == sessions[0].UUID },
	}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, '1', tcell.ModNone))
	if done {
		t.Error("number selecting locked session should not finish")
	}
	if ui.mode != modeLockedDialog {
		t.Errorf("mode = %d, want modeLockedDialog", ui.mode)
	}
}

func TestTUI_HandleMenu_CloneOpensDialog(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))
	if ui.mode != modeCloneDialog {
		t.Errorf("mode = %d, want modeCloneDialog", ui.mode)
	}
}

func TestTUI_HandleMenu_CloneNoSessionsIgnored(t *testing.T) {
	ui := &tui{sessions: nil, cursor: 0}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

// --- handleCloneDialogKey ---

func TestTUI_HandleCloneDialog_Enter(t *testing.T) {
	sessions := testSessions()
	ui := &tui{sessions: sessions, cursor: 0, mode: modeCloneDialog, inputBuf: []rune("clone-name")}
	sel, done := ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !done {
		t.Error("Enter should finish the clone dialog")
	}
	if sel.Action != ActionCloneSession || sel.Name != "clone-name" || sel.SessionID != sessions[0].UUID {
		t.Errorf("unexpected selection: %+v", sel)
	}
}

func TestTUI_HandleCloneDialog_InvalidName(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeCloneDialog, inputBuf: []rune("")}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if done {
		t.Error("Enter with empty name should not finish")
	}
	if ui.dialogErr == "" {
		t.Error("expected dialogErr for empty name")
	}
}

func TestTUI_HandleCloneDialog_OutOfRangeCursor(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 99, mode: modeCloneDialog, inputBuf: []rune("x")}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if done {
		t.Error("should not finish with bad cursor")
	}
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_HandleCloneDialog_Escape(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeCloneDialog, inputBuf: []rune("x"), dialogErr: "e"}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.mode != modeMenu || ui.dialogErr != "" || len(ui.inputBuf) != 0 {
		t.Errorf("escape did not reset state")
	}
}

func TestTUI_HandleCloneDialog_Rune(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeCloneDialog, dialogErr: "x"}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone))
	if string(ui.inputBuf) != "a" {
		t.Errorf("inputBuf = %q, want 'a'", string(ui.inputBuf))
	}
	if ui.dialogErr != "" {
		t.Error("rune should clear dialogErr")
	}
}

func TestTUI_HandleCloneDialog_Backspace(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeCloneDialog, inputBuf: []rune("ab"), dialogErr: "x"}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if string(ui.inputBuf) != "a" {
		t.Errorf("inputBuf after backspace = %q, want 'a'", string(ui.inputBuf))
	}
	if ui.dialogErr != "" {
		t.Error("backspace should clear dialogErr")
	}
}

func TestTUI_HandleCloneDialog_RuneMaxLength(t *testing.T) {
	ui := &tui{
		sessions: testSessions(),
		cursor:   0,
		mode:     modeCloneDialog,
		inputBuf: []rune(strings.Repeat("a", 64)),
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModNone))
	if len(ui.inputBuf) != 64 {
		t.Errorf("inputBuf length = %d, want 64 (cap enforced)", len(ui.inputBuf))
	}
}

// --- Create dialog: additional branches ---

func TestTUI_HandleCreateDialog_NameRuneMaxLength(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 0, inputBuf: []rune(strings.Repeat("a", 64))}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModNone))
	if len(ui.inputBuf) != 64 {
		t.Errorf("name length = %d, want cap 64", len(ui.inputBuf))
	}
}

func TestTUI_HandleCreateDialog_PortBackspace(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 2, inputBufPort: []rune("80"), dialogErr: "x"}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if string(ui.inputBufPort) != "8" {
		t.Errorf("port = %q, want '8'", string(ui.inputBufPort))
	}
	if ui.dialogErr != "" {
		t.Error("backspace should clear err")
	}
}

func TestTUI_HandleCreateDialog_PortMaxLength(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 2, inputBufPort: []rune("12345")}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, '6', tcell.ModNone))
	if string(ui.inputBufPort) != "12345" {
		t.Errorf("port buffer = %q, want '12345' (max 5 digits)", string(ui.inputBufPort))
	}
}

func TestTUI_CreateDialog_ProtocolToggleKeys(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 1, inputProtocol: "tcp"}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'u', tcell.ModNone))
	if ui.inputProtocol != "udp" {
		t.Errorf("'u' -> %q, want 'udp'", ui.inputProtocol)
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'T', tcell.ModNone))
	if ui.inputProtocol != "tcp" {
		t.Errorf("'T' -> %q, want 'tcp'", ui.inputProtocol)
	}
	// Space toggles.
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if ui.inputProtocol != "udp" {
		t.Errorf("Space from tcp -> %q, want 'udp'", ui.inputProtocol)
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if ui.inputProtocol != "tcp" {
		t.Errorf("Space from udp -> %q, want 'tcp'", ui.inputProtocol)
	}
}

func TestTUI_CreateDialog_ProtocolBackspaceNoOp(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 1, inputProtocol: "udp"}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if ui.inputProtocol != "udp" {
		t.Errorf("backspace on Protocol should not change value, got %q", ui.inputProtocol)
	}
}

func TestTUI_CreateDialog_ProtocolNonToggleKeyIgnored(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 1, inputProtocol: "tcp"}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if ui.inputProtocol != "tcp" {
		t.Errorf("'x' on Protocol should be ignored, got %q", ui.inputProtocol)
	}
}

func TestTUI_CreateDialog_DefaultsToTCPOnNKey(t *testing.T) {
	// agents=[1] so N's single-agent shortcut enters the form and seeds protocol.
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeMenu, agents: []string{"a1"}}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	if ui.inputProtocol != "tcp" {
		t.Errorf("N key should seed Protocol=tcp, got %q", ui.inputProtocol)
	}
}

func TestTUI_EditDialog_SeedsProtocolFromSession(t *testing.T) {
	sessions := []session.Metadata{
		{UUID: "aaaaaaaa-1111-1111-1111-111111111111", Name: "dns", Port: 53, Protocol: "udp"},
	}
	ui := &tui{sessions: sessions, cursor: 0, mode: modeMenu}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone))
	if ui.inputProtocol != "udp" {
		t.Errorf("E key should seed Protocol from session, got %q", ui.inputProtocol)
	}
}

func TestTUI_CreateDialog_ReturnsProtocolWithSelection(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 0}
	ui.inputBuf = []rune("svc")
	ui.inputProtocol = "udp"
	ui.inputBufPort = []rune("53")
	sel, done := ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !done {
		t.Fatal("Enter should finish")
	}
	if sel.Protocol != "udp" || sel.Port != 53 || sel.Name != "svc" {
		t.Errorf("unexpected selection: %+v", sel)
	}
}

func TestTUI_SessionTable_ShowsPortAndProtocol(t *testing.T) {
	sessions := []session.Metadata{
		{
			UUID:         "aaaaaaaa-1111-1111-1111-111111111111",
			Name:         "dns",
			CreatedAt:    time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
			LastAccessed: time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
			Port:         53,
			Protocol:     "udp",
		},
	}
	screen := newWideTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()
	_, _ = DisplayWithScreen(sessions, screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	row := getScreenText(screen, 4, 120)
	if !strings.Contains(row, "53/udp") {
		t.Errorf("row missing '53/udp', got: %q", row)
	}
}

func TestTUI_HandleCreateDialog_Escape(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 2, inputBuf: []rune("x"), inputBufPort: []rune("1"), dialogErr: "e"}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
	if len(ui.inputBuf) != 0 || len(ui.inputBufPort) != 0 || ui.dialogErr != "" || ui.activeField != 0 {
		t.Error("escape did not fully reset create-dialog state")
	}
}

// --- reload event path ---

func TestTUI_ReloadEventUpdatesSessions(t *testing.T) {
	reloadCalls := 0
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		// First tick should reload; then quit.
		time.Sleep(90 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	opts := DisplayOptions{
		Reload: func() ([]session.Metadata, error) {
			reloadCalls++
			return []session.Metadata{testSessions()[0]}, nil
		},
	}
	_, err := displayWithOptions(testSessions(), screen, 50*time.Millisecond, 50*time.Millisecond, opts)
	if err != nil {
		t.Fatalf("displayWithOptions failed: %v", err)
	}
	if reloadCalls == 0 {
		t.Error("reload callback was never invoked")
	}
}

func TestTUI_ReloadError_IsSwallowed(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(90 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	opts := DisplayOptions{
		Reload: func() ([]session.Metadata, error) { return nil, errors.New("boom") },
	}
	_, err := displayWithOptions(testSessions(), screen, 50*time.Millisecond, 50*time.Millisecond, opts)
	if err != nil {
		t.Fatalf("displayWithOptions should swallow reload errors, got: %v", err)
	}
}

func TestTUI_ReloadClampsCursor(t *testing.T) {
	// Start with 2 sessions, cursor at 1, then reload returns 1 session — cursor clamps.
	sessions := testSessions()
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyDown, 0, tcell.ModNone) // cursor -> 1
		time.Sleep(90 * time.Millisecond)                 // let reload fire
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	opts := DisplayOptions{
		Reload: func() ([]session.Metadata, error) {
			return []session.Metadata{sessions[0]}, nil
		},
	}
	_, _ = displayWithOptions(sessions, screen, 50*time.Millisecond, 50*time.Millisecond, opts)
}

// --- Edit dialog flow ---

func TestTUI_EditKey_OpensDialogWithCurrentValues(t *testing.T) {
	sessions := []session.Metadata{
		{
			UUID:         "aaaaaaaa-1111-1111-1111-111111111111",
			Name:         "proj",
			Port:         8080,
			CreatedAt:    time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
			LastAccessed: time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		},
	}
	ui := &tui{sessions: sessions, cursor: 0}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone))
	if ui.mode != modeEditDialog {
		t.Fatalf("mode = %d, want modeEditDialog", ui.mode)
	}
	if string(ui.inputBuf) != "proj" {
		t.Errorf("inputBuf = %q, want 'proj'", string(ui.inputBuf))
	}
	if string(ui.inputBufPort) != "8080" {
		t.Errorf("inputBufPort = %q, want '8080'", string(ui.inputBufPort))
	}
}

func TestTUI_EditKey_UpperE(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'E', tcell.ModNone))
	if ui.mode != modeEditDialog {
		t.Errorf("mode = %d, want modeEditDialog", ui.mode)
	}
}

func TestTUI_EditKey_NoSessionsIgnored(t *testing.T) {
	ui := &tui{sessions: nil, cursor: 0}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_EditKey_PortZeroNotPrepopulated(t *testing.T) {
	sessions := []session.Metadata{
		{UUID: "aaaaaaaa-1111-1111-1111-111111111111", Name: "noport", Port: 0},
	}
	ui := &tui{sessions: sessions, cursor: 0}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone))
	if len(ui.inputBufPort) != 0 {
		t.Errorf("expected empty port buffer for Port=0, got %q", string(ui.inputBufPort))
	}
}

func TestTUI_EditDialog_TabSwitchesFields(t *testing.T) {
	ui := &tui{mode: modeEditDialog, activeField: 0}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if ui.activeField != 1 {
		t.Errorf("Tab -> activeField = %d, want 1", ui.activeField)
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone))
	if ui.activeField != 0 {
		t.Errorf("Shift+Tab -> activeField = %d, want 0", ui.activeField)
	}
}

func TestTUI_EditDialog_TypeNameAndPort(t *testing.T) {
	ui := &tui{mode: modeEditDialog, activeField: 0}
	for _, r := range "my-session" {
		_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if string(ui.inputBuf) != "my-session" {
		t.Errorf("name buf = %q, want 'my-session'", string(ui.inputBuf))
	}
	// Tab past Protocol onto Port.
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	for _, r := range "9090a" { // 'a' filtered
		_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if string(ui.inputBufPort) != "9090" {
		t.Errorf("port buf = %q, want '9090'", string(ui.inputBufPort))
	}
}

func TestTUI_EditDialog_Backspace(t *testing.T) {
	ui := &tui{mode: modeEditDialog, activeField: 0, inputBuf: []rune("abc"), inputBufPort: []rune("80"), dialogErr: "x"}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if string(ui.inputBuf) != "ab" {
		t.Errorf("name after backspace = %q, want 'ab'", string(ui.inputBuf))
	}
	if ui.dialogErr != "" {
		t.Error("backspace should clear dialogErr")
	}
	ui.activeField = 2 // Port field
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if string(ui.inputBufPort) != "8" {
		t.Errorf("port after backspace = %q, want '8'", string(ui.inputBufPort))
	}
}

func TestTUI_EditDialog_NameMaxLength(t *testing.T) {
	ui := &tui{mode: modeEditDialog, activeField: 0, inputBuf: []rune(strings.Repeat("a", 64))}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModNone))
	if len(ui.inputBuf) != 64 {
		t.Errorf("name length = %d, want 64 (cap enforced)", len(ui.inputBuf))
	}
}

func TestTUI_EditDialog_PortMaxLength(t *testing.T) {
	ui := &tui{mode: modeEditDialog, activeField: 2, inputBufPort: []rune("12345")}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, '6', tcell.ModNone))
	if string(ui.inputBufPort) != "12345" {
		t.Errorf("port buffer = %q, want '12345' (max 5 digits)", string(ui.inputBufPort))
	}
}

func TestTUI_EditDialog_Escape(t *testing.T) {
	ui := &tui{
		mode:          modeEditDialog,
		activeField:   2,
		inputBuf:      []rune("x"),
		inputBufPort:  []rune("80"),
		inputProtocol: "udp",
		dialogErr:     "err",
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
	if len(ui.inputBuf) != 0 || len(ui.inputBufPort) != 0 || ui.inputProtocol != "" || ui.dialogErr != "" || ui.activeField != 0 {
		t.Error("escape did not reset state")
	}
}

func TestTUI_EditDialog_EnterInvalidName(t *testing.T) {
	ui := &tui{
		sessions:      testSessions(),
		cursor:        0,
		mode:          modeEditDialog,
		activeField:   2,
		inputBuf:      []rune(""),
		inputBufPort:  []rune("80"),
		inputProtocol: "tcp",
	}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if done {
		t.Error("invalid name should not finish")
	}
	if ui.dialogErr == "" {
		t.Error("expected dialogErr for empty name")
	}
	if ui.activeField != 0 {
		t.Errorf("activeField = %d, want 0 (focus returns to Name)", ui.activeField)
	}
}

func TestTUI_EditDialog_EnterInvalidPort(t *testing.T) {
	ui := &tui{
		sessions:      testSessions(),
		cursor:        0,
		mode:          modeEditDialog,
		activeField:   0,
		inputBuf:      []rune("good"),
		inputBufPort:  []rune("99999"),
		inputProtocol: "tcp",
	}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if done {
		t.Error("invalid port should not finish")
	}
	if ui.dialogErr == "" {
		t.Error("expected dialogErr for invalid port")
	}
	if ui.activeField != 2 {
		t.Errorf("activeField = %d, want 2 (focus returns to Port)", ui.activeField)
	}
}

func TestTUI_EditDialog_EnterCallsSave(t *testing.T) {
	sessions := testSessions()
	var savedID, savedName, savedProto string
	var savedPort int
	ui := &tui{
		sessions:      sessions,
		cursor:        0,
		mode:          modeEditDialog,
		inputBuf:      []rune("renamed"),
		inputBufPort:  []rune("8081"),
		inputProtocol: "udp",
		editSaveFunc: func(id, name, proto, _ string, port int) error {
			savedID, savedName, savedProto, savedPort = id, name, proto, port
			return nil
		},
	}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if done {
		t.Error("Save should transition to restart prompt, not finish")
	}
	if ui.mode != modeEditRestartPrompt {
		t.Errorf("mode = %d, want modeEditRestartPrompt", ui.mode)
	}
	if savedID != sessions[0].UUID {
		t.Errorf("save called with id %q, want %q", savedID, sessions[0].UUID)
	}
	if savedName != "renamed" {
		t.Errorf("save called with name %q, want 'renamed'", savedName)
	}
	if savedPort != 8081 {
		t.Errorf("save called with port %d, want 8081", savedPort)
	}
	if savedProto != "udp" {
		t.Errorf("save called with proto %q, want 'udp'", savedProto)
	}
}

func TestTUI_EditDialog_EnterSaveError(t *testing.T) {
	ui := &tui{
		sessions:      testSessions(),
		cursor:        0,
		mode:          modeEditDialog,
		inputBuf:      []rune("ok"),
		inputBufPort:  []rune("1234"),
		inputProtocol: "tcp",
		editSaveFunc:  func(string, string, string, string, int) error { return errors.New("collide") },
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.mode != modeEditDialog {
		t.Errorf("mode = %d, want modeEditDialog (stay on save error)", ui.mode)
	}
	if ui.dialogErr != "collide" {
		t.Errorf("dialogErr = %q, want 'collide'", ui.dialogErr)
	}
}

func TestTUI_EditDialog_EnterNoCallback(t *testing.T) {
	ui := &tui{
		sessions:     testSessions(),
		cursor:       0,
		mode:         modeEditDialog,
		inputBuf:     []rune("ok"),
		inputBufPort: []rune(""),
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.mode != modeEditRestartPrompt {
		t.Errorf("mode = %d, want modeEditRestartPrompt even without callback", ui.mode)
	}
}

func TestTUI_EditDialog_EnterOutOfRangeCursor(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 99, mode: modeEditDialog, inputBuf: []rune("x")}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_EditRestartPrompt_Yes(t *testing.T) {
	sessions := testSessions()
	var restarted string
	ui := &tui{
		sessions:    sessions,
		cursor:      0,
		mode:        modeEditRestartPrompt,
		restartFunc: func(id string) error { restarted = id; return nil },
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
	if restarted != sessions[0].UUID {
		t.Errorf("restartFunc called with %q, want %q", restarted, sessions[0].UUID)
	}
}

func TestTUI_EditRestartPrompt_YesError(t *testing.T) {
	ui := &tui{
		sessions:    testSessions(),
		cursor:      0,
		mode:        modeEditRestartPrompt,
		restartFunc: func(string) error { return errors.New("bad") },
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'Y', tcell.ModNone))
	if ui.mode != modeEditRestartPrompt {
		t.Errorf("mode = %d, want modeEditRestartPrompt (stay on error)", ui.mode)
	}
	if ui.dialogErr != "bad" {
		t.Errorf("dialogErr = %q, want 'bad'", ui.dialogErr)
	}
}

func TestTUI_EditRestartPrompt_YesNoCallback(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeEditRestartPrompt}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_EditRestartPrompt_YesOutOfRangeCursor(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 99, mode: modeEditRestartPrompt}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_EditRestartPrompt_No(t *testing.T) {
	ui := &tui{sessions: testSessions(), cursor: 0, mode: modeEditRestartPrompt}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'N', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_EditRestartPrompt_Escape(t *testing.T) {
	ui := &tui{mode: modeEditRestartPrompt, dialogErr: "x"}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.mode != modeMenu || ui.dialogErr != "" {
		t.Errorf("escape did not reset state")
	}
}

func TestTUI_MenuBarIncludesEdit(t *testing.T) {
	screen := newWideTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{Agents: []string{"test-agent"}, IsRunning: func(string) bool { return true }})

	row := getScreenText(screen, testScreenHeight-1, 120)
	if !strings.Contains(row, "(E)dit") {
		t.Errorf("menu bar missing '(E)dit', got: %q", row)
	}
}

func TestTUI_DrawEditDialogs(t *testing.T) {
	screen := newWideTestScreen(t)
	defer screen.Fini()
	ui := &tui{
		screen:       screen,
		sessions:     testSessions(),
		cursor:       0,
		mode:         modeEditDialog,
		inputBuf:     []rune("name"),
		inputBufPort: []rune("80"),
	}
	ui.draw()
	// With error.
	ui.dialogErr = "error text"
	ui.draw()
	// Restart prompt.
	ui.mode = modeEditRestartPrompt
	ui.dialogErr = ""
	ui.draw()
	ui.dialogErr = "retry err"
	ui.draw()
}

func TestTUI_DrawDialogs_TinyScreenClipsDialogs(t *testing.T) {
	// Clipping branches: dialog placed such that y0+dialogHeight > height or
	// x0+dialogWidth > width. We use a small-but-not-too-small screen that still
	// passes the top-level draw check (height>=4, width>=20) and exercises the
	// per-row clipping in each dialog.
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 8)

	modes := []tuiMode{
		modeCreateDialog,
		modeCloneDialog,
		modeDeleteConfirm,
		modeLockedDialog,
		modeOverrideConfirm,
		modeKillConfirm,
		modeNotRunningDialog,
		modeKillInitiated,
		modeBackgroundConfirm,
		modeBackgroundInitiated,
		modeNotConnectedDialog,
		modeSettingsDialog,
		modeRestartConfirm,
		modeRestartInitiated,
		modeAlreadyRunningDialog,
		modeEditDialog,
		modeEditRestartPrompt,
	}
	for _, m := range modes {
		ui := &tui{
			screen:       screen,
			sessions:     testSessions(),
			cursor:       0,
			mode:         m,
			inputBuf:     []rune("name"),
			inputBufPort: []rune("80"),
		}
		ui.draw()
	}
}

func TestTUI_DrawDialogs_WideDialogClippedByWidth(t *testing.T) {
	// Screen narrower than the dialog exercises the x0 < 0 clip branch.
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(25, 30)

	modes := []tuiMode{
		modeCreateDialog,
		modeCloneDialog,
		modeDeleteConfirm,
		modeLockedDialog,
		modeOverrideConfirm,
		modeKillConfirm,
		modeNotRunningDialog,
		modeKillInitiated,
		modeBackgroundConfirm,
		modeBackgroundInitiated,
		modeNotConnectedDialog,
		modeSettingsDialog,
		modeRestartConfirm,
		modeRestartInitiated,
		modeAlreadyRunningDialog,
		modeEditDialog,
		modeEditRestartPrompt,
	}
	for _, m := range modes {
		ui := &tui{
			screen:       screen,
			sessions:     testSessions(),
			cursor:       0,
			mode:         m,
			inputBuf:     []rune("n"),
			inputBufPort: []rune("1"),
			dialogErr:    "e",
		}
		ui.draw()
	}
}

func TestTUI_DrawDialogs_TallDialogClippedByHeight(t *testing.T) {
	// Screens where height < dialogHeight, exercising the y0 < 0 clip branch.
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(120, 5)

	modes := []tuiMode{
		modeCreateDialog,
		modeEditDialog,
		modeEditRestartPrompt,
		modeBackgroundConfirm,
		modeOverrideConfirm,
		modeKillConfirm,
		modeRestartConfirm,
		modeSettingsDialog,
		modeAlreadyRunningDialog,
	}
	for _, m := range modes {
		ui := &tui{
			screen:       screen,
			sessions:     testSessions(),
			cursor:       0,
			mode:         m,
			inputBuf:     []rune("n"),
			inputBufPort: []rune("1"),
			dialogErr:    "e",
		}
		ui.draw()
	}
}

func TestTUI_DrawInputField_TextOverflow(t *testing.T) {
	screen := newWideTestScreen(t)
	defer screen.Fini()
	ui := &tui{screen: screen}
	// Text longer than field width exercises the overflow branch.
	ui.drawInputField(0, 0, 5, []rune("abcdefghij"), true)
	// Also the unfocused path.
	ui.drawInputField(0, 1, 5, []rune("ab"), false)
}

func TestTUI_DrawEditDialogs_BadCursor(t *testing.T) {
	screen := newWideTestScreen(t)
	defer screen.Fini()
	for _, m := range []tuiMode{modeEditDialog, modeEditRestartPrompt} {
		ui := &tui{screen: screen, sessions: nil, cursor: 0, mode: m}
		ui.draw() // early return — no panic
	}
}

// --- wrapErrorText / sizeDialogForError tests ---

func TestWrapErrorText_Empty(t *testing.T) {
	if got := wrapErrorText("", 40); got != nil {
		t.Errorf("wrapErrorText(\"\") = %v, want nil", got)
	}
}

func TestWrapErrorText_NonPositiveWidth(t *testing.T) {
	if got := wrapErrorText("hello", 0); got != nil {
		t.Errorf("width=0 should return nil, got %v", got)
	}
	if got := wrapErrorText("hello", -3); got != nil {
		t.Errorf("negative width should return nil, got %v", got)
	}
}

func TestWrapErrorText_FitsOnOneLine(t *testing.T) {
	got := wrapErrorText("short message", 40)
	if len(got) != 1 || got[0] != "short message" {
		t.Errorf("got %v, want single line", got)
	}
}

func TestWrapErrorText_WordWrap(t *testing.T) {
	got := wrapErrorText("the quick brown fox jumps over the lazy dog", 15)
	// Expect wrapping at word boundaries; every line <= 15 chars.
	joined := strings.Join(got, " ")
	if joined != "the quick brown fox jumps over the lazy dog" {
		t.Errorf("reassembled %q, want original sentence", joined)
	}
	for _, line := range got {
		if len(line) > 15 {
			t.Errorf("line too long (%d > 15): %q", len(line), line)
		}
	}
}

func TestWrapErrorText_HardWrapLongWord(t *testing.T) {
	// A word longer than width is hard-wrapped.
	got := wrapErrorText("abcdefghij", 4)
	if len(got) != 3 {
		t.Fatalf("got %d lines, want 3: %v", len(got), got)
	}
	expected := []string{"abcd", "efgh", "ij"}
	for i, want := range expected {
		if got[i] != want {
			t.Errorf("line %d = %q, want %q", i, got[i], want)
		}
	}
}

func TestWrapErrorText_HardWrapMidSentence(t *testing.T) {
	// Long word in the middle should flush the current line before hard-wrap.
	got := wrapErrorText("hi abcdefghij world", 4)
	// "hi" flushes, then "abcdefghij" hard-wraps as "abcd"/"efgh"/"ij", then "world" hard-wraps.
	if len(got) < 4 {
		t.Fatalf("expected at least 4 lines, got %d: %v", len(got), got)
	}
	if got[0] != "hi" {
		t.Errorf("line 0 = %q, want 'hi'", got[0])
	}
}

func TestWrapErrorText_TruncatesAt500Chars(t *testing.T) {
	long := strings.Repeat("a", 600)
	got := wrapErrorText(long, 80)
	total := 0
	for _, line := range got {
		total += len(line)
	}
	// The input is truncated to 500 chars with "..." replacing the last 3 chars,
	// so wrapped output reconstructs to exactly 500 chars of content.
	if total != 500 {
		t.Errorf("total chars = %d, want 500 after truncation", total)
	}
	reassembled := strings.Join(got, "")
	if !strings.HasSuffix(reassembled, "...") {
		t.Errorf("truncated text should end with '...', got: %q", reassembled[len(reassembled)-10:])
	}
}

func TestSizeDialogForError_NoError(t *testing.T) {
	// Natural width >= min, no error → use natural.
	if got := sizeDialogForError(60, 40, "", 120); got != 60 {
		t.Errorf("got %d, want 60", got)
	}
	// Natural < min → use min.
	if got := sizeDialogForError(30, 40, "", 120); got != 40 {
		t.Errorf("got %d, want 40 (min)", got)
	}
}

func TestSizeDialogForError_WithError(t *testing.T) {
	// With error, widen to errorDialogMinWidth even if natural is smaller.
	got := sizeDialogForError(40, 40, "boom", 120)
	if got != errorDialogMinWidth {
		t.Errorf("got %d, want %d (errorDialogMinWidth)", got, errorDialogMinWidth)
	}
}

func TestSizeDialogForError_CapsToScreenWidth(t *testing.T) {
	// Screen narrower than preferred width: cap.
	got := sizeDialogForError(80, 40, "err", 50)
	if got != 48 {
		t.Errorf("got %d, want 48 (screenWidth-2)", got)
	}
}

func TestSizeDialogForError_ScreenTooNarrowHoldsMin(t *testing.T) {
	// Screen even narrower than min: refuse to go below min (so layout is
	// predictable even when the user drastically shrinks the terminal).
	got := sizeDialogForError(60, 40, "err", 30)
	if got != 40 {
		t.Errorf("got %d, want %d (min held)", got, 40)
	}
}

// --- Integration: restart dialog shows full multi-line error ---

func TestTUI_RestartDialog_LongErrorWraps(t *testing.T) {
	screen := newWideTestScreen(t)
	defer screen.Fini()

	longErr := "docker: Error response from daemon: driver failed programming external connectivity on endpoint convocate-session-xxx: Bind for 0.0.0.0:53 failed: port is already allocated."
	ui := &tui{
		screen:    screen,
		sessions:  testSessions(),
		cursor:    0,
		mode:      modeRestartConfirm,
		dialogErr: longErr,
	}
	ui.draw()

	// Search every row of the screen and normalize runs of whitespace so that
	// wrapped phrases reassemble across line breaks.
	w, h := screen.Size()
	var all strings.Builder
	for row := 0; row < h; row++ {
		all.WriteString(getScreenText(screen, row, w))
		all.WriteByte(' ')
	}
	joined := strings.Join(strings.Fields(all.String()), " ")
	if !strings.Contains(joined, "driver failed programming external connectivity") {
		t.Errorf("full error not rendered across wrapped lines; searched:\n%s", joined)
	}
	if !strings.Contains(joined, "port is already allocated") {
		t.Errorf("trailing part of error missing; searched:\n%s", joined)
	}
}

func TestTUI_RestartDialog_Truncates500(t *testing.T) {
	screen := newWideTestScreen(t)
	defer screen.Fini()
	ui := &tui{
		screen:    screen,
		sessions:  testSessions(),
		cursor:    0,
		mode:      modeRestartConfirm,
		dialogErr: strings.Repeat("a", 800),
	}
	ui.draw()
	w, h := screen.Size()
	var all strings.Builder
	for row := 0; row < h; row++ {
		all.WriteString(getScreenText(screen, row, w))
	}
	if !strings.Contains(all.String(), "...") {
		t.Error("expected ellipsis after 500-char truncation")
	}
}

// --- clipToWidth tests ---

func TestClipToWidth(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"this is too long for width", 10, "this is to"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		got := clipToWidth(tt.input, tt.width)
		if got != tt.expected {
			t.Errorf("clipToWidth(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.expected)
		}
	}
}

// --- agent picker (modeSelectAgent / modeNoAgents) ---

func TestTUI_PressN_NoAgents_ShowsNotice(t *testing.T) {
	ui := &tui{sessions: testSessions(), mode: modeMenu, agents: nil}
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	if done {
		t.Error("N with no agents should not quit the TUI")
	}
	if ui.mode != modeNoAgents {
		t.Errorf("mode = %d, want modeNoAgents", ui.mode)
	}
}

func TestTUI_PressN_SingleAgent_ShortcutsToCreate(t *testing.T) {
	ui := &tui{sessions: testSessions(), mode: modeMenu, agents: []string{"solo"}}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	if ui.mode != modeCreateDialog {
		t.Errorf("mode = %d, want modeCreateDialog (single-agent shortcut)", ui.mode)
	}
	if ui.chosenAgent != "solo" {
		t.Errorf("chosenAgent = %q, want 'solo'", ui.chosenAgent)
	}
}

func TestTUI_PressN_MultipleAgents_OpensPicker(t *testing.T) {
	ui := &tui{sessions: testSessions(), mode: modeMenu, agents: []string{"a", "b", "c"}}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	if ui.mode != modeSelectAgent {
		t.Errorf("mode = %d, want modeSelectAgent", ui.mode)
	}
	if ui.selectedAgent != 0 {
		t.Errorf("selectedAgent = %d, want 0 (starts at top)", ui.selectedAgent)
	}
}

func TestTUI_AgentPicker_DownEnter_SelectsAgentAndOpensCreate(t *testing.T) {
	ui := &tui{mode: modeSelectAgent, agents: []string{"a", "b", "c"}, selectedAgent: 0}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if ui.selectedAgent != 1 {
		t.Errorf("selectedAgent after down = %d, want 1", ui.selectedAgent)
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.chosenAgent != "b" {
		t.Errorf("chosenAgent = %q, want 'b'", ui.chosenAgent)
	}
	if ui.mode != modeCreateDialog {
		t.Errorf("mode = %d, want modeCreateDialog", ui.mode)
	}
}

func TestTUI_AgentPicker_EscCancelsToMenu(t *testing.T) {
	ui := &tui{mode: modeSelectAgent, agents: []string{"a", "b"}}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_NoAgentsDialog_AnyKeyDismisses(t *testing.T) {
	ui := &tui{mode: modeNoAgents}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("mode = %d, want modeMenu", ui.mode)
	}
}

func TestTUI_CreateDialog_CarriesAgentIntoSelection(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'n', tcell.ModNone)
		time.Sleep(10 * time.Millisecond)
		for _, ch := range "remote-proj" {
			screen.InjectKey(tcell.KeyRune, ch, tcell.ModNone)
			time.Sleep(3 * time.Millisecond)
		}
		time.Sleep(10 * time.Millisecond)
		screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{
		Agents: []string{"agent-alpha"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sel.AgentID != "agent-alpha" {
		t.Errorf("Selection.AgentID = %q, want 'agent-alpha'", sel.AgentID)
	}
	if sel.Name != "remote-proj" {
		t.Errorf("Selection.Name = %q", sel.Name)
	}
}

// fullScreenText concatenates every row of the simulation screen so tests
// can assert against multi-line dialog content with one Contains check.
func fullScreenText(screen tcell.SimulationScreen) string {
	w, h := screen.Size()
	var sb strings.Builder
	for row := 0; row < h; row++ {
		sb.WriteString(getScreenText(screen, row, w))
		sb.WriteByte('\n')
	}
	return sb.String()
}

func TestTUI_DrawSelectAgentDialog(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	ui := &tui{
		screen:        screen,
		agents:        []string{"alpha-agent", "beta-agent", "gamma-agent"},
		selectedAgent: 1,
	}
	ui.drawSelectAgentDialog(testScreenWidth, testScreenHeight)
	screen.Show()

	body := fullScreenText(screen)
	for _, want := range []string{
		"Select Agent for New Session",
		"alpha-agent",
		"> beta-agent", // highlighted row
		"gamma-agent",
		"Up/Down=Move",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("dialog missing %q\n%s", want, body)
		}
	}
}

func TestTUI_DrawNoAgentsDialog(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	ui := &tui{screen: screen}
	ui.drawNoAgentsDialog(testScreenWidth, testScreenHeight)
	screen.Show()

	body := fullScreenText(screen)
	for _, want := range []string{
		"No Agents",
		"No convocate-agent hosts are registered",
		"convocate-host init-agent",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("dialog missing %q\n%s", want, body)
		}
	}
}

func TestTUI_DrawNoAgentsDialog_NarrowScreen(t *testing.T) {
	// Confirm centering math doesn't index negative on a tiny screen.
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(20, 5)

	ui := &tui{screen: screen}
	ui.drawNoAgentsDialog(20, 5) // intentionally too narrow
	screen.Show()
	// Just assert it didn't panic and at least one rune of the title is
	// somewhere on the screen.
	body := fullScreenText(screen)
	if !strings.Contains(body, "N") {
		t.Errorf("expected some rendering on narrow screen, got %q", body)
	}
}

func TestTUI_SelectAgentKey_Navigation(t *testing.T) {
	// Drive the modeSelectAgent key handler directly so we can assert
	// Up/Down clamp at the boundaries and Enter commits.
	ui := &tui{agents: []string{"a", "b", "c"}, selectedAgent: 0}
	for _, k := range []tcell.Key{tcell.KeyDown, tcell.KeyDown, tcell.KeyDown /* clamp */} {
		ui.handleSelectAgentKey(tcell.NewEventKey(k, 0, tcell.ModNone))
	}
	if ui.selectedAgent != 2 {
		t.Errorf("after 3 Downs, selectedAgent = %d, want 2 (clamped)", ui.selectedAgent)
	}
	for _, k := range []tcell.Key{tcell.KeyUp, tcell.KeyUp, tcell.KeyUp /* clamp */} {
		ui.handleSelectAgentKey(tcell.NewEventKey(k, 0, tcell.ModNone))
	}
	if ui.selectedAgent != 0 {
		t.Errorf("after 3 Ups, selectedAgent = %d, want 0 (clamped)", ui.selectedAgent)
	}
}

func TestTUI_SelectAgentKey_EscapeReturnsToMenu(t *testing.T) {
	ui := &tui{agents: []string{"a"}, mode: modeSelectAgent}
	ui.handleSelectAgentKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("Escape should return to menu, got mode %d", ui.mode)
	}
}

func TestTUI_SelectAgentKey_EnterEmptyAgentsReturnsToMenu(t *testing.T) {
	// Defensive: if the dialog is somehow shown with an empty agent list
	// and Enter is pressed, the handler should bail rather than panic.
	ui := &tui{agents: nil, mode: modeSelectAgent}
	ui.handleSelectAgentKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.mode != modeMenu {
		t.Errorf("Enter on empty agents should bounce to menu, got mode %d", ui.mode)
	}
}
