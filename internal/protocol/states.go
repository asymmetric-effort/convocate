package protocol

import (
	"fmt"
	"strings"
)

// ContainerState represents the lifecycle state of an Agent Container.
type ContainerState string

const (
	ContainerProvisioning       ContainerState = "provisioning"
	ContainerRunning            ContainerState = "running"
	ContainerStopped            ContainerState = "stopped"
	ContainerProvisioningFailed ContainerState = "provisioning_failed"
	ContainerFailedDispatch     ContainerState = "failed_dispatch"
)

// ValidContainerStates is the canonical set of container states.
var ValidContainerStates = []ContainerState{
	ContainerProvisioning,
	ContainerRunning,
	ContainerStopped,
	ContainerProvisioningFailed,
	ContainerFailedDispatch,
}

// Valid reports whether the ContainerState is one of the canonical values.
func (s ContainerState) Valid() bool {
	for _, valid := range ValidContainerStates {
		if s == valid {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (s ContainerState) String() string {
	return string(s)
}

// ParseContainerState parses a string into a ContainerState, returning an error
// if the value is not in the canonical set.
func ParseContainerState(s string) (ContainerState, error) {
	state := ContainerState(strings.TrimSpace(s))
	if !state.Valid() {
		return "", fmt.Errorf("protocol: invalid container state %q", s)
	}
	return state, nil
}

// JobState represents the lifecycle state of a job within an Agent Container.
type JobState string

const (
	JobClaimed    JobState = "claimed"
	JobRunning    JobState = "running"
	JobComplete   JobState = "complete"
	JobFailed     JobState = "failed"
	JobClarifying JobState = "clarifying"
	JobTerminated JobState = "terminated"
)

// ValidJobStates is the canonical set of job lifecycle states.
var ValidJobStates = []JobState{
	JobClaimed,
	JobRunning,
	JobComplete,
	JobFailed,
	JobClarifying,
	JobTerminated,
}

// Valid reports whether the JobState is one of the canonical values.
func (s JobState) Valid() bool {
	for _, valid := range ValidJobStates {
		if s == valid {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (s JobState) String() string {
	return string(s)
}

// ParseJobState parses a string into a JobState, returning an error if the
// value is not in the canonical set.
func ParseJobState(s string) (JobState, error) {
	state := JobState(strings.TrimSpace(s))
	if !state.Valid() {
		return "", fmt.Errorf("protocol: invalid job state %q", s)
	}
	return state, nil
}

// ValidJobTransitions defines the allowed state transitions for jobs.
// The key is the current state; the value is the set of states it can
// transition to.
var ValidJobTransitions = map[JobState][]JobState{
	JobClaimed:    {JobRunning, JobFailed, JobTerminated},
	JobRunning:    {JobComplete, JobFailed, JobClarifying, JobTerminated},
	JobClarifying: {JobRunning, JobFailed, JobTerminated},
	JobComplete:   {},
	JobFailed:     {},
	JobTerminated: {},
}

// ValidTransition reports whether transitioning from 'from' to 'to' is allowed.
func ValidTransition(from, to JobState) bool {
	allowed, exists := ValidJobTransitions[from]
	if !exists {
		return false
	}
	for _, state := range allowed {
		if state == to {
			return true
		}
	}
	return false
}
