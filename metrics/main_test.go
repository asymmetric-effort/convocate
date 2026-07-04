package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// parseConfig tests
// ---------------------------------------------------------------------------

func TestRunFromEnv_Success(t *testing.T) {
	cleanup := setupFakeProc(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	t.Setenv("NODE_NAME", "env-test-node")
	t.Setenv("API_URL", server.URL)
	t.Setenv("METRICS_API_KEY", "")

	stopCh := make(chan struct{})
	done := make(chan struct{})
	go func() {
		runFromEnv(50*time.Millisecond, stopCh)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	close(stopCh)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runFromEnv did not stop")
	}
}

func TestParseConfig_WithNodeName(t *testing.T) {
	t.Setenv("NODE_NAME", "test-node-1")
	t.Setenv("API_URL", "http://localhost:9999")
	t.Setenv("METRICS_API_KEY", "secret-key")

	cfg, err := parseConfig()
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.nodeName != "test-node-1" {
		t.Errorf("nodeName = %q, want %q", cfg.nodeName, "test-node-1")
	}
	if cfg.apiURL != "http://localhost:9999" {
		t.Errorf("apiURL = %q, want %q", cfg.apiURL, "http://localhost:9999")
	}
	if cfg.metricsKey != "secret-key" {
		t.Errorf("metricsKey = %q, want %q", cfg.metricsKey, "secret-key")
	}
	if cfg.endpoint != "http://localhost:9999/api/v1/nmgr/metrics" {
		t.Errorf("endpoint = %q, want %q", cfg.endpoint, "http://localhost:9999/api/v1/nmgr/metrics")
	}
}

func TestParseConfig_DefaultAPIURL(t *testing.T) {
	t.Setenv("NODE_NAME", "node-x")
	t.Setenv("API_URL", "")
	t.Setenv("METRICS_API_KEY", "")

	cfg, err := parseConfig()
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.apiURL != "http://convocate-api.convocate.svc:8443" {
		t.Errorf("apiURL = %q, want default", cfg.apiURL)
	}
	if cfg.metricsKey != "" {
		t.Errorf("metricsKey = %q, want empty", cfg.metricsKey)
	}
}

func TestParseConfig_MissingNodeName(t *testing.T) {
	t.Setenv("NODE_NAME", "")

	_, err := parseConfig()
	if err == nil {
		t.Error("expected error when NODE_NAME is missing")
	}
}

// ---------------------------------------------------------------------------
// postMetrics tests
// ---------------------------------------------------------------------------

func setupFakeProc(t *testing.T) (cleanup func()) {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "loadavg"), []byte("0.50 0.40 0.30 1/100 999\n"), 0644)
	os.WriteFile(filepath.Join(dir, "meminfo"), []byte("MemTotal: 8000 kB\nMemAvailable: 4000 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\n"), 0644)
	os.WriteFile(filepath.Join(dir, "uptime"), []byte("3600.0 7200.0\n"), 0644)
	os.WriteFile(filepath.Join(dir, "cpuinfo"), []byte("processor\t: 0\n"), 0644)

	old := procPrefix
	procPrefix = dir
	return func() { procPrefix = old }
}

func TestPostMetrics_Success(t *testing.T) {
	cleanup := setupFakeProc(t)
	defer cleanup()

	var gotContentType, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := metricsConfig{
		nodeName:   "test-node",
		apiURL:     server.URL,
		metricsKey: "test-key",
		endpoint:   server.URL + "/api/v1/nmgr/metrics",
	}
	postMetrics(cfg)

	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotContentType, "application/json")
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-key")
	}
}

func TestPostMetrics_NoAuthKey(t *testing.T) {
	cleanup := setupFakeProc(t)
	defer cleanup()

	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := metricsConfig{
		nodeName: "node-no-key",
		endpoint: server.URL + "/api/v1/nmgr/metrics",
	}
	postMetrics(cfg)

	if gotAuth != "" {
		t.Errorf("Authorization should be empty, got %q", gotAuth)
	}
}

func TestPostMetrics_ServerError(t *testing.T) {
	cleanup := setupFakeProc(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := metricsConfig{
		nodeName:   "node-err",
		metricsKey: "key",
		endpoint:   server.URL + "/api/v1/nmgr/metrics",
	}
	// Should not panic, just log
	postMetrics(cfg)
}

func TestPostMetrics_BadEndpoint(t *testing.T) {
	cleanup := setupFakeProc(t)
	defer cleanup()

	cfg := metricsConfig{
		nodeName:   "node",
		metricsKey: "key",
		endpoint:   "http://127.0.0.1:1/bad",
	}
	// Should not panic, just log
	postMetrics(cfg)
}

func TestPostMetrics_InvalidURL(t *testing.T) {
	cleanup := setupFakeProc(t)
	defer cleanup()

	cfg := metricsConfig{
		nodeName: "node",
		endpoint: "://invalid-url", // will cause http.NewRequest to fail
	}
	// Should not panic, just log the request error
	postMetrics(cfg)
}

// ---------------------------------------------------------------------------
// run() — the main loop with stopCh
// ---------------------------------------------------------------------------

func TestRun_StopsOnSignal(t *testing.T) {
	cleanup := setupFakeProc(t)
	defer cleanup()

	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := metricsConfig{
		nodeName: "run-test",
		apiURL:   server.URL,
		endpoint: server.URL + "/api/v1/nmgr/metrics",
	}

	stopCh := make(chan struct{})
	done := make(chan struct{})
	go func() {
		run(cfg, 50*time.Millisecond, stopCh)
		close(done)
	}()

	// Let it run at least one iteration
	time.Sleep(100 * time.Millisecond)
	close(stopCh)

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("run did not stop within timeout")
	}

	if requestCount < 1 {
		t.Errorf("expected at least 1 request, got %d", requestCount)
	}
}

