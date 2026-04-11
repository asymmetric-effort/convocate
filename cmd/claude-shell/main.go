// Package main is the entry point for claude-shell.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/asymmetric-effort/claude-shell/internal/config"
	"github.com/asymmetric-effort/claude-shell/internal/container"
	"github.com/asymmetric-effort/claude-shell/internal/install"
	"github.com/asymmetric-effort/claude-shell/internal/logging"
	"github.com/asymmetric-effort/claude-shell/internal/menu"
	"github.com/asymmetric-effort/claude-shell/internal/session"
	"github.com/asymmetric-effort/claude-shell/internal/user"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 1 {
		switch args[1] {
		case "install":
			return install.New().Run()
		case "version":
			fmt.Printf("%s version %s\n", config.AppName, Version)
			return nil
		case "help", "--help", "-h":
			printUsage()
			return nil
		default:
			return fmt.Errorf("unknown command: %q (use 'help' for usage)", args[1])
		}
	}

	return runSessionManager()
}

func runSessionManager() error {
	log, err := logging.New(config.AppName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to connect to syslog: %v\n", err)
		// Continue without syslog
		return runSessionManagerWithLog(nil)
	}
	defer log.Close()
	return runSessionManagerWithLog(log)
}

func runSessionManagerWithLog(log *logging.Logger) error {
	// Check Docker image exists
	exists, err := container.ImageExists(nil)
	if err != nil {
		return fmt.Errorf("failed to check Docker image: %w", err)
	}
	if !exists {
		return fmt.Errorf("Docker image %q not found; run 'claude-shell install' first", config.ContainerImage())
	}

	// Lookup claude user
	userInfo, err := user.Lookup(config.ClaudeUser)
	if err != nil {
		return fmt.Errorf("failed to lookup claude user: %w", err)
	}

	paths := config.PathsFromHome(userInfo.HomeDir)

	mgr := session.NewManager(paths.SessionsBase, paths.SkelDir)

	for {
		sessions, err := mgr.List()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		sel, err := menu.Display(sessions, os.Stdin, os.Stdout)
		if err != nil {
			return fmt.Errorf("menu error: %w", err)
		}

		switch sel.Action {
		case menu.ActionQuit:
			fmt.Println("Goodbye!")
			return nil
		case menu.ActionNewSession:
			if err := handleNewSession(mgr, userInfo, paths, log); err != nil {
				fmt.Fprintf(os.Stderr, "session error: %v\n", err)
			}
			continue
		case menu.ActionReload:
			continue
		case menu.ActionDeleteSession:
			if len(sessions) == 0 {
				fmt.Println("No sessions to delete.")
				continue
			}
			if err := handleDeleteSession(mgr, sessions); err != nil {
				fmt.Fprintf(os.Stderr, "delete error: %v\n", err)
			}
			continue
		default:
			// Resume existing session
			if err := handleResumeSession(mgr, sel.SessionID, userInfo, paths, log); err != nil {
				fmt.Fprintf(os.Stderr, "session error: %v\n", err)
			}
			continue
		}
	}
}

func handleNewSession(mgr *session.Manager, userInfo user.Info, paths config.Paths, log *logging.Logger) error {
	name, err := menu.PromptSessionName(os.Stdin, os.Stdout)
	if err != nil {
		return err
	}

	if err := session.ValidateName(name); err != nil {
		return fmt.Errorf("invalid session name: %w", err)
	}

	meta, err := mgr.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	if log != nil {
		log.Infof("created session %s (%s)", meta.UUID, meta.Name)
	}
	fmt.Printf("Created session %q (%s)\n", meta.Name, meta.UUID[:8])

	return launchSession(mgr, meta.UUID, userInfo, paths, log)
}

func handleResumeSession(mgr *session.Manager, sessionID string, userInfo user.Info, paths config.Paths, log *logging.Logger) error {
	meta, err := mgr.Get(sessionID)
	if err != nil {
		return err
	}

	if log != nil {
		log.Infof("resuming session %s (%s)", meta.UUID, meta.Name)
	}
	fmt.Printf("Resuming session %q (%s)\n", meta.Name, meta.UUID[:8])

	return launchSession(mgr, sessionID, userInfo, paths, log)
}

func handleDeleteSession(mgr *session.Manager, sessions []session.Metadata) error {
	id, err := menu.PromptDeleteSession(sessions, os.Stdin, os.Stdout)
	if err != nil {
		return err
	}
	if id == "" {
		return nil // cancelled
	}

	meta, err := mgr.Get(id)
	if err != nil {
		return err
	}

	confirmed, err := menu.ConfirmDelete(meta.Name, meta.UUID, os.Stdin, os.Stdout)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Println("Cancelled.")
		return nil
	}

	if err := mgr.Delete(id); err != nil {
		return err
	}

	fmt.Printf("Deleted session %q (%s)\n", meta.Name, id[:8])
	return nil
}

func launchSession(mgr *session.Manager, sessionID string, userInfo user.Info, paths config.Paths, log *logging.Logger) error {
	unlock, err := mgr.Lock(sessionID)
	if err != nil {
		return err
	}
	defer unlock()

	if err := mgr.Touch(sessionID); err != nil {
		if log != nil {
			log.Warningf("failed to update last accessed time: %v", err)
		}
	}

	runner := container.NewRunner(sessionID, mgr.SessionDir(sessionID), userInfo, paths)

	// Check if container is already running
	running, err := runner.IsRunning()
	if err != nil {
		return fmt.Errorf("failed to check container status: %w", err)
	}
	if running {
		if log != nil {
			log.Infof("attaching to running container for session %s", sessionID)
		}
		if err := runner.Attach(); err != nil {
			// Ignore normal exit status from tmux detach
			if !strings.Contains(err.Error(), "exit status") {
				return fmt.Errorf("attach error: %w", err)
			}
		}
		return nil
	}

	// Set up signal handling for graceful termination
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	done := make(chan error, 1)
	go func() {
		done <- runner.Start()
	}()

	select {
	case sig := <-sigCh:
		if log != nil {
			log.Infof("received signal %v, stopping session %s", sig, sessionID)
		}
		fmt.Printf("\n\nReceived %v. Gracefully stopping session...\n", sig)
		if err := runner.Stop(); err != nil {
			if log != nil {
				log.Errorf("failed to stop container: %v", err)
			}
		}
		return nil
	case err := <-done:
		if err != nil {
			// Don't report error if it was just the container exiting normally
			if strings.Contains(err.Error(), "exit status") {
				return nil
			}
			return fmt.Errorf("session error: %w", err)
		}
		return nil
	}
}

func printUsage() {
	fmt.Printf(`%s - Isolated Claude CLI sessions in Docker containers

Usage:
  %s              Launch the session manager
  %s install      Install and configure claude-shell
  %s version      Print version information
  %s help         Show this help message
`, config.AppName, config.AppName, config.AppName, config.AppName, config.AppName)
}
