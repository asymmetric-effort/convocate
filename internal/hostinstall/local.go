package hostinstall

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// LocalRunner executes commands via os/exec on the local host.
type LocalRunner struct{}

// NewLocalRunner returns a Runner that shells out to the local machine.
func NewLocalRunner() *LocalRunner { return &LocalRunner{} }

// Target implements Runner.
func (*LocalRunner) Target() string { return "local" }

// Close implements Runner.
func (*LocalRunner) Close() error { return nil }

// Run implements Runner.
func (*LocalRunner) Run(ctx context.Context, cmd string, opts RunOptions) error {
	shell := "bash"
	args := []string{"-c", cmd}
	if opts.Sudo {
		// -n: never prompt; fail immediately if a password would be required.
		// -- terminates sudo's own flag parsing so cmd is forwarded verbatim.
		shell = "sudo"
		args = []string{"-n", "--", "bash", "-c", cmd}
	}
	c := exec.CommandContext(ctx, shell, args...)
	c.Env = append(os.Environ(), opts.Env...)
	c.Stdin = opts.Stdin
	c.Stdout = opts.Stdout
	c.Stderr = opts.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("local: %w", err)
	}
	return nil
}

// CopyFile implements Runner — it's just a file copy for LocalRunner.
func (*LocalRunner) CopyFile(_ context.Context, srcPath, destPath string, mode os.FileMode) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open src %s: %w", srcPath, err)
	}
	defer src.Close()

	tmp := destPath + ".tmp"
	dst, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", tmp, err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("copy: %w", err)
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close: %w", err)
	}
	if err := os.Rename(tmp, destPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename -> %s: %w", destPath, err)
	}
	return nil
}
