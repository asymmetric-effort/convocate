package hostinstall

import (
	"context"
	"fmt"
	"io"
	"time"
)

// RebootOptions controls how RebootAndReconnect waits for the target to come
// back after issuing a reboot.
type RebootOptions struct {
	// InitialWait is the delay before the first reconnect attempt. Gives the
	// target time to actually drop the SSH connection and shut down.
	InitialWait time.Duration

	// PollInterval is the delay between reconnect attempts.
	PollInterval time.Duration

	// Timeout caps the total time spent waiting for the target to come back.
	Timeout time.Duration

	// Progress receives status lines (e.g. "waiting for target...").
	Progress io.Writer
}

// DefaultRebootOptions returns tuning that works for most cloud/bare-metal
// Ubuntu hosts: ~10s for SSH to die, then poll every 5s for up to 5 min.
func DefaultRebootOptions() RebootOptions {
	return RebootOptions{
		InitialWait:  10 * time.Second,
		PollInterval: 5 * time.Second,
		Timeout:      5 * time.Minute,
	}
}

// RebootAndReconnect issues a reboot on the target through the current
// runner, closes that runner, waits for the target to come back, and returns
// a fresh SSHRunner. Only usable against an SSHRunner — calling against a
// LocalRunner returns an error because rebooting the machine the CLI is
// running on would abort the install.
func RebootAndReconnect(ctx context.Context, current *SSHRunner, cfg SSHConfig, opts RebootOptions) (*SSHRunner, error) {
	if current == nil {
		return nil, fmt.Errorf("reboot: nil runner")
	}
	if opts.InitialWait == 0 {
		opts.InitialWait = 10 * time.Second
	}
	if opts.PollInterval == 0 {
		opts.PollInterval = 5 * time.Second
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}

	progress := opts.Progress
	if progress == nil {
		progress = io.Discard
	}
	// Wrap so the SSH stdout/stderr goroutines (x/crypto/ssh spawns one
	// each) can write to the same destination without racing.
	syncProgress := &syncWriter{w: progress}
	fmt.Fprintf(syncProgress, "[claude-host] rebooting %s...\n", current.Target())

	// `shutdown -r +0` or plain `reboot`. We use `systemctl reboot` when
	// available (graceful) and fall back to `reboot` otherwise. Ignore the
	// error from the reboot command itself — the SSH session typically dies
	// mid-reply, which looks like an error on the client side.
	_ = current.Run(ctx, "systemctl reboot || reboot", RunOptions{Sudo: true, Stdout: syncProgress, Stderr: syncProgress})
	_ = current.Close()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(opts.InitialWait):
	}

	deadline := time.Now().Add(opts.Timeout)
	for attempt := 1; ; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		fmt.Fprintf(progress, "[claude-host] reconnect attempt %d...\n", attempt)
		r, err := NewSSHRunner(cfg)
		if err == nil {
			// One more check: the host is up enough to accept a trivial cmd.
			if runErr := r.Run(ctx, "true", RunOptions{}); runErr == nil {
				fmt.Fprintf(progress, "[claude-host] target reachable again.\n")
				return r, nil
			}
			_ = r.Close()
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("target did not come back within %s (last error: %v)", opts.Timeout, err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(opts.PollInterval):
		}
	}
}
