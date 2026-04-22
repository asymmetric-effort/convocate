package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asymmetric-effort/claude-shell/internal/config"
	"github.com/asymmetric-effort/claude-shell/internal/container"
	"github.com/asymmetric-effort/claude-shell/internal/session"
	"github.com/asymmetric-effort/claude-shell/internal/user"
)

// captureStdout replaces os.Stdout with a pipe, runs fn, and returns captured output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	w.Close()
	os.Stdout = orig
	return <-done
}

// --- run() entry-point tests ---

func TestRun_Version(t *testing.T) {
	origV := Version
	Version = "test-1.2.3"
	defer func() { Version = origV }()

	out := captureStdout(t, func() {
		if err := run([]string{"claude-shell", "version"}); err != nil {
			t.Fatalf("run version failed: %v", err)
		}
	})
	if !strings.Contains(out, "test-1.2.3") {
		t.Errorf("version output missing version string: %q", out)
	}
	if !strings.Contains(out, config.AppName) {
		t.Errorf("version output missing app name: %q", out)
	}
}

func TestRun_Help(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		out := captureStdout(t, func() {
			if err := run([]string{"claude-shell", arg}); err != nil {
				t.Errorf("run %s failed: %v", arg, err)
			}
		})
		if !strings.Contains(out, "Usage") {
			t.Errorf("run %s: output missing 'Usage': %q", arg, out)
		}
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	err := run([]string{"claude-shell", "bogus"})
	if err == nil {
		t.Error("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("error = %q, want 'unknown command'", err.Error())
	}
}

func TestRun_Install_NotRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test validates non-root behavior")
	}
	err := run([]string{"claude-shell", "install"})
	if err == nil {
		t.Error("expected install to fail when not root")
	}
}

func TestPrintUsageIncludesAppName(t *testing.T) {
	out := captureStdout(t, printUsage)
	if !strings.Contains(out, config.AppName) {
		t.Errorf("printUsage missing app name: %q", out)
	}
}

// --- restartSessionDetached tests ---

func testUserInfo() user.Info {
	return user.Info{UID: 1337, GID: 1337, Username: "claude", HomeDir: "/home/claude"}
}

func testPaths(home string) config.Paths {
	return config.Paths{
		ClaudeHome:   home,
		SessionsBase: home,
		SkelDir:      filepath.Join(home, ".skel"),
		ClaudeConfig: filepath.Join(home, ".claude"),
		SSHDir:       filepath.Join(home, ".ssh"),
		GitConfig:    filepath.Join(home, ".gitconfig"),
	}
}

// withRunner swaps the package-level newRunner factory for the duration of a
// test so restartSessionDetached uses an injectable exec function.
func withRunner(t *testing.T, execFn container.ExecFunc) {
	t.Helper()
	orig := newRunner
	newRunner = func(sessionID, sessionDir string, userInfo user.Info, paths config.Paths) *container.Runner {
		return container.NewRunnerWithExec(sessionID, sessionDir, userInfo, paths, execFn)
	}
	t.Cleanup(func() { newRunner = orig })
}

func TestRestartSessionDetached_Success(t *testing.T) {
	base := t.TempDir()
	mgr := session.NewManager(base, filepath.Join(base, "skel"))
	meta, err := mgr.CreateWithPort("proj", 8080)
	if err != nil {
		t.Fatal(err)
	}

	var calls []string
	withRunner(t, func(name string, args ...string) *exec.Cmd {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		// IsRunning inspects state; return "false" so restart proceeds.
		if len(args) > 0 && args[0] == "inspect" {
			return exec.Command("echo", "false")
		}
		return exec.Command("true")
	})

	if err := restartSessionDetached(mgr, meta.UUID, testUserInfo(), testPaths(base), nil); err != nil {
		t.Fatalf("restartSessionDetached failed: %v", err)
	}

	// Expect the docker run --detach call to include our port flag.
	joined := strings.Join(calls, "\n")
	if !strings.Contains(joined, "run") || !strings.Contains(joined, "--detach") {
		t.Errorf("expected docker run --detach invocation, got:\n%s", joined)
	}
	if !strings.Contains(joined, "-p 8080:8080") {
		t.Errorf("expected -p 8080:8080 from session.json port, got:\n%s", joined)
	}
}

