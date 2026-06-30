// Process management for the Claude CLI child process.
// Handles spawning, stdin/stdout/stderr pipes with Go channels,
// fan-out to multiple WebSocket subscribers, restart, and stop.

package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Process manages a Claude CLI child process and its I/O channels.
type Process struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	mu        sync.Mutex // guards cmd lifecycle
	startedAt time.Time
	flags     []string
	workDir   string
	metrics   *Metrics

	// Fan-out channels — subscribers register/unregister via methods
	stdoutSubs []chan []byte
	stderrSubs []chan []byte
	subMu      sync.RWMutex

	// done signals when the process has exited
	done chan struct{}
}

// NewProcess spawns a Claude CLI process with the given flags.
// The process runs in workDir with stdin/stdout/stderr pipes.
func NewProcess(flags []string, workDir string, metrics *Metrics) (*Process, error) {
	p := &Process{
		flags:   flags,
		workDir: workDir,
		metrics: metrics,
	}
	if err := p.start(); err != nil {
		return nil, err
	}
	return p, nil
}

// start creates and starts the child process.
func (p *Process) start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Build the claude command with flags
	args := []string{"-c", "claude " + strings.Join(p.flags, " ")}
	cmd := exec.Command("bash", args...)
	cmd.Dir = p.workDir
	cmd.Env = os.Environ()

	var err error
	p.stdin, err = cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	p.cmd = cmd
	p.startedAt = time.Now()
	p.done = make(chan struct{})

	// Start pump goroutines for stdout and stderr
	go p.pumpOutput(stdout, true)
	go p.pumpOutput(stderr, false)

	// Monitor process exit
	go func() {
		_ = cmd.Wait()
		close(p.done)
	}()

	log.Printf("[process] Claude CLI started (pid=%d, flags=%v)", cmd.Process.Pid, p.flags)
	return nil
}

// pumpOutput reads from a pipe and fans out to all subscribers.
func (p *Process) pumpOutput(reader io.Reader, isStdout bool) {
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])

			// Update metrics
			if isStdout {
				p.metrics.StdoutBytes.Add(int64(n))
				p.metrics.StdoutMessages.Add(1)
			} else {
				p.metrics.StderrBytes.Add(int64(n))
				p.metrics.StderrMessages.Add(1)
			}

			// Fan out to subscribers
			p.subMu.RLock()
			var subs []chan []byte
			if isStdout {
				subs = p.stdoutSubs
			} else {
				subs = p.stderrSubs
			}
			for _, ch := range subs {
				select {
				case ch <- data:
				default:
					// Drop if subscriber is slow
				}
			}
			p.subMu.RUnlock()
		}
		if err != nil {
			return
		}
	}
}

// WriteStdin writes raw bytes to the Claude CLI's stdin.
func (p *Process) WriteStdin(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stdin == nil {
		return io.ErrClosedPipe
	}
	n, err := p.stdin.Write(data)
	if err != nil {
		return err
	}
	p.metrics.StdinBytes.Add(int64(n))
	p.metrics.StdinMessages.Add(1)
	return nil
}

// SubscribeStdout registers a channel to receive stdout data.
// Returns an unsubscribe function.
func (p *Process) SubscribeStdout() (chan []byte, func()) {
	return p.subscribe(true)
}

// SubscribeStderr registers a channel to receive stderr data.
// Returns an unsubscribe function.
func (p *Process) SubscribeStderr() (chan []byte, func()) {
	return p.subscribe(false)
}

func (p *Process) subscribe(isStdout bool) (chan []byte, func()) {
	ch := make(chan []byte, 64)
	p.subMu.Lock()
	if isStdout {
		p.stdoutSubs = append(p.stdoutSubs, ch)
	} else {
		p.stderrSubs = append(p.stderrSubs, ch)
	}
	p.subMu.Unlock()

	p.metrics.ActiveConnections.Add(1)

	unsub := func() {
		p.subMu.Lock()
		defer p.subMu.Unlock()
		if isStdout {
			p.stdoutSubs = removeChan(p.stdoutSubs, ch)
		} else {
			p.stderrSubs = removeChan(p.stderrSubs, ch)
		}
		p.metrics.ActiveConnections.Add(-1)
	}
	return ch, unsub
}

// removeChan removes a channel from a slice.
func removeChan(subs []chan []byte, ch chan []byte) []chan []byte {
	for i, s := range subs {
		if s == ch {
			return append(subs[:i], subs[i+1:]...)
		}
	}
	return subs
}

// Stop sends SIGTERM and waits for the process to exit.
// If it doesn't exit within timeout, sends SIGKILL.
func (p *Process) Stop(timeout time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	// Send SIGTERM
	_ = p.cmd.Process.Signal(syscall.SIGTERM)

	// Wait for exit or timeout
	select {
	case <-p.done:
		return nil
	case <-time.After(timeout):
		// Force kill
		_ = p.cmd.Process.Kill()
		<-p.done
		return nil
	}
}

// Restart stops the current process and starts a new one.
func (p *Process) Restart(flags []string) error {
	log.Printf("[process] Restarting Claude CLI...")
	if err := p.Stop(10 * time.Second); err != nil {
		log.Printf("[process] Stop error: %v", err)
	}
	p.flags = flags
	p.metrics.ClaudeRestarts.Add(1)
	return p.start()
}

// IsRunning returns true if the process is still running.
func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

// Uptime returns how long the current Claude CLI process has been running.
func (p *Process) Uptime() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.startedAt.IsZero() {
		return 0
	}
	return time.Since(p.startedAt)
}

// Signal sends an OS signal to the Claude CLI process.
func (p *Process) Signal(sig syscall.Signal) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Signal(sig)
}

// Done returns a channel that is closed when the process exits.
func (p *Process) Done() <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.done
}
