// Package install handles the claude-shell install subcommand.
package install

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/asymmetric-effort/claude-shell/internal/assets"
	"github.com/asymmetric-effort/claude-shell/internal/config"
	"github.com/asymmetric-effort/claude-shell/internal/diskspace"
	"github.com/asymmetric-effort/claude-shell/internal/skel"
)

// Installer performs installation tasks for claude-shell.
type Installer struct {
	execFn ExecFunc
}

// ExecFunc abstracts command execution for testing.
type ExecFunc func(name string, args ...string) *exec.Cmd

// DefaultExecFunc returns a standard exec.Cmd.
func DefaultExecFunc(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// New creates a new Installer with the default exec function.
func New() *Installer {
	return &Installer{execFn: DefaultExecFunc}
}

// NewWithExec creates a new Installer with a custom exec function.
func NewWithExec(execFn ExecFunc) *Installer {
	return &Installer{execFn: execFn}
}

// Run executes all installation steps.
func (inst *Installer) Run() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("claude-shell install must be run as root (use sudo)")
	}

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Checking platform", inst.checkPlatform},
		{"Checking Docker", inst.checkDocker},
		{"Creating claude user", inst.createUser},
		{"Setting up skeleton directory", inst.setupSkel},
		{"Checking claude CLI", inst.checkClaudeCLI},
		{"Installing claude-shell binary", inst.installBinary},
		{"Building Docker image", inst.buildImage},
		{"Configuring login shell", inst.configureLoginShell},
	}

	for _, step := range steps {
		fmt.Printf("[install] %s...\n", step.name)
		if err := step.fn(); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
		fmt.Printf("[install] %s... done\n", step.name)
	}

	fmt.Println("\n[install] Installation complete.")
	return nil
}

func (inst *Installer) checkPlatform() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("claude-shell requires Linux (detected: %s)", runtime.GOOS)
	}
	return nil
}

func (inst *Installer) checkDocker() error {
	cmd := inst.execFn("docker", "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker is not available or not running: %w", err)
	}
	return nil
}

func (inst *Installer) createUser() error {
	_, err := user.Lookup(config.ClaudeUser)
	if err == nil {
		fmt.Printf("[install]   User %q already exists, ensuring group membership...\n", config.ClaudeUser)
		cmd := inst.execFn("usermod", "-aG", "docker", config.ClaudeUser)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add user to docker group: %w", err)
		}
		return nil
	}

	cmd := inst.execFn("useradd",
		"--create-home",
		"--shell", "/bin/bash",
		"--groups", "docker",
		config.ClaudeUser,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user %q: %w", config.ClaudeUser, err)
	}

	return nil
}

func (inst *Installer) setupSkel() error {
	u, err := user.Lookup(config.ClaudeUser)
	if err != nil {
		return fmt.Errorf("cannot find user %q: %w", config.ClaudeUser, err)
	}

	skelPath := filepath.Join(u.HomeDir, config.SkelDir)
	if err := skel.Setup(skelPath); err != nil {
		return err
	}

	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	return chownRecursive(skelPath, uid, gid)
}

func (inst *Installer) checkClaudeCLI() error {
	if _, err := os.Stat(config.ClaudeBinaryPath); os.IsNotExist(err) {
		return fmt.Errorf("claude CLI not found at %s; please install it first", config.ClaudeBinaryPath)
	}
	fmt.Printf("[install]   Claude CLI found at %s\n", config.ClaudeBinaryPath)
	return nil
}

func (inst *Installer) buildImage() error {
	// Write embedded assets to a temporary build context directory.
	buildCtx, err := os.MkdirTemp("", "claude-shell-build-*")
	if err != nil {
		return fmt.Errorf("failed to create build context: %w", err)
	}
	defer os.RemoveAll(buildCtx)

	dockerfile, err := assets.Dockerfile()
	if err != nil {
		return fmt.Errorf("failed to extract embedded Dockerfile: %w", err)
	}
	entrypoint, err := assets.Entrypoint()
	if err != nil {
		return fmt.Errorf("failed to extract embedded entrypoint.sh: %w", err)
	}

	totalSize := int64(len(dockerfile) + len(entrypoint))
	if err := diskspace.CheckForFile(buildCtx, totalSize); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(buildCtx, "Dockerfile"), dockerfile, 0644); err != nil {
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}
	if err := os.WriteFile(filepath.Join(buildCtx, "entrypoint.sh"), entrypoint, 0755); err != nil {
		return fmt.Errorf("failed to write entrypoint.sh: %w", err)
	}

	cmd := inst.execFn("docker", "build",
		"-t", config.ContainerImage(),
		buildCtx,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build Docker image: %w", err)
	}

	return nil
}

func (inst *Installer) installBinary() error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine own executable path: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("failed to resolve executable symlink: %w", err)
	}

	dest := config.ClaudeShellBinaryPath

	// Skip if we're already running from the install location.
	if self == dest {
		fmt.Printf("[install]   Already installed at %s\n", dest)
		return nil
	}

	if err := copyBinary(self, dest); err != nil {
		return err
	}

	fmt.Printf("[install]   Installed %s\n", dest)
	return nil
}

// copyBinary copies src to dest atomically via a temporary file, preserving 0755 permissions.
func copyBinary(src, dest string) error {
	sf, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source binary: %w", err)
	}
	defer sf.Close()

	tmp := dest + ".tmp"
	df, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination binary: %w", err)
	}
	defer func() {
		df.Close()
		os.Remove(tmp) // clean up on failure; no-op after rename
	}()

	if _, err := io.Copy(df, sf); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	if err := df.Close(); err != nil {
		return fmt.Errorf("failed to close destination binary: %w", err)
	}

	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}

	return nil
}

func (inst *Installer) configureLoginShell() error {
	shellPath := config.ClaudeShellBinaryPath

	// Ensure the binary exists at the expected path.
	if _, err := os.Stat(shellPath); os.IsNotExist(err) {
		return fmt.Errorf("claude-shell binary not found at %s", shellPath)
	}

	// Add to /etc/shells if not already present.
	if err := ensureInEtcShells("/etc/shells", shellPath); err != nil {
		return fmt.Errorf("failed to update /etc/shells: %w", err)
	}

	// Set the login shell for the claude user.
	cmd := inst.execFn("usermod", "--shell", shellPath, config.ClaudeUser)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set login shell for %q: %w", config.ClaudeUser, err)
	}

	fmt.Printf("[install]   Login shell set to %s for user %q\n", shellPath, config.ClaudeUser)
	return nil
}

// ensureInEtcShells adds shellPath to the given shells file if it's not already listed.
func ensureInEtcShells(shellsFile, shellPath string) error {
	f, err := os.Open(shellsFile)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == shellPath {
			return nil // already present
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	// Append the shell path.
	af, err := os.OpenFile(shellsFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer af.Close()

	if _, err := fmt.Fprintf(af, "%s\n", shellPath); err != nil {
		return err
	}

	return nil
}

func chownRecursive(path string, uid, gid int) error {
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, uid, gid)
	})
}
