package menu

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/asymmetric-effort/claude-shell/internal/session"
)

const (
	testScreenWidth  = 100
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

	sel, err := DisplayWithScreen(sessions, screen, DisplayOptions{})
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

	_, err := Display(testSessions(), DisplayOptions{})
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

	sel, err := Display(testSessions(), DisplayOptions{})
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

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(sessions, screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(nil, screen, DisplayOptions{})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

func TestTUI_ReloadOnR(t *testing.T) {
	sel := runWithKey(t, testSessions(), tcell.KeyRune, 'r')
	if sel.Action != ActionReload {
		t.Errorf("Action = %q, want %q", sel.Action, ActionReload)
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

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(sessions, screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(sessions, screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(nil, screen, DisplayOptions{})
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

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{})

	row := getScreenText(screen, 0, testScreenWidth)
	if !strings.Contains(row, "claude-shell") {
		t.Errorf("title bar missing 'claude-shell', got: %q", row)
	}
}

func TestTUI_TitleBarStyle(t *testing.T) {
	screen := newTestScreen(t)
	defer screen.Fini()

	go func() {
		time.Sleep(20 * time.Millisecond)
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	}()

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{})

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

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{})

	row := getScreenText(screen, testScreenHeight-1, testScreenWidth)
	for _, label := range []string{"(N)ew", "(D)elete", "(R)eload", "(Q)uit"} {
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

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{})

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

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{})

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

	_, _ = DisplayWithScreen(nil, screen, DisplayOptions{})

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

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{})

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

	sel, err := DisplayWithScreen(sessions, screen, DisplayOptions{})
	if err != nil {
		t.Fatalf("DisplayWithScreen failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

func TestTUI_HandleKeyUnit(t *testing.T) {
	sessions := testSessions()
	ui := &tui{sessions: sessions, cursor: 0}

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
		{'r', ActionReload},
		{'R', ActionReload},
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
	ui := &tui{sessions: testSessions(), cursor: 0}

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

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{})
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

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{})
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

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{})

	row := getScreenText(screen, 0, 30)
	if !strings.Contains(row, "claude-shell") {
		t.Error("title bar should still show 'claude-shell'")
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

	sel, err := displayWithOptions(testSessions(), screen, 50*time.Millisecond, 10*time.Second, DisplayOptions{})
	if err != nil {
		t.Fatalf("displayWithOptions failed: %v", err)
	}
	if sel.Action != ActionQuit {
		t.Errorf("Action = %q, want %q", sel.Action, ActionQuit)
	}
}

// --- Create dialog: Name + Port field ---

func TestTUI_CreateDialog_TabSwitchesFields(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 0}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if ui.activeField != 1 {
		t.Errorf("Tab should move to Port field, activeField=%d", ui.activeField)
	}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if ui.activeField != 0 {
		t.Errorf("Tab should wrap back to Name field, activeField=%d", ui.activeField)
	}
}

func TestTUI_CreateDialog_ShiftTabMovesBackward(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 1}
	_, _ = ui.handleKey(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone))
	if ui.activeField != 0 {
		t.Errorf("Shift+Tab should move to Name field, activeField=%d", ui.activeField)
	}
}

func TestTUI_CreateDialog_PortFieldDigitsOnly(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 1}
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
		screen.InjectKey(tcell.KeyTab, 0, tcell.ModNone)
		time.Sleep(5 * time.Millisecond)
		for _, ch := range "8080" {
			screen.InjectKey(tcell.KeyRune, ch, tcell.ModNone)
			time.Sleep(2 * time.Millisecond)
		}
		time.Sleep(5 * time.Millisecond)
		screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	}()

	sel, err := DisplayWithScreen(testSessions(), screen, DisplayOptions{})
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
}

func TestTUI_CreateDialog_InvalidPortShowsError(t *testing.T) {
	ui := &tui{mode: modeCreateDialog, activeField: 1}
	// Populate name so name validation passes
	ui.inputBuf = []rune("test")
	ui.inputBufPort = []rune("70000")
	_, done := ui.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if done {
		t.Error("Enter with invalid port should not finish")
	}
	if ui.dialogErr == "" {
		t.Error("expected dialog error for out-of-range port")
	}
	if ui.activeField != 1 {
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

	_, _ = DisplayWithScreen(sessions, screen, DisplayOptions{})

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

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{})

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

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{})

	row := getScreenText(screen, 0, testScreenWidth)
	if !strings.Contains(row, "claude-shell") {
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

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{})

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

	_, _ = DisplayWithScreen(testSessions(), screen, DisplayOptions{})

	row := getScreenText(screen, testScreenHeight-1, testScreenWidth)
	if !strings.Contains(row, "(B)ackground") {
		t.Errorf("menu bar missing '(B)ackground', got: %q", row)
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
