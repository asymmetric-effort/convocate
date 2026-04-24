package agentclient

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"github.com/asymmetric-effort/claude-shell/internal/agentserver"
)

// AttachOptions configures one remote-attach session.
type AttachOptions struct {
	// SessionID is the UUID of the session on the agent to attach to.
	SessionID string

	// Stdin/Stdout/Stderr are the user-facing streams. For an interactive
	// attach from the TUI these are os.Stdin/os.Stdout/os.Stderr; tests
	// pass in-memory pipes.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// Cols/Rows carry the initial terminal size. Zero values default to
	// 80x24. In interactive use, leave zero and set EnableRawTerminal=true
	// so Attach probes the controlling terminal instead.
	Cols uint16
	Rows uint16

	// EnableRawTerminal flips stdin into raw mode for the duration of the
	// attach and installs a SIGWINCH handler that forwards resize events
	// to the agent. Only safe when Stdin refers to an actual TTY — set
	// false for tests.
	EnableRawTerminal bool
}

// Attach opens the claude-agent-attach subsystem on conn, sends the
// AttachRequest header for cfg.SessionID, and bridges stdin/stdout to the
// remote pty until either side closes. Blocks for the duration of the
// interactive session.
//
// Reuses conn's underlying TCP/SSH connection — the shell holds one open
// per agent so attach doesn't pay the handshake cost per launch.
func Attach(conn *ssh.Client, cfg AttachOptions) error {
	if cfg.SessionID == "" {
		return fmt.Errorf("agentclient: SessionID required for attach")
	}
	if cfg.Stdin == nil || cfg.Stdout == nil {
		return fmt.Errorf("agentclient: Stdin + Stdout required for attach")
	}

	// Probe the terminal for initial size when raw-mode is on and sizes
	// weren't set explicitly. A non-TTY stdin (pipe in tests) just falls
	// back to 80x24.
	cols, rows := cfg.Cols, cfg.Rows
	if cfg.EnableRawTerminal {
		if f, ok := cfg.Stdin.(*os.File); ok {
			if w, h, err := term.GetSize(int(f.Fd())); err == nil {
				if cols == 0 {
					cols = uint16(w)
				}
				if rows == 0 {
					rows = uint16(h)
				}
			}
		}
	}
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}

	sess, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("open session: %w", err)
	}
	defer sess.Close()

	stdin, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := sess.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}
	if err := sess.RequestSubsystem(agentserver.AttachSubsystem); err != nil {
		return fmt.Errorf("request subsystem: %w", err)
	}

	// Write the header line.
	hdr, err := json.Marshal(agentserver.AttachRequest{
		ID:   cfg.SessionID,
		Cols: cols,
		Rows: rows,
	})
	if err != nil {
		return fmt.Errorf("marshal header: %w", err)
	}
	if _, err := stdin.Write(append(hdr, '\n')); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// Optional raw-mode + winch wiring. Restore on return no matter what.
	var restore func()
	if cfg.EnableRawTerminal {
		restore = enableRawAndWinch(cfg.Stdin, sess)
	}
	if restore != nil {
		defer restore()
	}

	// Three copy goroutines: stdin -> agent, agent stdout -> stdout, agent
	// stderr -> stderr. The attach is "done" when stdout or stderr hit EOF
	// (the server closes the channel after HandleAttach returns). We avoid
	// sess.Wait() because the ssh lib refuses it when the session was
	// started via RequestSubsystem — which would give us "ssh: session not
	// started" in tests and in prod.
	var wg sync.WaitGroup
	wg.Add(2)
	done := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(done); _ = sess.Close() }) }

	go func() {
		// Stdin copy runs without a WaitGroup slot — when remote side
		// closes, the session closes and this Copy unblocks.
		_, _ = io.Copy(stdin, cfg.Stdin)
		_ = stdin.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(cfg.Stdout, stdout)
		finish()
	}()
	go func() {
		defer wg.Done()
		if cfg.Stderr != nil {
			_, _ = io.Copy(cfg.Stderr, stderrPipe)
		} else {
			_, _ = io.Copy(io.Discard, stderrPipe)
		}
		finish()
	}()

	<-done
	wg.Wait()
	return nil
}

// enableRawAndWinch switches the controlling terminal to raw mode and
// installs a SIGWINCH forwarder. Returns a restore func the caller should
// defer — always non-nil even if raw-mode setup fails, so callers don't
// have to nil-check.
func enableRawAndWinch(stdin io.Reader, sess *ssh.Session) func() {
	noop := func() {}

	f, ok := stdin.(*os.File)
	if !ok {
		return noop
	}
	fd := int(f.Fd())
	if !term.IsTerminal(fd) {
		return noop
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return noop
	}

	// Winch handler forwards resize to the remote pty.
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	winchDone := make(chan struct{})
	go func() {
		for {
			select {
			case <-winchDone:
				return
			case <-winch:
				w, h, err := term.GetSize(fd)
				if err != nil {
					continue
				}
				// Session.WindowChange takes (height, width).
				_ = sess.WindowChange(h, w)
			}
		}
	}()

	return func() {
		close(winchDone)
		signal.Stop(winch)
		_ = term.Restore(fd, oldState)
	}
}

// SSHClient exposes the underlying SSH client so callers can use other
// subsystems (notably attach) without re-dialing. Reusing a single
// connection per agent is why we keep the CRUD client open in the first
// place. The returned client may be swapped out by a reconnect — take
// it under the client's lock at call time rather than caching it.
func (c *CRUDClient) SSHClient() *ssh.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}
