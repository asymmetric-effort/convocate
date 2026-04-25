package hostinstall

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDefaultRebootOptions(t *testing.T) {
	o := DefaultRebootOptions()
	if o.InitialWait != 10*time.Second {
		t.Errorf("InitialWait = %v, want 10s", o.InitialWait)
	}
	if o.PollInterval != 5*time.Second {
		t.Errorf("PollInterval = %v, want 5s", o.PollInterval)
	}
	if o.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want 5m", o.Timeout)
	}
}

func TestRebootAndReconnect_NilRunnerErrors(t *testing.T) {
	_, err := RebootAndReconnect(context.Background(), nil, SSHConfig{}, RebootOptions{})
	if err == nil || !strings.Contains(err.Error(), "nil runner") {
		t.Errorf("expected nil-runner error, got %v", err)
	}
}
