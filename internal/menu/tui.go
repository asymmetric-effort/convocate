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
	titleStyle    = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlue)
	menuBarStyle  = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
	headerStyle   = tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
	normalStyle   = tcell.StyleDefault
	selectedStyle = tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorWhite)
	separatorStyle = tcell.StyleDefault.Foreground(tcell.ColorGray)
)

type tui struct {
	screen       tcell.Screen
	sessions     []session.Metadata
	cursor       int
	offset       int
	tickInterval time.Duration
}

// newScreenFunc creates a new tcell.Screen. Override in tests.
var newScreenFunc = tcell.NewScreen

// Display renders the TUI session menu and returns the user's selection.
func Display(sessions []session.Metadata) (Selection, error) {
	screen, err := newScreenFunc()
	if err != nil {
		return Selection{}, fmt.Errorf("failed to create screen: %w", err)
	}
	if err := screen.Init(); err != nil {
		return Selection{}, fmt.Errorf("failed to init screen: %w", err)
	}
	defer screen.Fini()

	return DisplayWithScreen(sessions, screen)
}

// DisplayWithScreen renders the TUI on a provided screen (for testing).
func DisplayWithScreen(sessions []session.Metadata, screen tcell.Screen) (Selection, error) {
	return displayWithOptions(sessions, screen, 1*time.Second)
}

func displayWithOptions(sessions []session.Metadata, screen tcell.Screen, tickInterval time.Duration) (Selection, error) {
	t := &tui{
		screen:       screen,
		sessions:     sessions,
		tickInterval: tickInterval,
	}

	// Tick periodically to keep the clock updated
	go func() {
		ticker := time.NewTicker(t.tickInterval)
		defer ticker.Stop()
		for range ticker.C {
			screen.PostEvent(tcell.NewEventInterrupt(nil))
		}
	}()

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
			// clock tick — just redraw
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
	header := fmt.Sprintf("  %-4s| %-20s | %-36s | %-12s | %s",
		"#", "Name", "Session ID", "Created", "Last Accessed")
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
		drawString(t.screen, 2, sessionsStart, "No sessions. Press (C) to create one.", normalStyle)
		return
	}

	for i := 0; i < visibleRows && i+t.offset < len(t.sessions); i++ {
		idx := i + t.offset
		s := t.sessions[idx]

		style := normalStyle
		if idx == t.cursor {
			style = selectedStyle
		}

		line := fmt.Sprintf("  %-4s| %-20s | %s | %-12s | %s",
			strconv.Itoa(idx+1),
			truncate(s.Name, 20),
			s.UUID,
			s.CreatedAt.Format("2006-01-02"),
			s.LastAccessed.Format("2006-01-02"),
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
		"(C)reate a session | (D)elete a session | (R)eload | (Q)uit",
		menuBarStyle)
}

func (t *tui) handleKey(ev *tcell.EventKey) (Selection, bool) {
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
			return Selection{Action: s.UUID, SessionID: s.UUID}, true
		}
	case tcell.KeyEscape:
		return Selection{Action: ActionQuit}, true
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'c', 'C':
			return Selection{Action: ActionNewSession}, true
		case 'd', 'D':
			return Selection{Action: ActionDeleteSession}, true
		case 'r', 'R':
			return Selection{Action: ActionReload}, true
		case 'q', 'Q':
			return Selection{Action: ActionQuit}, true
		default:
			if ev.Rune() >= '1' && ev.Rune() <= '9' {
				idx := int(ev.Rune() - '1')
				if idx < len(t.sessions) {
					s := t.sessions[idx]
					return Selection{Action: s.UUID, SessionID: s.UUID}, true
				}
			}
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
