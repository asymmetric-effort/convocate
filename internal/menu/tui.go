package menu

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/asymmetric-effort/claude-shell/internal/session"
)

var (
	titleStyle     = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlue)
	menuBarStyle   = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
	headerStyle    = tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
	normalStyle    = tcell.StyleDefault
	selectedStyle  = tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorWhite)
	separatorStyle = tcell.StyleDefault.Foreground(tcell.ColorGray)
	dialogStyle    = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorDarkBlue)
	dialogErrStyle = tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorDarkBlue)
	dialogWarnStyle = tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.ColorRed)
	inputStyle     = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
)

type tuiMode int

const (
	modeMenu tuiMode = iota
	modeCreateDialog
	modeCloneDialog
	modeDeleteConfirm
	modeLockedDialog
	modeOverrideConfirm
	modeKillConfirm
	modeNotRunningDialog
	modeKillInitiated
)

type tui struct {
	screen         tcell.Screen
	sessions       []session.Metadata
	cursor         int
	offset         int
	tickInterval   time.Duration
	reloadInterval time.Duration
	mode           tuiMode
	inputBuf       []rune
	dialogErr      string
	isLockedFunc     func(id string) bool
	isRunningFunc    func(id string) bool
	reloadFunc       func() ([]session.Metadata, error)
	overrideLockFunc func(id string) error
	killFunc         func(id string) error
}

func (t *tui) isLocked(id string) bool {
	if t.isLockedFunc == nil {
		return false
	}
	return t.isLockedFunc(id)
}

func (t *tui) isRunning(id string) bool {
	if t.isRunningFunc == nil {
		return false
	}
	return t.isRunningFunc(id)
}

// screenFactory creates and initializes a tcell.Screen. Override in tests.
var screenFactory func() (tcell.Screen, error) = func() (tcell.Screen, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, fmt.Errorf("failed to create screen: %w", err)
	}
	if err := screen.Init(); err != nil {
		return nil, fmt.Errorf("failed to init screen: %w", err)
	}
	return screen, nil
}

// DisplayOptions configures the TUI display.
type DisplayOptions struct {
	IsLocked     func(string) bool
	IsRunning    func(string) bool
	Reload       func() ([]session.Metadata, error)
	OverrideLock func(string) error
	KillSession  func(string) error
}

// Display renders the TUI session menu and returns the user's selection.
func Display(sessions []session.Metadata, opts DisplayOptions) (Selection, error) {
	screen, err := screenFactory()
	if err != nil {
		return Selection{}, err
	}
	defer screen.Fini()

	return DisplayWithScreen(sessions, screen, opts)
}

// DisplayWithScreen renders the TUI on a provided screen (for testing).
func DisplayWithScreen(sessions []session.Metadata, screen tcell.Screen, opts DisplayOptions) (Selection, error) {
	return displayWithOptions(sessions, screen, 1*time.Second, 10*time.Second, opts)
}

func displayWithOptions(sessions []session.Metadata, screen tcell.Screen, tickInterval, reloadInterval time.Duration, opts DisplayOptions) (Selection, error) {
	done := make(chan struct{})
	defer close(done)

	isLocked := opts.IsLocked
	if isLocked == nil {
		isLocked = func(string) bool { return false }
	}

	isRunning := opts.IsRunning
	if isRunning == nil {
		isRunning = func(string) bool { return false }
	}

	t := &tui{
		screen:           screen,
		sessions:         sessions,
		tickInterval:     tickInterval,
		reloadInterval:   reloadInterval,
		isLockedFunc:     isLocked,
		isRunningFunc:    isRunning,
		reloadFunc:       opts.Reload,
		overrideLockFunc: opts.OverrideLock,
		killFunc:         opts.KillSession,
	}

	// Tick periodically to keep the clock updated
	go func() {
		ticker := time.NewTicker(t.tickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				screen.PostEvent(tcell.NewEventInterrupt(nil))
			}
		}
	}()

	// Reload sessions periodically
	if t.reloadFunc != nil {
		go func() {
			ticker := time.NewTicker(t.reloadInterval)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					reloadFlag := true
					screen.PostEvent(tcell.NewEventInterrupt(reloadFlag))
				}
			}
		}()
	}

	return t.run()
}

