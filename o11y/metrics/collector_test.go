package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Unit tests — /proc parsers
// ---------------------------------------------------------------------------

func TestReadLoadAvg(t *testing.T) {
	// Create a fake /proc/loadavg
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "loadavg"), []byte("1.23 4.56 7.89 3/456 12345\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	old := procPrefix
	procPrefix = dir
	defer func() { procPrefix = old }()

	la, err := readLoadAvg()
	if err != nil {
		t.Fatalf("readLoadAvg: %v", err)
	}
	if la.One != 1.23 {
		t.Errorf("One = %f, want 1.23", la.One)
	}
	if la.Five != 4.56 {
		t.Errorf("Five = %f, want 4.56", la.Five)
	}
	if la.Fifteen != 7.89 {
		t.Errorf("Fifteen = %f, want 7.89", la.Fifteen)
	}
}

func TestReadLoadAvg_MissingFile(t *testing.T) {
	old := procPrefix
	procPrefix = t.TempDir() // empty dir, no loadavg file
	defer func() { procPrefix = old }()

	_, err := readLoadAvg()
	if err == nil {
		t.Error("expected error for missing /proc/loadavg")
	}
}

func TestReadMemInfo(t *testing.T) {
	dir := t.TempDir()
	content := `MemTotal:       16384000 kB
MemFree:         2048000 kB
MemAvailable:    8192000 kB
Buffers:          512000 kB
Cached:          4096000 kB
SwapTotal:       4096000 kB
SwapFree:        3072000 kB
`
	err := os.WriteFile(filepath.Join(dir, "meminfo"), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	old := procPrefix
	procPrefix = dir
	defer func() { procPrefix = old }()

	mi, err := readMemInfo()
	if err != nil {
		t.Fatalf("readMemInfo: %v", err)
	}
	if mi.MemTotal != 16384000*1024 {
		t.Errorf("MemTotal = %d, want %d", mi.MemTotal, 16384000*1024)
	}
	if mi.MemAvailable != 8192000*1024 {
		t.Errorf("MemAvailable = %d, want %d", mi.MemAvailable, 8192000*1024)
	}
	if mi.SwapTotal != 4096000*1024 {
		t.Errorf("SwapTotal = %d, want %d", mi.SwapTotal, 4096000*1024)
	}
	if mi.SwapFree != 3072000*1024 {
		t.Errorf("SwapFree = %d, want %d", mi.SwapFree, 3072000*1024)
	}
}

func TestReadMemInfo_MissingFile(t *testing.T) {
	old := procPrefix
	procPrefix = t.TempDir()
	defer func() { procPrefix = old }()

	_, err := readMemInfo()
	if err == nil {
		t.Error("expected error for missing /proc/meminfo")
	}
}

func TestReadUptime(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "uptime"), []byte("123456.78 234567.89\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	old := procPrefix
	procPrefix = dir
	defer func() { procPrefix = old }()

	uptime, err := readUptime()
	if err != nil {
		t.Fatalf("readUptime: %v", err)
	}
	if uptime != 123456 {
		t.Errorf("uptime = %d, want 123456", uptime)
	}
}

func TestReadCPUCount(t *testing.T) {
	dir := t.TempDir()
	content := `processor	: 0
model name	: Intel Xeon
processor	: 1
model name	: Intel Xeon
processor	: 2
model name	: Intel Xeon
processor	: 3
model name	: Intel Xeon
`
	err := os.WriteFile(filepath.Join(dir, "cpuinfo"), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	old := procPrefix
	procPrefix = dir
	defer func() { procPrefix = old }()

	count := readCPUCount()
	if count != 4 {
		t.Errorf("cpuCount = %d, want 4", count)
	}
}

func TestReadCPUCount_MissingFile(t *testing.T) {
	old := procPrefix
	procPrefix = t.TempDir()
	defer func() { procPrefix = old }()

	count := readCPUCount()
	if count != 0 {
		t.Errorf("cpuCount = %d, want 0 for missing file", count)
	}
}

// ---------------------------------------------------------------------------
// Unit test — collectAll assembles a complete report
// ---------------------------------------------------------------------------

func TestCollectAll(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "loadavg"), []byte("0.50 0.40 0.30 1/100 999\n"), 0644)
	os.WriteFile(filepath.Join(dir, "meminfo"), []byte("MemTotal: 8000000 kB\nMemAvailable: 4000000 kB\nSwapTotal: 2000000 kB\nSwapFree: 1500000 kB\n"), 0644)
	os.WriteFile(filepath.Join(dir, "uptime"), []byte("86400.00 172800.00\n"), 0644)
	os.WriteFile(filepath.Join(dir, "cpuinfo"), []byte("processor\t: 0\nprocessor\t: 1\n"), 0644)

	old := procPrefix
	procPrefix = dir
	defer func() { procPrefix = old }()

	report := collectAll("test-node")
	if report.NodeName != "test-node" {
		t.Errorf("NodeName = %q, want %q", report.NodeName, "test-node")
	}
	if report.LoadAvg.One != 0.50 {
		t.Errorf("LoadAvg.One = %f, want 0.50", report.LoadAvg.One)
	}
	if report.MemTotalBytes != 8000000*1024 {
		t.Errorf("MemTotalBytes = %d, want %d", report.MemTotalBytes, 8000000*1024)
	}
	memUsed := report.MemTotalBytes - 4000000*1024
	if report.MemUsedBytes != memUsed {
		t.Errorf("MemUsedBytes = %d, want %d", report.MemUsedBytes, memUsed)
	}
	if report.UptimeSeconds != 86400 {
		t.Errorf("UptimeSeconds = %d, want 86400", report.UptimeSeconds)
	}
	if report.CPUCount != 2 {
		t.Errorf("CPUCount = %d, want 2", report.CPUCount)
	}
}

