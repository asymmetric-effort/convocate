package logging

import (
	"log/syslog"
	"testing"
)

// mockWriter implements a minimal syslog.Writer for testing.
// Since syslog.Writer is a struct (not an interface), we test via the dialer.

func TestNew_Success(t *testing.T) {
	// This test requires syslog to be available
	logger, err := New("test-convocate")
	if err != nil {
		t.Skipf("syslog not available: %v", err)
	}
	defer logger.Close()

	// Should not panic
	logger.Info("test info message")
	logger.Infof("test info %s", "formatted")
	logger.Error("test error message")
	logger.Errorf("test error %s", "formatted")
	logger.Warning("test warning message")
	logger.Warningf("test warning %s", "formatted")
}

func TestNewWithDialer_Success(t *testing.T) {
	dialer := func(priority syslog.Priority, tag string) (*syslog.Writer, error) {
		return syslog.New(priority, tag)
	}

	logger, err := NewWithDialer("test-convocate", dialer)
	if err != nil {
		t.Skipf("syslog not available: %v", err)
	}
	defer logger.Close()

	logger.Info("test message via custom dialer")
}

func TestNewWithDialer_Failure(t *testing.T) {
	dialer := func(priority syslog.Priority, tag string) (*syslog.Writer, error) {
		return nil, &syslogError{msg: "connection refused"}
	}

	_, err := NewWithDialer("test", dialer)
	if err == nil {
		t.Error("expected error from failing dialer, got nil")
	}
}

type syslogError struct {
	msg string
}

func (e *syslogError) Error() string {
	return e.msg
}

func TestDefaultDialer(t *testing.T) {
	w, err := DefaultDialer(syslog.LOG_INFO|syslog.LOG_USER, "test")
	if err != nil {
		t.Skipf("syslog not available: %v", err)
	}
	_ = w.Close()
}