func (t *tui) run() (Selection, error) {
	for {
		t.draw()
		t.screen.Show()

		ev := t.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			sel, done := t.handleKey(ev)
			if done {
				return sel, nil
			}
		case *tcell.EventResize:
			t.screen.Sync()
		case *tcell.EventInterrupt:
			if _, ok := ev.Data().(bool); ok && t.reloadFunc != nil {
				if sessions, err := t.reloadFunc(); err == nil {
					t.sessions = sessions
					if t.cursor >= len(t.sessions) {
						t.cursor = len(t.sessions) - 1
					}
					if t.cursor < 0 {
						t.cursor = 0
					}
				}
			}
		}
	}
}

func (t *tui) draw() {
	t.screen.Clear()
	width, height := t.screen.Size()
	if height < 4 || width < 20 {
		return
	}

	t.drawTitleBar(width)
	t.drawSessionTable(width, height)
	t.drawMenuBar(width, height)

	switch t.mode {
	case modeCreateDialog:
		t.drawCreateDialog(width, height)
	case modeCloneDialog:
		t.drawCloneDialog(width, height)
	case modeDeleteConfirm:
		t.drawDeleteDialog(width, height)
	case modeLockedDialog:
		t.drawLockedDialog(width, height)
	case modeOverrideConfirm:
		t.drawOverrideDialog(width, height)
	case modeKillConfirm:
		t.drawKillDialog(width, height)
	case modeNotRunningDialog:
		t.drawNotRunningDialog(width, height)
	case modeKillInitiated:
		t.drawKillInitiatedDialog(width, height)
	}
}

func (t *tui) drawTitleBar(width int) {
	fillRow(t.screen, 0, width, titleStyle)
	drawString(t.screen, 1, 0, "claude-shell", titleStyle)

	clock := time.Now().Format("2006-01-02 15:04:05")
	x := width - len(clock) - 1
	if x > 13 {
		drawString(t.screen, x, 0, clock, titleStyle)
	}
}

func (t *tui) drawSessionTable(width, height int) {
	headerRow := 2
	sepRow := 3
	sessionsStart := 4
	menuBarRow := height - 1

	// Column header
	header := fmt.Sprintf("  %-4s| %-20s | %-36s | %-12s | %-13s | %s",
		"#", "Name", "Session ID", "Created", "Last Accessed", "S")
	drawString(t.screen, 0, headerRow, clipToWidth(header, width), headerStyle)

	// Separator
	drawString(t.screen, 0, sepRow, clipToWidth(strings.Repeat("\u2500", width), width), separatorStyle)

	visibleRows := menuBarRow - sessionsStart
	if visibleRows < 1 {
		return
	}

	// Clamp cursor
	if len(t.sessions) > 0 {
		if t.cursor >= len(t.sessions) {
			t.cursor = len(t.sessions) - 1
		}
		if t.cursor < 0 {
			t.cursor = 0
		}
	}

	// Adjust scroll offset to keep cursor visible
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+visibleRows {
		t.offset = t.cursor - visibleRows + 1
	}

	if len(t.sessions) == 0 {
		drawString(t.screen, 2, sessionsStart, "No sessions. Press (N) to create one.", normalStyle)
		return
	}

	for i := 0; i < visibleRows && i+t.offset < len(t.sessions); i++ {
		idx := i + t.offset
		s := t.sessions[idx]

		style := normalStyle
		if idx == t.cursor {
			style = selectedStyle
		}

		statusIndicator := "-"
		if t.isRunning(s.UUID) {
			statusIndicator = "R"
		} else if t.isLocked(s.UUID) {
			statusIndicator = "L"
		}

		line := fmt.Sprintf("  %-4s| %-20s | %s | %-12s | %-13s | %s",
			strconv.Itoa(idx+1),
			truncate(s.Name, 20),
			s.UUID,
			s.CreatedAt.Format("2006-01-02"),
			s.LastAccessed.Format("2006-01-02"),
			statusIndicator,
		)

		row := sessionsStart + i
		fillRow(t.screen, row, width, style)
		drawString(t.screen, 0, row, clipToWidth(line, width), style)
	}
}

