// Package main implements the node-metrics collector.
// This file reads system metrics from /proc and filesystem stats.

package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// procPrefix is the mount point for the host's /proc filesystem.
// Defaults to "/host/proc" when running in a container with a
// hostPath volume mount.
var procPrefix = envOrDefault("PROC_PREFIX", "/host/proc")

// rootPrefix is the mount point for the host's root filesystem.
// Used for disk usage via Statfs.
var rootPrefix = envOrDefault("ROOT_PREFIX", "/host/root")

// envOrDefault returns the env var value or the fallback.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// LoadAvg holds 1m, 5m, 15m Linux load averages.
type LoadAvg struct {
	One     float64 `json:"one"`
	Five    float64 `json:"five"`
	Fifteen float64 `json:"fifteen"`
}

// MetricsReport is the JSON payload posted to the API.
type MetricsReport struct {
	NodeName       string  `json:"nodeName"`
	LoadAvg        LoadAvg `json:"loadAvg"`
	MemUsedBytes   int64   `json:"memUsedBytes"`
	MemTotalBytes  int64   `json:"memTotalBytes"`
	SwapUsedBytes  int64   `json:"swapUsedBytes"`
	SwapTotalBytes int64   `json:"swapTotalBytes"`
	DiskUsedBytes  int64   `json:"diskUsedBytes"`
	DiskTotalBytes int64   `json:"diskTotalBytes"`
	UptimeSeconds  int64   `json:"uptimeSeconds"`
	KubeletVersion string  `json:"kubeletVersion"`
	CPUCount       int     `json:"cpuCount"`
	Timestamp      string  `json:"timestamp"`
}

// readLoadAvg parses /proc/loadavg for 1m, 5m, 15m load averages.
func readLoadAvg() (LoadAvg, error) {
	data, err := os.ReadFile(procPrefix + "/loadavg")
	if err != nil {
		return LoadAvg{}, err
	}
	var one, five, fifteen float64
	_, err = fmt.Sscanf(string(data), "%f %f %f", &one, &five, &fifteen)
	if err != nil {
		return LoadAvg{}, err
	}
	return LoadAvg{One: one, Five: five, Fifteen: fifteen}, nil
}

// memInfo holds parsed values from /proc/meminfo.
type memInfo struct {
	MemTotal     int64
	MemAvailable int64
	SwapTotal    int64
	SwapFree     int64
}

// readMemInfo parses /proc/meminfo for memory and swap stats.
// Values in /proc/meminfo are in kB (1024 bytes).
func readMemInfo() (memInfo, error) {
	data, err := os.ReadFile(procPrefix + "/meminfo")
	if err != nil {
		return memInfo{}, err
	}
	var mi memInfo
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		var val int64
		fmt.Sscanf(fields[1], "%d", &val)
		val *= 1024 // kB → bytes
		switch fields[0] {
		case "MemTotal:":
			mi.MemTotal = val
		case "MemAvailable:":
			mi.MemAvailable = val
		case "SwapTotal:":
			mi.SwapTotal = val
		case "SwapFree:":
			mi.SwapFree = val
		}
	}
	return mi, nil
}

// readUptime parses /proc/uptime for system uptime in seconds.
func readUptime() (int64, error) {
	data, err := os.ReadFile(procPrefix + "/uptime")
	if err != nil {
		return 0, err
	}
	var uptime float64
	fmt.Sscanf(string(data), "%f", &uptime)
	return int64(uptime), nil
}

// readCPUCount counts processor entries in /proc/cpuinfo.
func readCPUCount() int {
	data, err := os.ReadFile(procPrefix + "/cpuinfo")
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "processor") {
			count++
		}
	}
	return count
}

// diskUsage holds filesystem space stats.
type diskUsage struct {
	TotalBytes int64
	UsedBytes  int64
}

// readDiskUsage uses Statfs on the mounted host root filesystem.
func readDiskUsage() (diskUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(rootPrefix, &stat); err != nil {
		return diskUsage{}, err
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bfree) * int64(stat.Bsize)
	return diskUsage{TotalBytes: total, UsedBytes: total - free}, nil
}

// readKubeletVersion attempts to read the kubelet version from the
// host filesystem.  Falls back to the KUBELET_VERSION env var.
func readKubeletVersion() string {
	if v := os.Getenv("KUBELET_VERSION"); v != "" {
		return v
	}
	// Try reading from the host's kubelet binary via /proc/version
	// or from dpkg info in the host root
	data, err := os.ReadFile(rootPrefix + "/var/lib/kubelet/config.yaml")
	if err == nil {
		// Not a great source for version, but it confirms kubelet exists.
		// The version will come from the API's K8s node info instead.
		_ = data
	}
	return ""
}

// collectAll gathers all system metrics into a MetricsReport.
func collectAll(nodeName string) MetricsReport {
	report := MetricsReport{
		NodeName:       nodeName,
		KubeletVersion: readKubeletVersion(),
	}

	if la, err := readLoadAvg(); err == nil {
		report.LoadAvg = la
	}

	if mi, err := readMemInfo(); err == nil {
		report.MemTotalBytes = mi.MemTotal
		report.MemUsedBytes = mi.MemTotal - mi.MemAvailable
		report.SwapTotalBytes = mi.SwapTotal
		report.SwapUsedBytes = mi.SwapTotal - mi.SwapFree
	}

	if uptime, err := readUptime(); err == nil {
		report.UptimeSeconds = uptime
	}

	report.CPUCount = readCPUCount()

	if du, err := readDiskUsage(); err == nil {
		report.DiskTotalBytes = du.TotalBytes
		report.DiskUsedBytes = du.UsedBytes
	}

	return report
}
