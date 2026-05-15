package protocol

import (
	"fmt"
	"time"

	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// StatusTransitionRequest is the payload sent by a Dispatch Service to
// POST /v1/status.
type StatusTransitionRequest struct {
	HostID      string    `json:"host_id"`
	ContainerID string    `json:"container_id"`
	JobID       uuid.UUID `json:"job_id"`
	FromState   JobState  `json:"from_state"`
	ToState     JobState  `json:"to_state"`
	Timestamp   time.Time `json:"timestamp"`
	Reason      string    `json:"reason,omitempty"`
	PullURL     string    `json:"pr_url,omitempty"`
}

// Validate checks that all required fields are present and the state
// transition is allowed.
func (r StatusTransitionRequest) Validate() error {
	if r.HostID == "" {
		return fmt.Errorf("protocol: status transition missing host_id")
	}
	if r.ContainerID == "" {
		return fmt.Errorf("protocol: status transition missing container_id")
	}
	if r.JobID.IsZero() {
		return fmt.Errorf("protocol: status transition missing job_id")
	}
	if !r.FromState.Valid() {
		return fmt.Errorf("protocol: status transition invalid from_state %q", r.FromState)
	}
	if !r.ToState.Valid() {
		return fmt.Errorf("protocol: status transition invalid to_state %q", r.ToState)
	}
	if r.Timestamp.IsZero() {
		return fmt.Errorf("protocol: status transition missing timestamp")
	}
	if !ValidTransition(r.FromState, r.ToState) {
		return fmt.Errorf("protocol: invalid transition %s -> %s", r.FromState, r.ToState)
	}
	return nil
}

// StatusTransitionResponse is the response returned by POST /v1/status.
type StatusTransitionResponse struct {
	Accepted bool   `json:"accepted"`
	Error    string `json:"error,omitempty"`
}