func TestRestartSessionDetached_MissingSession(t *testing.T) {
	base := t.TempDir()
	mgr := session.NewManager(base, "")
	withRunner(t, func(name string, args ...string) *exec.Cmd { return exec.Command("true") })
	err := restartSessionDetached(mgr, "missing-uuid", testUserInfo(), testPaths(base), nil)
	if err == nil {
		t.Error("expected error for missing session")
	}
}

func TestRestartSessionDetached_AlreadyRunning(t *testing.T) {
	base := t.TempDir()
	mgr := session.NewManager(base, filepath.Join(base, "skel"))
	meta, err := mgr.Create("running")
	if err != nil {
		t.Fatal(err)
	}

	withRunner(t, func(name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "inspect" {
			return exec.Command("echo", "true")
		}
		return exec.Command("true")
	})

	err = restartSessionDetached(mgr, meta.UUID, testUserInfo(), testPaths(base), nil)
	if err == nil {
		t.Error("expected error when container is already running")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("error = %q, want 'already running'", err.Error())
	}
}

func TestRestartSessionDetached_StartFails(t *testing.T) {
	base := t.TempDir()
	mgr := session.NewManager(base, filepath.Join(base, "skel"))
	meta, err := mgr.Create("s")
	if err != nil {
		t.Fatal(err)
	}

	withRunner(t, func(name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "inspect" {
			return exec.Command("echo", "false")
		}
		return exec.Command("false")
	})

	err = restartSessionDetached(mgr, meta.UUID, testUserInfo(), testPaths(base), nil)
	if err == nil {
		t.Error("expected error when docker run fails")
	}
}

// --- handleNewSession, handleCloneSession, handleResumeSession: error branch only ---

func TestHandleNewSession_CreateFails(t *testing.T) {
	base := t.TempDir()
	// Use a skel dir we can't copy from (nonexistent is fine — Create allows it).
	// Force a name validation error by passing an invalid name.
	mgr := session.NewManager(base, filepath.Join(base, "skel"))

	// A name over 64 chars trips ValidateName inside CreateWithPort? No — Create
	// does not call ValidateName; but it still succeeds for any string. To force
	// an error, create the session dir with no write perms first.
	badBase := filepath.Join(base, "noperm")
	if err := os.MkdirAll(badBase, 0500); err != nil {
		t.Fatal(err)
	}
	_ = mgr // silence linter
	badMgr := session.NewManager(badBase, filepath.Join(base, "skel"))
	withRunner(t, func(name string, args ...string) *exec.Cmd { return exec.Command("true") })
	err := handleNewSession(badMgr, "fails", 0, testUserInfo(), testPaths(base), nil)
	if err == nil {
		t.Error("expected error when session directory creation fails")
	}
}

func TestHandleCloneSession_MissingSource(t *testing.T) {
	base := t.TempDir()
	mgr := session.NewManager(base, filepath.Join(base, "skel"))
	withRunner(t, func(name string, args ...string) *exec.Cmd { return exec.Command("true") })
	err := handleCloneSession(mgr, "missing-uuid", "new", testUserInfo(), testPaths(base), nil)
	if err == nil {
		t.Error("expected error when cloning missing source")
	}
}

func TestHandleResumeSession_Missing(t *testing.T) {
	base := t.TempDir()
	mgr := session.NewManager(base, "")
	err := handleResumeSession(mgr, "missing-uuid", testUserInfo(), testPaths(base), nil)
	if err == nil {
		t.Error("expected error when resuming missing session")
	}
}

// --- launchSession: locked session error ---

func TestLaunchSession_LockHeld(t *testing.T) {
	base := t.TempDir()
	mgr := session.NewManager(base, filepath.Join(base, "skel"))
	meta, err := mgr.Create("s")
	if err != nil {
		t.Fatal(err)
	}
	// Acquire the lock so the second attempt fails.
	unlock, err := mgr.Lock(meta.UUID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	err = launchSession(mgr, meta.UUID, 0, testUserInfo(), testPaths(base), nil)
	if err == nil {
		t.Error("expected lock error on second launchSession")
	}
}

// --- Compile-time assertion: errReader is unused by now; keep to satisfy unused-check ---

var _ = errors.New
