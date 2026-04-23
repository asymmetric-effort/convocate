package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn with os.Stdout replaced by a pipe and returns what
// was written. Used to assert against help/version output.
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

func TestRun_Version(t *testing.T) {
	origV := Version
	Version = "test-0.0.0"
	defer func() { Version = origV }()

	out := captureStdout(t, func() {
		if err := run([]string{"claude-host", "version"}); err != nil {
			t.Fatalf("run version: %v", err)
		}
	})
	if !strings.Contains(out, "test-0.0.0") || !strings.Contains(out, appName) {
		t.Errorf("version output = %q", out)
	}
}

func TestRun_Help(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		out := captureStdout(t, func() {
			if err := run([]string{"claude-host", arg}); err != nil {
				t.Errorf("run %s: %v", arg, err)
			}
		})
		if !strings.Contains(out, "Usage") {
			t.Errorf("%s: help output missing Usage: %q", arg, out)
		}
	}
}

func TestRun_NoArgs_ShowsUsage(t *testing.T) {
	out := captureStdout(t, func() {
		if err := run([]string{"claude-host"}); err != nil {
			t.Errorf("run (no args): %v", err)
		}
	})
	if !strings.Contains(out, "Usage") {
		t.Errorf("expected usage when no args, got: %q", out)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	err := run([]string{"claude-host", "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("error = %q, want 'unknown command'", err.Error())
	}
}

// The *local* mode invariants — each subcommand must refuse to run without
// root. Running the test suite as non-root verifies the friendly error.

func TestCmdInstall_LocalNonRoot_FriendlyError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("tests the non-root path only")
	}
	err := run([]string{"claude-host", "install"})
	if err == nil {
		t.Fatal("expected error without root")
	}
	if !strings.Contains(err.Error(), "run as root") {
		t.Errorf("error = %q, want 'run as root' guidance", err.Error())
	}
}

func TestCmdInitShell_LocalNonRoot_FriendlyError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("tests the non-root path only")
	}
	err := run([]string{"claude-host", "init-shell"})
	if err == nil || !strings.Contains(err.Error(), "run as root") {
		t.Errorf("expected friendly root error, got: %v", err)
	}
}

func TestCmdInitAgent_LocalNonRoot_FriendlyError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("tests the non-root path only")
	}
	err := run([]string{"claude-host", "init-agent"})
	if err == nil || !strings.Contains(err.Error(), "run as root") {
		t.Errorf("expected friendly root error, got: %v", err)
	}
}

func TestCmdUpdate_LocalNonRoot_FriendlyError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("tests the non-root path only")
	}
	err := run([]string{"claude-host", "update"})
	if err == nil || !strings.Contains(err.Error(), "run as root") {
		t.Errorf("expected friendly root error, got: %v", err)
	}
}

// Flag parsing — invalid flag should surface from Parse rather than panic.
func TestCmdInstall_BadFlag(t *testing.T) {
	err := run([]string{"claude-host", "install", "--nosuchflag"})
	if err == nil {
		t.Fatal("expected error for bad flag")
	}
}

func TestDescribeTarget(t *testing.T) {
	if describeTarget(targetFlags{}) != "local" {
		t.Error("empty target should describe as local")
	}
	if got := describeTarget(targetFlags{host: "h", user: "u"}); got != "u@h" {
		t.Errorf("got %q, want 'u@h'", got)
	}
}
