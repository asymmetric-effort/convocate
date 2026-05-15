package protocol

import (
	"testing"
)

func TestContainerStateValid(t *testing.T) {
	tests := []struct {
		state ContainerState
		want  bool
	}{
		{ContainerProvisioning, true},
		{ContainerRunning, true},
		{ContainerStopped, true},
		{ContainerProvisioningFailed, true},
		{ContainerFailedDispatch, true},
		{"unknown", false},
		{"", false},
	}
	for _, testCase := range tests {
		t.Run(string(testCase.state), func(t *testing.T) {
			got := testCase.state.Valid()
			if got != testCase.want {
				t.Errorf("ContainerState(%q).Valid() = %v, want %v", testCase.state, got, testCase.want)
			}
		})
	}
}

func TestContainerStateString(t *testing.T) {
	if ContainerRunning.String() != "running" {
		t.Errorf("ContainerRunning.String() = %q, want %q", ContainerRunning.String(), "running")
	}
}

func TestParseContainerState(t *testing.T) {
	tests := []struct {
		input   string
		want    ContainerState
		wantErr bool
	}{
		{"running", ContainerRunning, false},
		{"provisioning", ContainerProvisioning, false},
		{"stopped", ContainerStopped, false},
		{"provisioning_failed", ContainerProvisioningFailed, false},
		{"failed_dispatch", ContainerFailedDispatch, false},
		{" running ", ContainerRunning, false},
		{"invalid", "", true},
		{"", "", true},
	}
	for _, testCase := range tests {
		t.Run(testCase.input, func(t *testing.T) {
			got, err := ParseContainerState(testCase.input)
			if (err != nil) != testCase.wantErr {
				t.Errorf("ParseContainerState(%q) error = %v, wantErr %v", testCase.input, err, testCase.wantErr)
				return
			}
			if got != testCase.want {
				t.Errorf("ParseContainerState(%q) = %q, want %q", testCase.input, got, testCase.want)
			}
		})
	}
}

func TestJobStateValid(t *testing.T) {
	tests := []struct {
		state JobState
		want  bool
	}{
		{JobClaimed, true},
		{JobRunning, true},
		{JobComplete, true},
		{JobFailed, true},
		{JobClarifying, true},
		{JobTerminated, true},
		{"unknown", false},
		{"", false},
	}
	for _, testCase := range tests {
		t.Run(string(testCase.state), func(t *testing.T) {
			got := testCase.state.Valid()
			if got != testCase.want {
				t.Errorf("JobState(%q).Valid() = %v, want %v", testCase.state, got, testCase.want)
			}
		})
	}
}

func TestJobStateString(t *testing.T) {
	if JobRunning.String() != "running" {
		t.Errorf("JobRunning.String() = %q, want %q", JobRunning.String(), "running")
	}
}

func TestParseJobState(t *testing.T) {
	tests := []struct {
		input   string
		want    JobState
		wantErr bool
	}{
		{"claimed", JobClaimed, false},
		{"running", JobRunning, false},
		{"complete", JobComplete, false},
		{"failed", JobFailed, false},
		{"clarifying", JobClarifying, false},
		{"terminated", JobTerminated, false},
		{" running ", JobRunning, false},
		{"invalid", "", true},
		{"", "", true},
	}
	for _, testCase := range tests {
		t.Run(testCase.input, func(t *testing.T) {
			got, err := ParseJobState(testCase.input)
			if (err != nil) != testCase.wantErr {
				t.Errorf("ParseJobState(%q) error = %v, wantErr %v", testCase.input, err, testCase.wantErr)
				return
			}
			if got != testCase.want {
				t.Errorf("ParseJobState(%q) = %q, want %q", testCase.input, got, testCase.want)
			}
		})
	}
}

func TestValidTransition(t *testing.T) {
	tests := []struct {
		from JobState
		to   JobState
		want bool
	}{
		// claimed -> ...
		{JobClaimed, JobRunning, true},
		{JobClaimed, JobFailed, true},
		{JobClaimed, JobTerminated, true},
		{JobClaimed, JobComplete, false},
		{JobClaimed, JobClarifying, false},

		// running -> ...
		{JobRunning, JobComplete, true},
		{JobRunning, JobFailed, true},
		{JobRunning, JobClarifying, true},
		{JobRunning, JobTerminated, true},
		{JobRunning, JobClaimed, false},

		// clarifying -> ...
		{JobClarifying, JobRunning, true},
		{JobClarifying, JobFailed, true},
		{JobClarifying, JobTerminated, true},
		{JobClarifying, JobComplete, false},
		{JobClarifying, JobClaimed, false},

		// terminal states -> nothing
		{JobComplete, JobRunning, false},
		{JobComplete, JobFailed, false},
		{JobFailed, JobRunning, false},
		{JobTerminated, JobRunning, false},

		// invalid state
		{JobState("bogus"), JobRunning, false},
	}
	for _, testCase := range tests {
		name := testCase.from.String() + "->" + testCase.to.String()
		t.Run(name, func(t *testing.T) {
			got := ValidTransition(testCase.from, testCase.to)
			if got != testCase.want {
				t.Errorf("ValidTransition(%q, %q) = %v, want %v", testCase.from, testCase.to, got, testCase.want)
			}
		})
	}
}