func (t *tui) drawMenuBar(width, height int) {
	row := height - 1
	fillRow(t.screen, row, width, menuBarStyle)
	drawString(t.screen, 1, row,
		"(N)ew | (C)lone | (D)elete | (K)ill | (O)verride lock | (R)eload | (Q)uit",
		menuBarStyle)
}

func (t *tui) drawCreateDialog(width, height int) {
	const dialogWidth = 52
	const dialogHeight = 7

	x0 := (width - dialogWidth) / 2
	y0 := (height - dialogHeight) / 2

	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	// Draw dialog background
	for row := y0; row < y0+dialogHeight && row < height; row++ {
		for col := x0; col < x0+dialogWidth && col < width; col++ {
			t.screen.SetContent(col, row, ' ', nil, dialogStyle)
		}
	}

	// Title
	title := " Create New Session "
	drawString(t.screen, x0+(dialogWidth-len(title))/2, y0, title, dialogStyle.Bold(true))

	// Label
	drawString(t.screen, x0+2, y0+2, "Name:", dialogStyle)

	// Input field background
	inputX := x0 + 8
	inputW := dialogWidth - 10
	for col := inputX; col < inputX+inputW; col++ {
		t.screen.SetContent(col, y0+2, ' ', nil, inputStyle)
	}

	// Input text
	inputText := string(t.inputBuf)
	if len(inputText) > inputW {
		inputText = inputText[len(inputText)-inputW:]
	}
	drawString(t.screen, inputX, y0+2, inputText, inputStyle)

	// Cursor
	cursorX := inputX + len(t.inputBuf)
	if len(t.inputBuf) > inputW {
		cursorX = inputX + inputW
	}
	if cursorX < x0+dialogWidth-2 {
		t.screen.SetContent(cursorX, y0+2, ' ', nil, inputStyle.Reverse(true))
	}

	// Error message
	if t.dialogErr != "" {
		errMsg := clipToWidth(t.dialogErr, dialogWidth-4)
		drawString(t.screen, x0+2, y0+4, errMsg, dialogErrStyle)
	}

	// Hint
	hint := "Enter=Create  Esc=Cancel"
	drawString(t.screen, x0+(dialogWidth-len(hint))/2, y0+dialogHeight-1, hint, dialogStyle)
}

func (t *tui) drawCloneDialog(width, height int) {
	if t.cursor < 0 || t.cursor >= len(t.sessions) {
		return
	}
	s := t.sessions[t.cursor]

	const dialogWidth = 60
	const dialogHeight = 8

	x0 := (width - dialogWidth) / 2
	y0 := (height - dialogHeight) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	for row := y0; row < y0+dialogHeight && row < height; row++ {
		for col := x0; col < x0+dialogWidth && col < width; col++ {
			t.screen.SetContent(col, row, ' ', nil, dialogStyle)
		}
	}

	title := " Clone Session "
	drawString(t.screen, x0+(dialogWidth-len(title))/2, y0, title, dialogStyle.Bold(true))

	source := fmt.Sprintf("From: %s", truncate(s.Name, dialogWidth-10))
	drawString(t.screen, x0+2, y0+2, source, dialogStyle)

	drawString(t.screen, x0+2, y0+3, "Name:", dialogStyle)

	inputX := x0 + 8
	inputW := dialogWidth - 10
	for col := inputX; col < inputX+inputW; col++ {
		t.screen.SetContent(col, y0+3, ' ', nil, inputStyle)
	}

	inputText := string(t.inputBuf)
	if len(inputText) > inputW {
		inputText = inputText[len(inputText)-inputW:]
	}
	drawString(t.screen, inputX, y0+3, inputText, inputStyle)

	cursorX := inputX + len(t.inputBuf)
	if len(t.inputBuf) > inputW {
		cursorX = inputX + inputW
	}
	if cursorX < x0+dialogWidth-2 {
		t.screen.SetContent(cursorX, y0+3, ' ', nil, inputStyle.Reverse(true))
	}

	if t.dialogErr != "" {
		errMsg := clipToWidth(t.dialogErr, dialogWidth-4)
		drawString(t.screen, x0+2, y0+5, errMsg, dialogErrStyle)
	}

	hint := "Enter=Clone  Esc=Cancel"
	drawString(t.screen, x0+(dialogWidth-len(hint))/2, y0+dialogHeight-1, hint, dialogStyle)
}

