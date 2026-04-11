// Package install handles the claude-shell install subcommand.
package install

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"

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
		{"Building Docker image", inst.buildImage},
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

func chownRecursive(path string, uid, gid int) error {
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, uid, gid)
	})
}