// ---------------------------------------------------------------------------
// readDiskUsage — test with a real path (success) and error path
// ---------------------------------------------------------------------------

func TestReadDiskUsage_Success(t *testing.T) {
	old := rootPrefix
	rootPrefix = t.TempDir()
	defer func() { rootPrefix = old }()

	du, err := readDiskUsage()
	if err != nil {
		t.Fatalf("readDiskUsage: %v", err)
	}
	if du.TotalBytes <= 0 {
		t.Errorf("TotalBytes = %d, want > 0", du.TotalBytes)
	}
	if du.UsedBytes < 0 {
		t.Errorf("UsedBytes = %d, must not be negative", du.UsedBytes)
	}
	if du.UsedBytes > du.TotalBytes {
		t.Errorf("UsedBytes (%d) > TotalBytes (%d)", du.UsedBytes, du.TotalBytes)
	}
}

func TestReadDiskUsage_NonexistentPath(t *testing.T) {
	old := rootPrefix
	rootPrefix = "/nonexistent/path/that/does/not/exist"
	defer func() { rootPrefix = old }()

	_, err := readDiskUsage()
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

// ---------------------------------------------------------------------------
// readKubeletVersion
// ---------------------------------------------------------------------------

func TestReadKubeletVersion_FromEnv(t *testing.T) {
	t.Setenv("KUBELET_VERSION", "v1.30.2")
	got := readKubeletVersion()
	if got != "v1.30.2" {
		t.Errorf("readKubeletVersion = %q, want %q", got, "v1.30.2")
	}
}

func TestReadKubeletVersion_NoEnvNoFile(t *testing.T) {
	t.Setenv("KUBELET_VERSION", "")
	old := rootPrefix
	rootPrefix = t.TempDir()
	defer func() { rootPrefix = old }()

	got := readKubeletVersion()
	if got != "" {
		t.Errorf("readKubeletVersion = %q, want empty string", got)
	}
}

func TestReadKubeletVersion_NoEnvWithFile(t *testing.T) {
	t.Setenv("KUBELET_VERSION", "")
	dir := t.TempDir()

	kubeletDir := filepath.Join(dir, "var", "lib", "kubelet")
	os.MkdirAll(kubeletDir, 0755)
	os.WriteFile(filepath.Join(kubeletDir, "config.yaml"), []byte("apiVersion: kubelet.config.k8s.io/v1beta1\n"), 0644)

	old := rootPrefix
	rootPrefix = dir
	defer func() { rootPrefix = old }()

	got := readKubeletVersion()
	if got != "" {
		t.Errorf("readKubeletVersion = %q, want empty (file exists but version comes from API)", got)
	}
}

// ---------------------------------------------------------------------------
// envOrDefault
// ---------------------------------------------------------------------------

func TestEnvOrDefault_Set(t *testing.T) {
	t.Setenv("TEST_ENV_METRIC_X", "custom-value")
	got := envOrDefault("TEST_ENV_METRIC_X", "default")
	if got != "custom-value" {
		t.Errorf("envOrDefault = %q, want %q", got, "custom-value")
	}
}

func TestEnvOrDefault_Unset(t *testing.T) {
	os.Unsetenv("TEST_ENV_METRIC_Y")
	got := envOrDefault("TEST_ENV_METRIC_Y", "fallback")
	if got != "fallback" {
		t.Errorf("envOrDefault = %q, want %q", got, "fallback")
	}
}

// ---------------------------------------------------------------------------
// readLoadAvg — malformed content
// ---------------------------------------------------------------------------

func TestReadLoadAvg_MalformedContent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "loadavg"), []byte("not a valid loadavg file\n"), 0644)

	old := procPrefix
	procPrefix = dir
	defer func() { procPrefix = old }()

	_, err := readLoadAvg()
	if err == nil {
		t.Error("expected error for malformed loadavg")
	}
}

// ---------------------------------------------------------------------------
// readUptime — malformed content and missing file
// ---------------------------------------------------------------------------

