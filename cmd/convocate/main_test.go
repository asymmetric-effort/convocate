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

	"github.com/asymmetric-effort/convocate/internal/config"
	"github.com/asymmetric-effort/convocate/internal/container"
	"github.com/asymmetric-effort/convocate/internal/multihost"
	"github.com/asymmetric-effort/convocate/internal/session"
	"github.com/asymmetric-effort/convocate/internal/user"
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
		if err := run([]string{"convocate", "version"}); err != nil {
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
			if err := run([]string{"convocate", arg}); err != nil {
				t.Errorf("run %s failed: %v", arg, err)
			}
		})
		if !strings.Contains(out, "Usage") {
			t.Errorf("run %s: output missing 'Usage': %q", arg, out)
		}
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	err := run([]string{"convocate", "bogus"})
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
	err := run([]string{"convocate", "install"})
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

// --- handleNewSession + handleCloneSession: v2.x (router-only) paths ---

func TestHandleNewSession_RequiresAgentID(t *testing.T) {
	base := t.TempDir()
	mgr := session.NewManager(base, filepath.Join(base, "skel"))
	router := &multihost.Router{Local: mgr}
	err := handleNewSession(router, "", "no-agent", 0, "tcp", "", nil)
	if err == nil || !strings.Contains(err.Error(), "agent-id required") {
		t.Errorf("expected agent-id-required error, got %v", err)
	}
}

func TestHandleCloneSession_LocalSourceReturnsOrphanError(t *testing.T) {
	base := t.TempDir()
	mgr := session.NewManager(base, filepath.Join(base, "skel"))
	router := &multihost.Router{Local: mgr}
	// No agents registered → every session is treated as a local orphan.
	err := handleCloneSession(router, "some-uuid", "new", nil)
	if err == nil || !strings.Contains(err.Error(), "orphan") {
		t.Errorf("expected orphan error, got %v", err)
	}
}

// Compile-time assertions: silence "unused import" noise after the
// strip-local-docker cleanup.
var (
	_ = errors.New
	_ = user.Info{}
	_ = config.AppName
	_ = exec.Command
	_ = session.PortAuto
	_ = filepath.Join
	_ = io.Copy
	_ = bytes.NewBufferString
	_ = container.NewRunner
)
