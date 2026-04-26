// Package hostinstall drives convocate-host's install/update/init-* workflows.
// It exposes a Runner abstraction so the same step list runs locally or
// against a remote host over SSH, plus helpers for copying binaries and
// waiting for the target to come back after a reboot.
package hostinstall

import (
	"context"
	"io"
	"os"
)

// Runner executes shell commands and copies files against a target — either
// the local host (LocalRunner) or a remote host over SSH (SSHRunner).
type Runner interface {
	// Run executes cmd on the target. Stdout/stderr of the child process are
	// streamed to opts.Stdout/opts.Stderr. When opts.Sudo is true, the command
	// is prefixed with "sudo -n -- bash -c" so a NOPASSWD sudoers rule on the
	// remote is required. The method returns a non-nil error when the command
	// exits non-zero.
	Run(ctx context.Context, cmd string, opts RunOptions) error

	// CopyFile uploads src (a local path) to destPath on the target with the
	// given file mode. Parent directories on the target must already exist.
	CopyFile(ctx context.Context, srcPath, destPath string, mode os.FileMode) error

	// Target returns a short description of where commands run ("local" or
	// "user@host").
	Target() string

	// Close releases any underlying SSH connection. Safe to call on
	// LocalRunner (no-op).
	Close() error
}

// RunOptions configures a single Run call.
type RunOptions struct {
	// Sudo wraps the command with "sudo -n --" before executing. The target
	// user must have NOPASSWD sudoers privileges for this to succeed.
	Sudo bool

	// Stdout/Stderr receive the streamed child-process output. nil discards.
	Stdout io.Writer
	Stderr io.Writer

	// Stdin is piped to the child process. nil means no stdin.
	Stdin io.Reader

	// Env adds environment variables to the child process in "K=V" form.
	Env []string
}