func (t *tui) drawDeleteDialog(width, height int) {
	if t.cursor < 0 || t.cursor >= len(t.sessions) {
		return
	}
	s := t.sessions[t.cursor]

	name := truncate(s.Name, 30)
	prompt := fmt.Sprintf("Delete session %q?", name)
	dialogWidth := len(prompt) + 6
	if dialogWidth < 36 {
		dialogWidth = 36
	}
	const dialogHeight = 5

	x0 := (width - dialogWidth) / 2
	y0 := (height - dialogHeight) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	// Draw dialog background
	for row := y0; row < y0+dialogHeight && row < height; row++ {
		for col := x0; col < x0+dialogWidth && col < width; col++ {
			t.screen.SetContent(col, row, ' ', nil, dialogStyle)
		}
	}

	// Title
	title := " Confirm Delete "
	drawString(t.screen, x0+(dialogWidth-len(title))/2, y0, title, dialogStyle.Bold(true))

	// Prompt
	drawString(t.screen, x0+2, y0+2, prompt, dialogStyle)

	// Hint
	hint := "(Y)es  (N)o"
	drawString(t.screen, x0+(dialogWidth-len(hint))/2, y0+dialogHeight-1, hint, dialogStyle)
}

func (t *tui) drawLockedDialog(width, height int) {
	if t.cursor < 0 || t.cursor >= len(t.sessions) {
		return
	}
	s := t.sessions[t.cursor]

	name := truncate(s.Name, 30)
	msg := fmt.Sprintf("Session %q is currently locked.", name)
	dialogWidth := len(msg) + 6
	if dialogWidth < 40 {
		dialogWidth = 40
	}
	const dialogHeight = 5

	x0 := (width - dialogWidth) / 2
	y0 := (height - dialogHeight) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	// Draw dialog background
	for row := y0; row < y0+dialogHeight && row < height; row++ {
		for col := x0; col < x0+dialogWidth && col < width; col++ {
			t.screen.SetContent(col, row, ' ', nil, dialogStyle)
		}
	}

	// Title
	title := " Session Locked "
	drawString(t.screen, x0+(dialogWidth-len(title))/2, y0, title, dialogStyle.Bold(true))

	// Message
	drawString(t.screen, x0+2, y0+2, msg, dialogStyle)

	// Hint
	hint := "Press any key to continue"
	drawString(t.screen, x0+(dialogWidth-len(hint))/2, y0+dialogHeight-1, hint, dialogStyle)
}

func (t *tui) handleLockedDialogKey(ev *tcell.EventKey) (Selection, bool) {
	t.mode = modeMenu
	return Selection{}, false
}

func (t *tui) drawNotRunningDialog(width, height int) {
	if t.cursor < 0 || t.cursor >= len(t.sessions) {
		return
	}
	s := t.sessions[t.cursor]

	name := truncate(s.Name, 30)
	msg := fmt.Sprintf("Session %q is not running.", name)
	dialogWidth := len(msg) + 6
	if dialogWidth < 40 {
		dialogWidth = 40
	}
	const dialogHeight = 5

	x0 := (width - dialogWidth) / 2
	y0 := (height - dialogHeight) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	// Draw dialog background (red)
	for row := y0; row < y0+dialogHeight && row < height; row++ {
		for col := x0; col < x0+dialogWidth && col < width; col++ {
			t.screen.SetContent(col, row, ' ', nil, dialogWarnStyle)
		}
	}

	// Title
	title := " Not Running "
	drawString(t.screen, x0+(dialogWidth-len(title))/2, y0, title, dialogWarnStyle.Bold(true))

	// Message
	drawString(t.screen, x0+2, y0+2, msg, dialogWarnStyle)

	// Hint
	hint := "Press any key to continue"
	drawString(t.screen, x0+(dialogWidth-len(hint))/2, y0+dialogHeight-1, hint, dialogWarnStyle)
}

func (t *tui) handleNotRunningDialogKey(ev *tcell.EventKey) (Selection, bool) {
	t.mode = modeMenu
	return Selection{}, false
}

