// Package main is the entry point for claude-shell.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/asymmetric-effort/claude-shell/internal/capacity"
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

		sel, err := menu.Display(sessions, menu.DisplayOptions{
			IsLocked:          mgr.IsLocked,
			IsRunning:         container.IsContainerRunning,
			Reload:            mgr.List,
			OverrideLock:      mgr.OverrideLock,
			KillSession:       container.StopContainer,
			BackgroundSession: container.DetachClients,
			RestartSession: func(id string) error {
				return restartSessionDetached(mgr, id, userInfo, paths, log)
			},
			SaveSessionEdit: func(id, name, protocol string, port int) error {
				_, err := mgr.Update(id, name, port, protocol)
				if err != nil {
					return err
				}
				if log != nil {
					log.Infof("updated session %s: name=%q port=%d/%s", id, name, port, protocol)
				}
				return nil
			},
		})
		if err != nil {
			return fmt.Errorf("menu error: %w", err)
		}

		switch sel.Action {
		case menu.ActionQuit:
			fmt.Println("Goodbye!")
			return nil
		case menu.ActionNewSession:
			if err := handleNewSession(mgr, sel.Name, sel.Port, sel.Protocol, userInfo, paths, log); err != nil {
				fmt.Fprintf(os.Stderr, "session error: %v\n", err)
			}
			continue
		case menu.ActionCloneSession:
			if err := handleCloneSession(mgr, sel.SessionID, sel.Name, userInfo, paths, log); err != nil {
				fmt.Fprintf(os.Stderr, "session error: %v\n", err)
			}
			continue
		case menu.ActionDeleteSession:
			if err := mgr.Delete(sel.SessionID); err != nil {
				fmt.Fprintf(os.Stderr, "delete error: %v\n", err)
			} else {
				fmt.Printf("Deleted session %q (%s)\n", sel.Name, sel.SessionID[:8])
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

func handleNewSession(mgr *session.Manager, name string, port int, protocol string, userInfo user.Info, paths config.Paths, log *logging.Logger) error {
	meta, err := mgr.CreateWithPortProtocol(name, port, protocol)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	if log != nil {
		log.Infof("created session %s (%s) port=%d/%s", meta.UUID, meta.Name, meta.Port, meta.EffectiveProtocol())
	}
	if meta.Port > 0 {
		fmt.Printf("Created session %q (%s) on port %d/%s\n", meta.Name, meta.UUID[:8], meta.Port, meta.EffectiveProtocol())
	} else {
		fmt.Printf("Created session %q (%s)\n", meta.Name, meta.UUID[:8])
	}

	return launchSession(mgr, meta.UUID, meta.Port, meta.EffectiveProtocol(), userInfo, paths, log)
}

func handleCloneSession(mgr *session.Manager, sourceID, name string, userInfo user.Info, paths config.Paths, log *logging.Logger) error {
	meta, err := mgr.Clone(sourceID, name)
	if err != nil {
		return fmt.Errorf("failed to clone session: %w", err)
	}

	if log != nil {
		log.Infof("cloned session %s from %s (%s)", meta.UUID, sourceID, meta.Name)
	}
	fmt.Printf("Cloned session %q (%s) from %s\n", meta.Name, meta.UUID[:8], sourceID[:8])

	return launchSession(mgr, meta.UUID, meta.Port, meta.EffectiveProtocol(), userInfo, paths, log)
}

// newRunner is the package-level factory for container runners. Tests override
// it to substitute a runner backed by a mock exec function.
var newRunner = container.NewRunner

// restartSessionDetached launches the session's container in background mode
// without attaching a user terminal. The container runs autonomously until the
// user attaches (via Enter on a running session) or kills it.
func restartSessionDetached(mgr *session.Manager, sessionID string, userInfo user.Info, paths config.Paths, log *logging.Logger) error {
	meta, err := mgr.Get(sessionID)
	if err != nil {
		return err
	}

	if err := mgr.Touch(sessionID); err != nil {
		if log != nil {
			log.Warningf("failed to update last accessed time: %v", err)
		}
	}

	runner := newRunner(sessionID, mgr.SessionDir(sessionID), userInfo, paths)
	runner.SetPort(meta.Port)
	runner.SetProtocol(meta.EffectiveProtocol())

	running, err := runner.IsRunning()
	if err != nil {
		return fmt.Errorf("failed to check container status: %w", err)
	}
	if running {
		return fmt.Errorf("session %q is already running", meta.Name)
	}

	if err := capacity.Check(capacity.DefaultThreshold); err != nil {
		return err
	}

	if log != nil {
		log.Infof("restarting session %s (%s) in background", meta.UUID, meta.Name)
	}
	return runner.StartDetached()
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

	return launchSession(mgr, sessionID, meta.Port, meta.EffectiveProtocol(), userInfo, paths, log)
}

func launchSession(mgr *session.Manager, sessionID string, port int, protocol string, userInfo user.Info, paths config.Paths, log *logging.Logger) error {
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
	runner.SetPort(port)
	runner.SetProtocol(protocol)

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

	// Refuse to start new containers when the host is already saturated.
	if err := capacity.Check(capacity.DefaultThreshold); err != nil {
		return err
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
