package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m.startTime.IsZero() {
		t.Error("startTime should not be zero")
	}
}

func TestMetrics_AtomicCounters(t *testing.T) {
	m := NewMetrics()
	m.StdinBytes.Add(100)
	m.StdoutBytes.Add(200)
	m.StderrBytes.Add(50)
	m.StdinMessages.Add(5)
	m.StdoutMessages.Add(10)
	m.StderrMessages.Add(3)
	m.ClaudeRestarts.Add(2)
	m.ActiveConnections.Add(1)

	if m.StdinBytes.Load() != 100 {
		t.Errorf("StdinBytes = %d, want 100", m.StdinBytes.Load())
	}
	if m.StdoutBytes.Load() != 200 {
		t.Errorf("StdoutBytes = %d, want 200", m.StdoutBytes.Load())
	}
	if m.StderrBytes.Load() != 50 {
		t.Errorf("StderrBytes = %d, want 50", m.StderrBytes.Load())
	}
	if m.ClaudeRestarts.Load() != 2 {
		t.Errorf("ClaudeRestarts = %d, want 2", m.ClaudeRestarts.Load())
	}
}

func TestMetrics_Snapshot(t *testing.T) {
	m := NewMetrics()
	m.StdinBytes.Add(42)
	m.StdoutMessages.Add(7)

	snap := m.Snapshot("1.0.0", "0.2.169", "pod-1", "node-1", 60*time.Second)

	if snap.WrapperVersion != "1.0.0" {
		t.Errorf("WrapperVersion = %q, want %q", snap.WrapperVersion, "1.0.0")
	}
	if snap.ClaudeCodeVersion != "0.2.169" {
		t.Errorf("ClaudeCodeVersion = %q, want %q", snap.ClaudeCodeVersion, "0.2.169")
	}
	if snap.StdinBytes != 42 {
		t.Errorf("StdinBytes = %d, want 42", snap.StdinBytes)
	}
	if snap.StdoutMessages != 7 {
		t.Errorf("StdoutMessages = %d, want 7", snap.StdoutMessages)
	}
	if snap.ClaudeUptime != 60 {
		t.Errorf("ClaudeUptime = %d, want 60", snap.ClaudeUptime)
	}
	if snap.PodName != "pod-1" {
		t.Errorf("PodName = %q, want %q", snap.PodName, "pod-1")
	}
	if snap.UptimeSeconds < 0 {
		t.Errorf("UptimeSeconds = %d, should be >= 0", snap.UptimeSeconds)
	}
}

func TestMetrics_SnapshotJSON(t *testing.T) {
	m := NewMetrics()
	m.StdinBytes.Add(10)

	data, err := m.SnapshotJSON("v1", "v2", "pod", "node", 0)
	if err != nil {
		t.Fatalf("SnapshotJSON: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	expectedFields := []string{
		"stdinBytes", "stdoutBytes", "stderrBytes",
		"stdinMessages", "stdoutMessages", "stderrMessages",
		"claudeRestarts", "wrapperVersion", "claudeCodeVersion",
		"uptimeSeconds", "claudeUptimeSeconds", "activeConnections",
		"podName", "nodeName",
	}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("missing field %q in JSON", field)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0d0h0m"},
		{25 * time.Hour, "1d1h0m"},
		{90 * time.Minute, "0d1h30m"},
		{48*time.Hour + 30*time.Minute, "2d0h30m"},
	}
	for _, tt := range tests {
		got := FormatDuration(tt.d)
		if got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