func (t *tui) drawKillInitiatedDialog(width, height int) {
	if t.cursor < 0 || t.cursor >= len(t.sessions) {
		return
	}
	s := t.sessions[t.cursor]

	name := truncate(s.Name, 30)
	msg := fmt.Sprintf("Kill initiated for session %q.", name)
	dialogWidth := len(msg) + 6
	if dialogWidth < 40 {
		dialogWidth = 40
	}
	const dialogHeight = 5

	x0 := (width - dialogWidth) / 2
	y0 := (height - dialogHeight) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	// Draw dialog background
	for row := y0; row < y0+dialogHeight && row < height; row++ {
		for col := x0; col < x0+dialogWidth && col < width; col++ {
			t.screen.SetContent(col, row, ' ', nil, dialogStyle)
		}
	}

	// Title
	title := " Kill Initiated "
	drawString(t.screen, x0+(dialogWidth-len(title))/2, y0, title, dialogStyle.Bold(true))

	// Message
	drawString(t.screen, x0+2, y0+2, msg, dialogStyle)

	// Hint
	hint := "Press any key to continue"
	drawString(t.screen, x0+(dialogWidth-len(hint))/2, y0+dialogHeight-1, hint, dialogStyle)
}

func (t *tui) handleKillInitiatedDialogKey(ev *tcell.EventKey) (Selection, bool) {
	t.mode = modeMenu
	return Selection{}, false
}

func (t *tui) drawOverrideDialog(width, height int) {
	if t.cursor < 0 || t.cursor >= len(t.sessions) {
		return
	}
	s := t.sessions[t.cursor]

	name := truncate(s.Name, 30)
	prompt := fmt.Sprintf("Override lock for session %q?", name)
	dialogWidth := len(prompt) + 6
	if dialogWidth < 44 {
		dialogWidth = 44
	}
	dialogHeight := 5
	if t.dialogErr != "" {
		dialogHeight = 7
	}

	x0 := (width - dialogWidth) / 2
	y0 := (height - dialogHeight) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	// Draw dialog background
	for row := y0; row < y0+dialogHeight && row < height; row++ {
		for col := x0; col < x0+dialogWidth && col < width; col++ {
			t.screen.SetContent(col, row, ' ', nil, dialogStyle)
		}
	}

	// Title
	title := " Override Lock "
	drawString(t.screen, x0+(dialogWidth-len(title))/2, y0, title, dialogStyle.Bold(true))

	// Prompt
	drawString(t.screen, x0+2, y0+2, prompt, dialogStyle)

	// Error message
	if t.dialogErr != "" {
		errMsg := clipToWidth(t.dialogErr, dialogWidth-4)
		drawString(t.screen, x0+2, y0+4, errMsg, dialogErrStyle)
	}

	// Hint
	hint := "(Y)es  (N)o"
	drawString(t.screen, x0+(dialogWidth-len(hint))/2, y0+dialogHeight-1, hint, dialogStyle)
}

func (t *tui) handleOverrideDialogKey(ev *tcell.EventKey) (Selection, bool) {
	switch ev.Key() {
	case tcell.KeyEscape:
		t.mode = modeMenu
		t.dialogErr = ""
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'y', 'Y':
			if t.cursor >= 0 && t.cursor < len(t.sessions) && t.overrideLockFunc != nil {
				s := t.sessions[t.cursor]
				if err := t.overrideLockFunc(s.UUID); err != nil {
					t.dialogErr = err.Error()
					return Selection{}, false
				}
				t.mode = modeMenu
				t.dialogErr = ""
			} else {
				t.mode = modeMenu
			}
		case 'n', 'N':
			t.mode = modeMenu
			t.dialogErr = ""
		}
	}
	return Selection{}, false
}

