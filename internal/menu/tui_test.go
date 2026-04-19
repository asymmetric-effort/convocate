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
	if fg != tcell.ColorWhite {
		t.Errorf("menu bar fg = %v, want White", fg)
	}
	if bg != tcell.ColorBlack {
		t.Errorf("menu bar bg = %v, want Black", bg)
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
