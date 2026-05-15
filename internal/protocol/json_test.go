package protocol

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/internal/uuid"
)

func TestJobSubmissionRequestJSONRoundTrip(t *testing.T) {
	original := JobSubmissionRequest{
		Repository:  "org/repo",
		IssueNumber: 42,
		IssueTitle:  "Fix login bug",
		IssueBody:   "The login page crashes when...",
		IssueAuthor: "alice",
		RunID:       12345,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded JobSubmissionRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip mismatch:\ngot  %+v\nwant %+v", decoded, original)
	}
}

func TestJobSubmissionResponseJSONRoundTrip(t *testing.T) {
	original := JobSubmissionResponse{
		JobID:      uuid.MustNew(),
		Duplicate:  false,
		Repository: "org/repo",
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded JobSubmissionResponse
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.JobID != original.JobID {
		t.Errorf("JobID: got %s, want %s", decoded.JobID, original.JobID)
	}
	if decoded.Duplicate != original.Duplicate {
		t.Errorf("Duplicate: got %v, want %v", decoded.Duplicate, original.Duplicate)
	}
	if decoded.Repository != original.Repository {
		t.Errorf("Repository: got %q, want %q", decoded.Repository, original.Repository)
	}
}

func TestStatusTransitionRequestJSONRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	original := StatusTransitionRequest{
		HostID:      "host-1",
		ContainerID: "container-abc",
		JobID:       uuid.MustNew(),
		FromState:   JobClaimed,
		ToState:     JobRunning,
		Timestamp:   now,
		Reason:      "starting implementation",
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded StatusTransitionRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.HostID != original.HostID {
		t.Errorf("HostID: got %q, want %q", decoded.HostID, original.HostID)
	}
	if decoded.JobID != original.JobID {
		t.Errorf("JobID: got %s, want %s", decoded.JobID, original.JobID)
	}
	if decoded.FromState != original.FromState {
		t.Errorf("FromState: got %q, want %q", decoded.FromState, original.FromState)
	}
	if decoded.ToState != original.ToState {
		t.Errorf("ToState: got %q, want %q", decoded.ToState, original.ToState)
	}
}

func TestHeartbeatRequestJSONRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	original := HeartbeatRequest{
		HostID:         "host-1",
		ContainerCount: 5,
		CPUPercent:     65.3,
		MemoryPercent:  78.1,
		Timestamp:      now,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded HeartbeatRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.HostID != original.HostID {
		t.Errorf("HostID: got %q, want %q", decoded.HostID, original.HostID)
	}
	if decoded.ContainerCount != original.ContainerCount {
		t.Errorf("ContainerCount: got %d, want %d", decoded.ContainerCount, original.ContainerCount)
	}
}

func TestDispatchEventJSONRoundTrip(t *testing.T) {
	original := DispatchEvent{
		JobID:       uuid.MustNew(),
		ContainerID: "container-xyz",
		Repository:  "org/repo",
		IssueNumber: 10,
		IssueTitle:  "Add feature X",
		IssueBody:   "We need feature X because...",
		IssueAuthor: "bob",
		AdHoc:       false,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded DispatchEvent
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.JobID != original.JobID {
		t.Errorf("JobID: got %s, want %s", decoded.JobID, original.JobID)
	}
	if decoded.ContainerID != original.ContainerID {
		t.Errorf("ContainerID: got %q, want %q", decoded.ContainerID, original.ContainerID)
	}
	if decoded.Repository != original.Repository {
		t.Errorf("Repository: got %q, want %q", decoded.Repository, original.Repository)
	}
}

func TestJobSubmissionRequestIdempotencyKey(t *testing.T) {
	req := JobSubmissionRequest{
		Repository:  "org/repo",
		IssueNumber: 42,
		RunID:       12345,
	}
	key := req.IdempotencyKey()
	if key.Repository != "org/repo" {
		t.Errorf("Repository: got %q, want %q", key.Repository, "org/repo")
	}
	if key.IssueNumber != 42 {
		t.Errorf("IssueNumber: got %d, want 42", key.IssueNumber)
	}
	if key.RunID != 12345 {
		t.Errorf("RunID: got %d, want 12345", key.RunID)
	}
}

func TestJobMetadataJSONRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	completedAt := now.Add(5 * time.Minute)
	original := JobMetadata{
		JobID:       uuid.MustNew(),
		Repository:  "org/repo",
		IssueNumber: 42,
		IssueTitle:  "Fix the bug",
		IssueBody:   "It crashes when...",
		IssueAuthor: "alice",
		BranchName:  "fix/issue-42",
		PullURL:     "https://github.com/org/repo/pull/100",
		Status:      JobComplete,
		HostID:      "host-1",
		ContainerID: "container-abc",
		AdHoc:       false,
		CreatedAt:   now,
		UpdatedAt:   now,
		CompletedAt: &completedAt,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded JobMetadata
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.JobID != original.JobID {
		t.Errorf("JobID: got %s, want %s", decoded.JobID, original.JobID)
	}
	if decoded.Status != original.Status {
		t.Errorf("Status: got %q, want %q", decoded.Status, original.Status)
	}
	if decoded.CompletedAt == nil {
		t.Error("CompletedAt: got nil, want non-nil")
	}
}

func TestCreateProjectRequestJSONRoundTrip(t *testing.T) {
	original := CreateProjectRequest{
		Repository:    "org/repo",
		SSHPrivateKey: "-----BEGIN OPENSSH PRIVATE KEY-----\nfake\n-----END OPENSSH PRIVATE KEY-----",
		GitHubPAT:     "ghp_abc123",
		CustomSecrets: map[string]string{"NPM_TOKEN": "tok_xyz"},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded CreateProjectRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.Repository != original.Repository {
		t.Errorf("Repository: got %q, want %q", decoded.Repository, original.Repository)
	}
	if decoded.SSHPrivateKey != original.SSHPrivateKey {
		t.Error("SSHPrivateKey mismatch")
	}
	if decoded.GitHubPAT != original.GitHubPAT {
		t.Errorf("GitHubPAT: got %q, want %q", decoded.GitHubPAT, original.GitHubPAT)
	}
	if decoded.CustomSecrets["NPM_TOKEN"] != "tok_xyz" {
		t.Error("CustomSecrets mismatch")
	}
}

func TestAdHocSubmissionRequestJSONRoundTrip(t *testing.T) {
	original := AdHocSubmissionRequest{
		ProjectID: uuid.MustNew(),
		Prompt:    "Add a health check endpoint",
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded AdHocSubmissionRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.ProjectID != original.ProjectID {
		t.Errorf("ProjectID: got %s, want %s", decoded.ProjectID, original.ProjectID)
	}
	if decoded.Prompt != original.Prompt {
		t.Errorf("Prompt: got %q, want %q", decoded.Prompt, original.Prompt)
	}
}

func TestSetClusterAuthRequestJSONRoundTrip(t *testing.T) {
	original := SetClusterAuthRequest{
		Mode:   AuthModeAnthropicKey,
		APIKey: "sk-ant-api03-abc123",
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded SetClusterAuthRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.Mode != original.Mode {
		t.Errorf("Mode: got %q, want %q", decoded.Mode, original.Mode)
	}
	if decoded.APIKey != original.APIKey {
		t.Errorf("APIKey: got %q, want %q", decoded.APIKey, original.APIKey)
	}
}

func TestHealthResponseJSON(t *testing.T) {
	original := HealthResponse{
		Status:  "ok",
		Version: "v0.2.0",
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded HealthResponse
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip mismatch:\ngot  %+v\nwant %+v", decoded, original)
	}
}
