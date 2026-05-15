package protocol

import (
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/uuid"
)

func TestStatusTransitionRequestValidate(t *testing.T) {
	validRequest := StatusTransitionRequest{
		HostID:      "host-1",
		ContainerID: "container-abc",
		JobID:       uuid.MustNew(),
		FromState:   JobClaimed,
		ToState:     JobRunning,
		Timestamp:   time.Now(),
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

	t.Run("missing container_id", func(t *testing.T) {
		r := validRequest
		r.ContainerID = ""
		err := r.Validate()
		if err == nil {
			t.Error("expected error for missing container_id")
		}
	})

	t.Run("zero job_id", func(t *testing.T) {
		r := validRequest
		r.JobID = uuid.UUID{}
		err := r.Validate()
		if err == nil {
			t.Error("expected error for zero job_id")
		}
	})

	t.Run("invalid from_state", func(t *testing.T) {
		r := validRequest
		r.FromState = "bogus"
		err := r.Validate()
		if err == nil {
			t.Error("expected error for invalid from_state")
		}
	})

	t.Run("invalid to_state", func(t *testing.T) {
		r := validRequest
		r.ToState = "bogus"
		err := r.Validate()
		if err == nil {
			t.Error("expected error for invalid to_state")
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

	t.Run("invalid transition", func(t *testing.T) {
		r := validRequest
		r.FromState = JobComplete
		r.ToState = JobRunning
		err := r.Validate()
		if err == nil {
			t.Error("expected error for invalid transition complete->running")
		}
	})

	t.Run("with optional fields", func(t *testing.T) {
		r := validRequest
		r.Reason = "tests passed"
		r.PullURL = "https://github.com/org/repo/pull/1"
		err := r.Validate()
		if err != nil {
			t.Errorf("Validate() unexpected error with optional fields: %v", err)
		}
	})
}
