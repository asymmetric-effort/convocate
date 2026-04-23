package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

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
		if err := run([]string{"claude-agent", "version"}); err != nil {
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
			if err := run([]string{"claude-agent", arg}); err != nil {
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
		if err := run([]string{"claude-agent"}); err != nil {
			t.Errorf("run (no args): %v", err)
		}
	})
	if !strings.Contains(out, "Usage") {
		t.Errorf("expected usage, got: %q", out)
	}
}

func TestRun_Install_Placeholder(t *testing.T) {
	out := captureStdout(t, func() {
		if err := run([]string{"claude-agent", "install"}); err != nil {
			t.Errorf("install should return nil (placeholder), got: %v", err)
		}
	})
	if !strings.Contains(out, "install placeholder") {
		t.Errorf("unexpected install output: %q", out)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	err := run([]string{"claude-agent", "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("error = %q", err.Error())
	}
}