func TestReadUptime_MissingFile(t *testing.T) {
	old := procPrefix
	procPrefix = t.TempDir()
	defer func() { procPrefix = old }()

	_, err := readUptime()
	if err == nil {
		t.Error("expected error for missing /proc/uptime")
	}
}

func TestReadUptime_MalformedContent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "uptime"), []byte("not_a_number idle\n"), 0644)

	old := procPrefix
	procPrefix = dir
	defer func() { procPrefix = old }()

	uptime, err := readUptime()
	if err != nil {
		t.Fatalf("readUptime returned unexpected error: %v", err)
	}
	if uptime != 0 {
		t.Errorf("uptime = %d, want 0 for malformed input", uptime)
	}
}

// ---------------------------------------------------------------------------
// readMemInfo — edge cases
// ---------------------------------------------------------------------------

func TestReadMemInfo_EmptyContent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "meminfo"), []byte(""), 0644)

	old := procPrefix
	procPrefix = dir
	defer func() { procPrefix = old }()

	mi, err := readMemInfo()
	if err != nil {
		t.Fatalf("readMemInfo: %v", err)
	}
	if mi.MemTotal != 0 || mi.MemAvailable != 0 || mi.SwapTotal != 0 || mi.SwapFree != 0 {
		t.Errorf("all values should be zero for empty meminfo: %+v", mi)
	}
}

func TestReadMemInfo_ShortLines(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "meminfo"), []byte("ShortField\nMemTotal: 1000 kB\n"), 0644)

	old := procPrefix
	procPrefix = dir
	defer func() { procPrefix = old }()

	mi, err := readMemInfo()
	if err != nil {
		t.Fatalf("readMemInfo: %v", err)
	}
	if mi.MemTotal != 1000*1024 {
		t.Errorf("MemTotal = %d, want %d", mi.MemTotal, 1000*1024)
	}
}

// ---------------------------------------------------------------------------
// collectAll — with failing sub-collectors
// ---------------------------------------------------------------------------

func TestCollectAll_AllFailing(t *testing.T) {
	dir := t.TempDir()
	old := procPrefix
	procPrefix = dir
	defer func() { procPrefix = old }()

	oldRoot := rootPrefix
	rootPrefix = "/nonexistent/path/does/not/exist"
	defer func() { rootPrefix = oldRoot }()

	t.Setenv("KUBELET_VERSION", "")

	report := collectAll("failing-node")
	if report.NodeName != "failing-node" {
		t.Errorf("NodeName = %q, want %q", report.NodeName, "failing-node")
	}
	if report.LoadAvg.One != 0 || report.LoadAvg.Five != 0 || report.LoadAvg.Fifteen != 0 {
		t.Errorf("LoadAvg should be zero: %+v", report.LoadAvg)
	}
	if report.MemTotalBytes != 0 || report.MemUsedBytes != 0 {
		t.Errorf("Mem should be zero: total=%d, used=%d", report.MemTotalBytes, report.MemUsedBytes)
	}
	if report.UptimeSeconds != 0 {
		t.Errorf("UptimeSeconds = %d, want 0", report.UptimeSeconds)
	}
	if report.CPUCount != 0 {
		t.Errorf("CPUCount = %d, want 0", report.CPUCount)
	}
	if report.DiskTotalBytes != 0 || report.DiskUsedBytes != 0 {
		t.Errorf("Disk should be zero: total=%d, used=%d", report.DiskTotalBytes, report.DiskUsedBytes)
	}
}

func TestCollectAll_WithDisk(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "loadavg"), []byte("0.10 0.20 0.30 1/1 1\n"), 0644)
	os.WriteFile(filepath.Join(dir, "meminfo"), []byte("MemTotal: 1000 kB\nMemAvailable: 500 kB\nSwapTotal: 100 kB\nSwapFree: 50 kB\n"), 0644)
	os.WriteFile(filepath.Join(dir, "uptime"), []byte("1000.0 2000.0\n"), 0644)
	os.WriteFile(filepath.Join(dir, "cpuinfo"), []byte("processor\t: 0\nprocessor\t: 1\nprocessor\t: 2\n"), 0644)

	old := procPrefix
	procPrefix = dir
	defer func() { procPrefix = old }()

	oldRoot := rootPrefix
	rootPrefix = t.TempDir()
	defer func() { rootPrefix = oldRoot }()

	t.Setenv("KUBELET_VERSION", "v1.29.0")

	report := collectAll("disk-node")
	if report.DiskTotalBytes <= 0 {
		t.Errorf("DiskTotalBytes = %d, want > 0", report.DiskTotalBytes)
	}
	if report.KubeletVersion != "v1.29.0" {
		t.Errorf("KubeletVersion = %q, want %q", report.KubeletVersion, "v1.29.0")
	}
	if report.CPUCount != 3 {
		t.Errorf("CPUCount = %d, want 3", report.CPUCount)
	}
	if report.SwapUsedBytes != 50*1024 {
		t.Errorf("SwapUsedBytes = %d, want %d", report.SwapUsedBytes, 50*1024)
	}
}
