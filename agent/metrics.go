// Package main provides the convocate-agent-wrapper binary.
// This file implements thread-safe I/O metrics tracking.

package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

// Metrics tracks I/O counters and agent statistics using lock-free atomics.
type Metrics struct {
	StdinBytes        atomic.Int64
	StdoutBytes       atomic.Int64
	StderrBytes       atomic.Int64
	StdinMessages     atomic.Int64
	StdoutMessages    atomic.Int64
	StderrMessages    atomic.Int64
	ClaudeRestarts    atomic.Int32
	ActiveConnections atomic.Int32
	startTime         time.Time
}

// MetricsSnapshot is the JSON-serializable form of Metrics.
type MetricsSnapshot struct {
	StdinBytes        int64  `json:"stdinBytes"`
	StdoutBytes       int64  `json:"stdoutBytes"`
	StderrBytes       int64  `json:"stderrBytes"`
	StdinMessages     int64  `json:"stdinMessages"`
	StdoutMessages    int64  `json:"stdoutMessages"`
	StderrMessages    int64  `json:"stderrMessages"`
	ClaudeRestarts    int    `json:"claudeRestarts"`
	WrapperVersion    string `json:"wrapperVersion"`
	ClaudeCodeVersion string `json:"claudeCodeVersion"`
	UptimeSeconds     int64  `json:"uptimeSeconds"`
	ClaudeUptime      int64  `json:"claudeUptimeSeconds"`
	ActiveConnections int    `json:"activeConnections"`
	PodName           string `json:"podName"`
	NodeName          string `json:"nodeName"`
}

// NewMetrics creates a Metrics instance with the start time set to now.
func NewMetrics() *Metrics {
	return &Metrics{startTime: time.Now()}
}

// Snapshot returns the current metrics as a JSON-serializable struct.
func (m *Metrics) Snapshot(wrapperVersion, claudeVersion, podName, nodeName string, claudeUptime time.Duration) MetricsSnapshot {
	return MetricsSnapshot{
		StdinBytes:        m.StdinBytes.Load(),
		StdoutBytes:       m.StdoutBytes.Load(),
		StderrBytes:       m.StderrBytes.Load(),
		StdinMessages:     m.StdinMessages.Load(),
		StdoutMessages:    m.StdoutMessages.Load(),
		StderrMessages:    m.StderrMessages.Load(),
		ClaudeRestarts:    int(m.ClaudeRestarts.Load()),
		WrapperVersion:    wrapperVersion,
		ClaudeCodeVersion: claudeVersion,
		UptimeSeconds:     int64(time.Since(m.startTime).Seconds()),
		ClaudeUptime:      int64(claudeUptime.Seconds()),
		ActiveConnections: int(m.ActiveConnections.Load()),
		PodName:           podName,
		NodeName:          nodeName,
	}
}

// SnapshotJSON returns the metrics snapshot as JSON bytes.
func (m *Metrics) SnapshotJSON(wrapperVersion, claudeVersion, podName, nodeName string, claudeUptime time.Duration) ([]byte, error) {
	snap := m.Snapshot(wrapperVersion, claudeVersion, podName, nodeName, claudeUptime)
	return json.Marshal(snap)
}

// DetectClaudeVersion runs `claude --version` and parses the output.
// Returns empty string on error.
func DetectClaudeVersion() string {
	out, err := exec.Command("claude", "--version").Output()
	if err != nil {
		return ""
	}
	version := strings.TrimSpace(string(out))
	// Claude CLI may output "claude v1.0.0" or just "1.0.0"
	if idx := strings.LastIndex(version, " "); idx >= 0 {
		version = version[idx+1:]
	}
	return version
}

// FormatDuration returns a human-readable duration string.
func FormatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dd%dh%dm", days, hours, mins)
}