func (t *tui) drawKillDialog(width, height int) {
	if t.cursor < 0 || t.cursor >= len(t.sessions) {
		return
	}
	s := t.sessions[t.cursor]

	name := truncate(s.Name, 30)
	prompt := fmt.Sprintf("Kill session %q?", name)
	dialogWidth := len(prompt) + 6
	if dialogWidth < 40 {
		dialogWidth = 40
	}
	dialogHeight := 5
	if t.dialogErr != "" {
		dialogHeight = 7
	}

	x0 := (width - dialogWidth) / 2
	y0 := (height - dialogHeight) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	// Draw dialog background
	for row := y0; row < y0+dialogHeight && row < height; row++ {
		for col := x0; col < x0+dialogWidth && col < width; col++ {
			t.screen.SetContent(col, row, ' ', nil, dialogStyle)
		}
	}

	// Title
	title := " Kill Session "
	drawString(t.screen, x0+(dialogWidth-len(title))/2, y0, title, dialogStyle.Bold(true))

	// Prompt
	drawString(t.screen, x0+2, y0+2, prompt, dialogStyle)

	// Error message
	if t.dialogErr != "" {
		errMsg := clipToWidth(t.dialogErr, dialogWidth-4)
		drawString(t.screen, x0+2, y0+4, errMsg, dialogErrStyle)
	}

	// Hint
	hint := "(Y)es  (N)o"
	drawString(t.screen, x0+(dialogWidth-len(hint))/2, y0+dialogHeight-1, hint, dialogStyle)
}

func (t *tui) handleKillDialogKey(ev *tcell.EventKey) (Selection, bool) {
	switch ev.Key() {
	case tcell.KeyEscape:
		t.mode = modeMenu
		t.dialogErr = ""
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'y', 'Y':
			if t.cursor >= 0 && t.cursor < len(t.sessions) {
				s := t.sessions[t.cursor]
				if !t.isRunning(s.UUID) {
					t.mode = modeNotRunningDialog
					t.dialogErr = ""
					return Selection{}, false
				}
				if t.killFunc != nil {
					if err := t.killFunc(s.UUID); err != nil {
						t.dialogErr = err.Error()
						return Selection{}, false
					}
				}
				t.mode = modeKillInitiated
				t.dialogErr = ""
			} else {
				t.mode = modeMenu
			}
		case 'n', 'N':
			t.mode = modeMenu
			t.dialogErr = ""
		}
	}
	return Selection{}, false
}

func (t *tui) handleDeleteDialogKey(ev *tcell.EventKey) (Selection, bool) {
	switch ev.Key() {
	case tcell.KeyEscape:
		t.mode = modeMenu
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'y', 'Y':
			if t.cursor >= 0 && t.cursor < len(t.sessions) {
				s := t.sessions[t.cursor]
				return Selection{Action: ActionDeleteSession, SessionID: s.UUID, Name: s.Name}, true
			}
			t.mode = modeMenu
		case 'n', 'N':
			t.mode = modeMenu
		}
	}
	return Selection{}, false
}

func (t *tui) handleKey(ev *tcell.EventKey) (Selection, bool) {
	switch t.mode {
	case modeCreateDialog:
		return t.handleCreateDialogKey(ev)
	case modeCloneDialog:
		return t.handleCloneDialogKey(ev)
	case modeDeleteConfirm:
		return t.handleDeleteDialogKey(ev)
	case modeLockedDialog:
		return t.handleLockedDialogKey(ev)
	case modeOverrideConfirm:
		return t.handleOverrideDialogKey(ev)
	case modeKillConfirm:
		return t.handleKillDialogKey(ev)
	case modeNotRunningDialog:
		return t.handleNotRunningDialogKey(ev)
	case modeKillInitiated:
		return t.handleKillInitiatedDialogKey(ev)
	default:
		return t.handleMenuKey(ev)
	}
}

