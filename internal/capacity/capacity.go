// Package capacity enforces system-resource limits before starting new
// claude-shell containers. If the host is already above the configured CPU or
// memory usage threshold, new containers are refused so they do not push the
// system into instability. Already-running containers are not affected.
package capacity

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultThreshold is the percentage (0-100) of CPU or memory above which new
// containers are refused.
const DefaultThreshold = 80.0

// MemoryUsagePercent returns the current memory usage as a percentage of total
// memory. Uses /proc/meminfo on Linux.
var MemoryUsagePercent = defaultMemoryUsagePercent

// CPUUsagePercent returns the current CPU usage over a short sampling window.
// Uses /proc/stat on Linux.
var CPUUsagePercent = defaultCPUUsagePercent

// Check verifies the system has capacity to start a new container. It returns
// nil when CPU usage AND memory usage are both below threshold, and a
// descriptive error otherwise. Non-fatal errors reading /proc (e.g. on
// non-Linux hosts or with unusual procfs contents) are treated as "no info"
// and the check is allowed to pass — the intent is to protect against known
// overload, not to block when the signal is missing.
func Check(threshold float64) error {
	if mem, err := MemoryUsagePercent(); err == nil && mem >= threshold {
		return fmt.Errorf("system memory usage is %.1f%% (>=%.0f%% cap); refusing to start a new container", mem, threshold)
	}
	if cpu, err := CPUUsagePercent(); err == nil && cpu >= threshold {
		return fmt.Errorf("system CPU usage is %.1f%% (>=%.0f%% cap); refusing to start a new container", cpu, threshold)
	}
	return nil
}

// memInfoReader is overridable for tests.
var memInfoReader = func() (io.Reader, func() error, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

func defaultMemoryUsagePercent() (float64, error) {
	r, closeFn, err := memInfoReader()
	if err != nil {
		return 0, err
	}
	if closeFn != nil {
		defer closeFn()
	}

	var total, avail int64
	haveTotal, haveAvail := false, false
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total, err = parseKBLine(line)
			if err != nil {
				return 0, err
			}
			haveTotal = true
		case strings.HasPrefix(line, "MemAvailable:"):
			avail, err = parseKBLine(line)
			if err != nil {
				return 0, err
			}
			haveAvail = true
		}
		if haveTotal && haveAvail {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if !haveTotal || !haveAvail || total <= 0 {
		return 0, fmt.Errorf("unable to parse /proc/meminfo")
	}
	used := total - avail
	return float64(used) * 100.0 / float64(total), nil
}

func parseKBLine(line string) (int64, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, fmt.Errorf("malformed meminfo line: %q", line)
	}
	return strconv.ParseInt(fields[1], 10, 64)
}

// cpuSampleInterval is the delta between CPU samples.
var cpuSampleInterval = 100 * time.Millisecond

func defaultCPUUsagePercent() (float64, error) {
	a, err := readCPUTotals()
	if err != nil {
		return 0, err
	}
	time.Sleep(cpuSampleInterval)
	b, err := readCPUTotals()
	if err != nil {
		return 0, err
	}
	totalA := a.total()
	totalB := b.total()
	if totalB <= totalA {
		return 0, fmt.Errorf("cpu totals did not advance")
	}
	idleDelta := float64(b.idle - a.idle)
	totalDelta := float64(totalB - totalA)
	busy := totalDelta - idleDelta
	if busy < 0 {
		busy = 0
	}
	return busy * 100.0 / totalDelta, nil
}

type cpuTotals struct {
	user, nice, system, idle, iowait, irq, softirq, steal int64
}

func (c cpuTotals) total() int64 {
	return c.user + c.nice + c.system + c.idle + c.iowait + c.irq + c.softirq + c.steal
}

// cpuStatReader is overridable for tests.
var cpuStatReader = func() ([]byte, error) { return os.ReadFile("/proc/stat") }

func readCPUTotals() (cpuTotals, error) {
	data, err := cpuStatReader()
	if err != nil {
		return cpuTotals{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		// "cpu", user, nice, system, idle, iowait, irq, softirq, steal, ...
		if len(fields) < 8 {
			return cpuTotals{}, fmt.Errorf("malformed /proc/stat cpu line: %q", line)
		}
		vals := make([]int64, 8)
		for i := 0; i < 8; i++ {
			v, err := strconv.ParseInt(fields[i+1], 10, 64)
			if err != nil {
				return cpuTotals{}, fmt.Errorf("parse /proc/stat field %d: %w", i, err)
			}
			vals[i] = v
		}
		return cpuTotals{
			user: vals[0], nice: vals[1], system: vals[2], idle: vals[3],
			iowait: vals[4], irq: vals[5], softirq: vals[6], steal: vals[7],
		}, nil
	}
	return cpuTotals{}, fmt.Errorf("no aggregate 'cpu ' line in /proc/stat")
}