// ---------------------------------------------------------------------------
// Integration test — HTTP POST to a mock API server
// ---------------------------------------------------------------------------

func TestMetricsPostToAPI(t *testing.T) {
	var received MetricsReport
	var gotAuth string

	// Start a mock API server that captures the posted report
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/nmgr/metrics" {
			t.Errorf("path = %s, want /api/v1/nmgr/metrics", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", ct)
		}
		gotAuth = r.Header.Get("Authorization")

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	// Set up fake /proc
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "loadavg"), []byte("1.00 2.00 3.00 1/1 1\n"), 0644)
	os.WriteFile(filepath.Join(dir, "meminfo"), []byte("MemTotal: 1000 kB\nMemAvailable: 500 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\n"), 0644)
	os.WriteFile(filepath.Join(dir, "uptime"), []byte("100.0 200.0\n"), 0644)
	os.WriteFile(filepath.Join(dir, "cpuinfo"), []byte("processor\t: 0\n"), 0644)

	old := procPrefix
	procPrefix = dir
	defer func() { procPrefix = old }()

	// Collect and POST
	report := collectAll("integration-test")
	report.Timestamp = "2026-06-30T00:00:00Z"

	body, _ := json.Marshal(report)
	req, _ := http.NewRequest("POST", server.URL+"/api/v1/nmgr/metrics", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key-123")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 204 {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if received.NodeName != "integration-test" {
		t.Errorf("received.NodeName = %q, want %q", received.NodeName, "integration-test")
	}
	if received.LoadAvg.One != 1.00 {
		t.Errorf("received.LoadAvg.One = %f, want 1.00", received.LoadAvg.One)
	}
	if gotAuth != "Bearer test-key-123" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-key-123")
	}
}

// ---------------------------------------------------------------------------
// E2E test — full collect → marshal → verify JSON structure
// ---------------------------------------------------------------------------

func TestCollectAllProducesValidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "loadavg"), []byte("0.10 0.20 0.30 1/1 1\n"), 0644)
	os.WriteFile(filepath.Join(dir, "meminfo"), []byte("MemTotal: 2000 kB\nMemAvailable: 1000 kB\nSwapTotal: 500 kB\nSwapFree: 250 kB\n"), 0644)
	os.WriteFile(filepath.Join(dir, "uptime"), []byte("3600.0 7200.0\n"), 0644)
	os.WriteFile(filepath.Join(dir, "cpuinfo"), []byte("processor\t: 0\nprocessor\t: 1\n"), 0644)

	old := procPrefix
	procPrefix = dir
	defer func() { procPrefix = old }()

	report := collectAll("e2e-node")
	report.Timestamp = "2026-06-30T12:00:00Z"

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	// Verify the JSON has all expected fields
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	expectedFields := []string{
		"nodeName", "loadAvg", "memUsedBytes", "memTotalBytes",
		"swapUsedBytes", "swapTotalBytes", "diskUsedBytes", "diskTotalBytes",
		"uptimeSeconds", "kubeletVersion", "cpuCount", "timestamp",
	}
	for _, field := range expectedFields {
		if _, ok := m[field]; !ok {
			t.Errorf("missing field %q in JSON output", field)
		}
	}

	// Verify loadAvg has sub-fields
	la, ok := m["loadAvg"].(map[string]interface{})
	if !ok {
		t.Fatal("loadAvg is not an object")
	}
	for _, sub := range []string{"one", "five", "fifteen"} {
		if _, ok := la[sub]; !ok {
			t.Errorf("missing loadAvg.%s", sub)
		}
	}

	// Verify no negative values (except disk which might be 0 in test)
	if report.MemUsedBytes < 0 {
		t.Errorf("MemUsedBytes = %d, must not be negative", report.MemUsedBytes)
	}
	if report.SwapUsedBytes < 0 {
		t.Errorf("SwapUsedBytes = %d, must not be negative", report.SwapUsedBytes)
	}
}
