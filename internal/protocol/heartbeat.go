package protocol

import (
	"fmt"
	"time"
)

// HeartbeatRequest is the payload sent by a Dispatch Service to
// POST /v1/heartbeat every 15 seconds.
type HeartbeatRequest struct {
	HostID         string    `json:"host_id"`
	ContainerCount int       `json:"container_count"`
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryPercent  float64   `json:"memory_percent"`
	Timestamp      time.Time `json:"timestamp"`
}

// Validate checks that all required fields are present.
func (r HeartbeatRequest) Validate() error {
	if r.HostID == "" {
		return fmt.Errorf("protocol: heartbeat missing host_id")
	}
	if r.Timestamp.IsZero() {
		return fmt.Errorf("protocol: heartbeat missing timestamp")
	}
	if r.CPUPercent < 0 || r.CPUPercent > 100 {
		return fmt.Errorf("protocol: heartbeat cpu_percent out of range: %f", r.CPUPercent)
	}
	if r.MemoryPercent < 0 || r.MemoryPercent > 100 {
		return fmt.Errorf("protocol: heartbeat memory_percent out of range: %f", r.MemoryPercent)
	}
	if r.ContainerCount < 0 {
		return fmt.Errorf("protocol: heartbeat container_count negative: %d", r.ContainerCount)
	}
	return nil
}

// HeartbeatResponse is the response returned by POST /v1/heartbeat.
type HeartbeatResponse struct {
	Accepted bool `json:"accepted"`
}