func (t *tui) handleMenuKey(ev *tcell.EventKey) (Selection, bool) {
	switch ev.Key() {
	case tcell.KeyUp:
		if t.cursor > 0 {
			t.cursor--
		}
	case tcell.KeyDown:
		if t.cursor < len(t.sessions)-1 {
			t.cursor++
		}
	case tcell.KeyEnter:
		if len(t.sessions) > 0 && t.cursor >= 0 && t.cursor < len(t.sessions) {
			s := t.sessions[t.cursor]
			if t.isLocked(s.UUID) {
				t.mode = modeLockedDialog
				return Selection{}, false
			}
			return Selection{Action: s.UUID, SessionID: s.UUID}, true
		}
	case tcell.KeyEscape:
		return Selection{Action: ActionQuit}, true
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'n', 'N':
			t.mode = modeCreateDialog
			t.inputBuf = nil
			t.dialogErr = ""
		case 'c', 'C':
			if len(t.sessions) > 0 && t.cursor >= 0 && t.cursor < len(t.sessions) {
				t.mode = modeCloneDialog
				t.inputBuf = nil
				t.dialogErr = ""
			}
		case 'd', 'D':
			if len(t.sessions) > 0 && t.cursor >= 0 && t.cursor < len(t.sessions) {
				t.mode = modeDeleteConfirm
			}
		case 'o', 'O':
			if len(t.sessions) > 0 && t.cursor >= 0 && t.cursor < len(t.sessions) {
				s := t.sessions[t.cursor]
				if t.isLocked(s.UUID) {
					t.mode = modeOverrideConfirm
					t.dialogErr = ""
				}
			}
		case 'k', 'K':
			if len(t.sessions) > 0 && t.cursor >= 0 && t.cursor < len(t.sessions) {
				t.mode = modeKillConfirm
				t.dialogErr = ""
			}
		case 'r', 'R':
			return Selection{Action: ActionReload}, true
		case 'q', 'Q':
			return Selection{Action: ActionQuit}, true
		default:
			if ev.Rune() >= '1' && ev.Rune() <= '9' {
				idx := int(ev.Rune() - '1')
				if idx < len(t.sessions) {
					s := t.sessions[idx]
					if t.isLocked(s.UUID) {
						t.cursor = idx
						t.mode = modeLockedDialog
						return Selection{}, false
					}
					return Selection{Action: s.UUID, SessionID: s.UUID}, true
				}
			}
		}
	}
	return Selection{}, false
}

func (t *tui) handleCreateDialogKey(ev *tcell.EventKey) (Selection, bool) {
	switch ev.Key() {
	case tcell.KeyEscape:
		t.mode = modeMenu
		t.inputBuf = nil
		t.dialogErr = ""
	case tcell.KeyEnter:
		name := strings.TrimSpace(string(t.inputBuf))
		if err := session.ValidateName(name); err != nil {
			t.dialogErr = err.Error()
			return Selection{}, false
		}
		return Selection{Action: ActionNewSession, Name: name}, true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(t.inputBuf) > 0 {
			t.inputBuf = t.inputBuf[:len(t.inputBuf)-1]
			t.dialogErr = ""
		}
	case tcell.KeyRune:
		if len(t.inputBuf) < 64 {
			t.inputBuf = append(t.inputBuf, ev.Rune())
			t.dialogErr = ""
		}
	}
	return Selection{}, false
}

func (t *tui) handleCloneDialogKey(ev *tcell.EventKey) (Selection, bool) {
	switch ev.Key() {
	case tcell.KeyEscape:
		t.mode = modeMenu
		t.inputBuf = nil
		t.dialogErr = ""
	case tcell.KeyEnter:
		if t.cursor < 0 || t.cursor >= len(t.sessions) {
			t.mode = modeMenu
			return Selection{}, false
		}
		name := strings.TrimSpace(string(t.inputBuf))
		if err := session.ValidateName(name); err != nil {
			t.dialogErr = err.Error()
			return Selection{}, false
		}
		s := t.sessions[t.cursor]
		return Selection{Action: ActionCloneSession, SessionID: s.UUID, Name: name}, true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(t.inputBuf) > 0 {
			t.inputBuf = t.inputBuf[:len(t.inputBuf)-1]
			t.dialogErr = ""
		}
	case tcell.KeyRune:
		if len(t.inputBuf) < 64 {
			t.inputBuf = append(t.inputBuf, ev.Rune())
			t.dialogErr = ""
		}
	}
	return Selection{}, false
}

func drawString(screen tcell.Screen, x, y int, s string, style tcell.Style) {
	for _, ch := range s {
		screen.SetContent(x, y, ch, nil, style)
		x++
	}
}

func fillRow(screen tcell.Screen, row, width int, style tcell.Style) {
	for x := 0; x < width; x++ {
		screen.SetContent(x, row, ' ', nil, style)
	}
}

func clipToWidth(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	return string(runes[:width])
}
