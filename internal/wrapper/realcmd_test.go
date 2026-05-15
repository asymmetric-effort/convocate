package wrapper

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRealCommandRunnerRun(t *testing.T) {
	runner := &realCommandRunner{}
	ctx := context.Background()

	output, err := runner.Run(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(output, "hello") {
		t.Errorf("output: got %q, want to contain hello", output)
	}
}

func TestRealCommandRunnerRunWithEnv(t *testing.T) {
	runner := &realCommandRunner{}
	ctx := context.Background()

	output, err := runner.RunWithEnv(ctx, []string{"TEST_VAR_XYZ=abc"}, "env")
	if err != nil {
		t.Fatalf("RunWithEnv: %v", err)
	}
	if !strings.Contains(output, "TEST_VAR_XYZ=abc") {
		t.Errorf("output: got %q, want to contain TEST_VAR_XYZ=abc", output)
	}
}

func TestRealCommandRunnerRunWithStdin(t *testing.T) {
	runner := &realCommandRunner{}
	ctx := context.Background()

	output, err := runner.RunWithStdin(ctx, "hello\n", "cat")
	if err != nil {
		t.Fatalf("RunWithStdin: %v", err)
	}
	if output != "hello\n" {
		t.Errorf("output: got %q, want %q", output, "hello\n")
	}
}

func TestRealCommandRunnerCancelledContext(t *testing.T) {
	runner := &realCommandRunner{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// sleep is long enough that the context will cancel before it finishes.
	_, err := runner.Run(ctx, "sleep", "10")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}
