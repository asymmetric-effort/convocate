package capacity

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func withMemReader(t *testing.T, fn func() (float64, error)) {
	t.Helper()
	orig := MemoryUsagePercent
	MemoryUsagePercent = fn
	t.Cleanup(func() { MemoryUsagePercent = orig })
}

func withCPUReader(t *testing.T, fn func() (float64, error)) {
	t.Helper()
	orig := CPUUsagePercent
	CPUUsagePercent = fn
	t.Cleanup(func() { CPUUsagePercent = orig })
}

func TestCheck_UnderThreshold(t *testing.T) {
	withMemReader(t, func() (float64, error) { return 25.0, nil })
	withCPUReader(t, func() (float64, error) { return 10.0, nil })
	if err := Check(80); err != nil {
		t.Errorf("Check returned error under threshold: %v", err)
	}
}

func TestCheck_MemoryAboveThreshold(t *testing.T) {
	withMemReader(t, func() (float64, error) { return 95.0, nil })
	withCPUReader(t, func() (float64, error) { return 10.0, nil })
	err := Check(80)
	if err == nil {
		t.Fatal("expected error when memory above threshold")
	}
	if !strings.Contains(err.Error(), "memory usage") {
		t.Errorf("error = %q, want mention of memory usage", err.Error())
	}
}

func TestCheck_CPUAboveThreshold(t *testing.T) {
	withMemReader(t, func() (float64, error) { return 10.0, nil })
	withCPUReader(t, func() (float64, error) { return 90.0, nil })
	err := Check(80)
	if err == nil {
		t.Fatal("expected error when CPU above threshold")
	}
	if !strings.Contains(err.Error(), "CPU usage") {
		t.Errorf("error = %q, want mention of CPU usage", err.Error())
	}
}

func TestCheck_AtExactlyThreshold_Refused(t *testing.T) {
	// The spec says ">= 80%" should refuse new starts.
	withMemReader(t, func() (float64, error) { return 80.0, nil })
	withCPUReader(t, func() (float64, error) { return 10.0, nil })
	if err := Check(80); err == nil {
		t.Error("expected error at exactly threshold")
	}
}

func TestCheck_ReadErrorsAllowPass(t *testing.T) {
	// When /proc readings fail, Check must not refuse — the cap is a safeguard,
	// not a hard block on hosts without the expected procfs entries.
	withMemReader(t, func() (float64, error) { return 0, errors.New("unavailable") })
	withCPUReader(t, func() (float64, error) { return 0, errors.New("unavailable") })
	if err := Check(80); err != nil {
		t.Errorf("Check should pass when readers return errors, got: %v", err)
	}
}

func TestDefaultMemoryUsagePercent_ReadsRealProc(t *testing.T) {
	// On a Linux test host this reads /proc/meminfo and yields a valid value.
	val, err := defaultMemoryUsagePercent()
	if err != nil {
		t.Skipf("skipping: /proc/meminfo unavailable: %v", err)
	}
	if val < 0 || val > 100 {
		t.Errorf("memory percent = %v, want 0-100", val)
	}
}

func TestDefaultCPUUsagePercent_ReadsRealProc(t *testing.T) {
	orig := cpuSampleInterval
	cpuSampleInterval = 10 * time.Millisecond
	t.Cleanup(func() { cpuSampleInterval = orig })
	val, err := defaultCPUUsagePercent()
	if err != nil {
		t.Skipf("skipping: /proc/stat unavailable: %v", err)
	}
	if val < 0 || val > 100 {
		t.Errorf("cpu percent = %v, want 0-100", val)
	}
}

func TestCPUReader_MalformedLine(t *testing.T) {
	orig := cpuStatReader
	cpuStatReader = func() ([]byte, error) { return []byte("cpu 1 2 3\n"), nil }
	t.Cleanup(func() { cpuStatReader = orig })
	_, err := readCPUTotals()
	if err == nil {
		t.Error("expected error for malformed cpu line")
	}
}

func TestCPUReader_NonNumeric(t *testing.T) {
	orig := cpuStatReader
	cpuStatReader = func() ([]byte, error) { return []byte("cpu a b c d e f g h\n"), nil }
	t.Cleanup(func() { cpuStatReader = orig })
	_, err := readCPUTotals()
	if err == nil {
		t.Error("expected parse error")
	}
}

func TestCPUReader_NoCPULine(t *testing.T) {
	orig := cpuStatReader
	cpuStatReader = func() ([]byte, error) { return []byte("unrelated 1 2 3\n"), nil }
	t.Cleanup(func() { cpuStatReader = orig })
	_, err := readCPUTotals()
	if err == nil {
		t.Error("expected error when no aggregate cpu line present")
	}
}

func TestCPUReader_ReadError(t *testing.T) {
	orig := cpuStatReader
	cpuStatReader = func() ([]byte, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { cpuStatReader = orig })
	_, err := readCPUTotals()
	if err == nil {
		t.Error("expected error from reader failure")
	}
}

