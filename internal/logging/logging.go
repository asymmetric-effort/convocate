// Package logging provides syslog-based logging for claude-shell.
package logging

import (
	"fmt"
	"log"
	"log/syslog"
)

// Logger wraps syslog with convenience methods.
type Logger struct {
	writer *syslog.Writer
}

// SyslogDialer abstracts syslog connection for testing.
type SyslogDialer func(priority syslog.Priority, tag string) (*syslog.Writer, error)

// DefaultDialer is the default syslog dialer.
func DefaultDialer(priority syslog.Priority, tag string) (*syslog.Writer, error) {
	return syslog.New(priority, tag)
}

// New creates a new Logger connected to syslog.
func New(tag string) (*Logger, error) {
	return NewWithDialer(tag, DefaultDialer)
}

// NewWithDialer creates a new Logger using the provided dialer.
func NewWithDialer(tag string, dial SyslogDialer) (*Logger, error) {
	w, err := dial(syslog.LOG_INFO|syslog.LOG_USER, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to syslog: %w", err)
	}
	return &Logger{writer: w}, nil
}

// Info logs an informational message.
func (l *Logger) Info(msg string) {
	if err := l.writer.Info(msg); err != nil {
		log.Printf("syslog info error: %v", err)
	}
}

// Infof logs a formatted informational message.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

// Error logs an error message.
func (l *Logger) Error(msg string) {
	if err := l.writer.Err(msg); err != nil {
		log.Printf("syslog error error: %v", err)
	}
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

// Warning logs a warning message.
func (l *Logger) Warning(msg string) {
	if err := l.writer.Warning(msg); err != nil {
		log.Printf("syslog warning error: %v", err)
	}
}

// Warningf logs a formatted warning message.
func (l *Logger) Warningf(format string, args ...interface{}) {
	l.Warning(fmt.Sprintf(format, args...))
}

// Close closes the syslog connection.
func (l *Logger) Close() error {
	return l.writer.Close()
}
