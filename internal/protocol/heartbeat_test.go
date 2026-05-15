package protocol

import (
	"testing"
	"time"
)

func TestHeartbeatRequestValidate(t *testing.T) {
	validRequest := HeartbeatRequest{
		HostID:         "host-1",
		ContainerCount: 3,
		CPUPercent:     45.5,
		MemoryPercent:  60.0,
		Timestamp:      time.Now(),
	}

	t.Run("valid request", func(t *testing.T) {
		err := validRequest.Validate()
		if err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("missing host_id", func(t *testing.T) {
		r := validRequest
		r.HostID = ""
		err := r.Validate()
		if err == nil {
			t.Error("expected error for missing host_id")
		}
	})

	t.Run("zero timestamp", func(t *testing.T) {
		r := validRequest
		r.Timestamp = time.Time{}
		err := r.Validate()
		if err == nil {
			t.Error("expected error for zero timestamp")
		}
	})

	t.Run("cpu percent negative", func(t *testing.T) {
		r := validRequest
		r.CPUPercent = -1.0
		err := r.Validate()
		if err == nil {
			t.Error("expected error for negative cpu_percent")
		}
	})

	t.Run("cpu percent over 100", func(t *testing.T) {
		r := validRequest
		r.CPUPercent = 101.0
		err := r.Validate()
		if err == nil {
			t.Error("expected error for cpu_percent > 100")
		}
	})

	t.Run("memory percent negative", func(t *testing.T) {
		r := validRequest
		r.MemoryPercent = -1.0
		err := r.Validate()
		if err == nil {
			t.Error("expected error for negative memory_percent")
		}
	})

	t.Run("memory percent over 100", func(t *testing.T) {
		r := validRequest
		r.MemoryPercent = 101.0
		err := r.Validate()
		if err == nil {
			t.Error("expected error for memory_percent > 100")
		}
	})

	t.Run("negative container count", func(t *testing.T) {
		r := validRequest
		r.ContainerCount = -1
		err := r.Validate()
		if err == nil {
			t.Error("expected error for negative container_count")
		}
	})

	t.Run("zero container count is valid", func(t *testing.T) {
		r := validRequest
		r.ContainerCount = 0
		err := r.Validate()
		if err != nil {
			t.Errorf("Validate() unexpected error for zero containers: %v", err)
		}
	})

	t.Run("boundary 100 percent values", func(t *testing.T) {
		r := validRequest
		r.CPUPercent = 100.0
		r.MemoryPercent = 100.0
		err := r.Validate()
		if err != nil {
			t.Errorf("Validate() unexpected error for 100%% values: %v", err)
		}
	})

	t.Run("boundary zero percent values", func(t *testing.T) {
		r := validRequest
		r.CPUPercent = 0.0
		r.MemoryPercent = 0.0
		err := r.Validate()
		if err != nil {
			t.Errorf("Validate() unexpected error for 0%% values: %v", err)
		}
	})
}