func TestDefaultCPUUsagePercent_TwoSamplesUseDifferentTotals(t *testing.T) {
	// Swap reader to return monotonically increasing totals so the sample delta
	// computes deterministically.
	orig := cpuStatReader
	calls := 0
	cpuStatReader = func() ([]byte, error) {
		calls++
		switch calls {
		case 1:
			return []byte("cpu 100 0 0 900 0 0 0 0 0 0\n"), nil
		default:
			return []byte("cpu 200 0 0 1000 0 0 0 0 0 0\n"), nil
		}
	}
	t.Cleanup(func() { cpuStatReader = orig })

	origInt := cpuSampleInterval
	cpuSampleInterval = time.Millisecond
	t.Cleanup(func() { cpuSampleInterval = origInt })

	val, err := defaultCPUUsagePercent()
	if err != nil {
		t.Fatalf("defaultCPUUsagePercent failed: %v", err)
	}
	// user delta 100, idle delta 100, total delta 200 → busy=100, 50% expected.
	if val < 49 || val > 51 {
		t.Errorf("cpu percent = %v, want ~50", val)
	}
}

func TestDefaultCPUUsagePercent_NoProgressError(t *testing.T) {
	orig := cpuStatReader
	cpuStatReader = func() ([]byte, error) {
		return []byte("cpu 100 0 0 900 0 0 0 0\n"), nil
	}
	t.Cleanup(func() { cpuStatReader = orig })

	origInt := cpuSampleInterval
	cpuSampleInterval = time.Millisecond
	t.Cleanup(func() { cpuSampleInterval = origInt })

	_, err := defaultCPUUsagePercent()
	if err == nil {
		t.Error("expected error when totals do not advance")
	}
}

func TestDefaultCPUUsagePercent_FirstReadError(t *testing.T) {
	orig := cpuStatReader
	cpuStatReader = func() ([]byte, error) { return nil, errors.New("gone") }
	t.Cleanup(func() { cpuStatReader = orig })
	_, err := defaultCPUUsagePercent()
	if err == nil {
		t.Error("expected error from first read")
	}
}

func TestDefaultCPUUsagePercent_SecondReadError(t *testing.T) {
	orig := cpuStatReader
	calls := 0
	cpuStatReader = func() ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte("cpu 100 0 0 900 0 0 0 0\n"), nil
		}
		return nil, errors.New("gone")
	}
	t.Cleanup(func() { cpuStatReader = orig })
	origInt := cpuSampleInterval
	cpuSampleInterval = time.Millisecond
	t.Cleanup(func() { cpuSampleInterval = origInt })

	_, err := defaultCPUUsagePercent()
	if err == nil {
		t.Error("expected error from second read")
	}
}

// --- defaultMemoryUsagePercent error paths ---

func withMemInfoReader(t *testing.T, content string, readErr error) {
	t.Helper()
	orig := memInfoReader
	memInfoReader = func() (io.Reader, func() error, error) {
		if readErr != nil {
			return nil, nil, readErr
		}
		return strings.NewReader(content), nil, nil
	}
	t.Cleanup(func() { memInfoReader = orig })
}

func TestDefaultMemoryUsagePercent_OpenError(t *testing.T) {
	withMemInfoReader(t, "", errors.New("no file"))
	_, err := defaultMemoryUsagePercent()
	if err == nil {
		t.Error("expected error from reader")
	}
}

func TestDefaultMemoryUsagePercent_MissingFields(t *testing.T) {
	withMemInfoReader(t, "SomethingElse: 1024 kB\n", nil)
	_, err := defaultMemoryUsagePercent()
	if err == nil {
		t.Error("expected error when required fields are missing")
	}
}

func TestDefaultMemoryUsagePercent_BadTotal(t *testing.T) {
	withMemInfoReader(t, "MemTotal: not-a-number kB\nMemAvailable: 100 kB\n", nil)
	_, err := defaultMemoryUsagePercent()
	if err == nil {
		t.Error("expected error parsing MemTotal")
	}
}

func TestDefaultMemoryUsagePercent_BadAvail(t *testing.T) {
	withMemInfoReader(t, "MemTotal: 1024 kB\nMemAvailable: xyz kB\n", nil)
	_, err := defaultMemoryUsagePercent()
	if err == nil {
		t.Error("expected error parsing MemAvailable")
	}
}

func TestDefaultMemoryUsagePercent_ZeroTotal(t *testing.T) {
	withMemInfoReader(t, "MemTotal: 0 kB\nMemAvailable: 0 kB\n", nil)
	_, err := defaultMemoryUsagePercent()
	if err == nil {
		t.Error("expected error for zero total")
	}
}

func TestDefaultMemoryUsagePercent_Success(t *testing.T) {
	withMemInfoReader(t, "MemTotal: 1000 kB\nMemAvailable: 250 kB\n", nil)
	val, err := defaultMemoryUsagePercent()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val < 74.9 || val > 75.1 {
		t.Errorf("memory usage = %v, want ~75", val)
	}
}

func TestParseKBLine_Malformed(t *testing.T) {
	if _, err := parseKBLine("MemTotal:"); err == nil {
		t.Error("expected error for short line")
	}
	if _, err := parseKBLine("MemTotal: abc kB"); err == nil {
		t.Error("expected error for non-numeric value")
	}
	if _, err := parseKBLine("MemTotal: 1024 kB"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
