package protocol

import (
	"time"

	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// JobSubmissionRequest is the payload sent by the GitHub Action to POST /v1/jobs.
type JobSubmissionRequest struct {
	Repository  string `json:"repository"`
	IssueTitle  string `json:"issue_title"`
	IssueBody   string `json:"issue_body"`
	IssueAuthor string `json:"issue_author"`
	RunID       int64  `json:"run_id"`
	IssueNumber int    `json:"issue_number"`
}

// IdempotencyKey derives the idempotency key from this submission.
func (r *JobSubmissionRequest) IdempotencyKey() IdempotencyKey {
	return IdempotencyKey{
		Repository:  r.Repository,
		IssueNumber: r.IssueNumber,
		RunID:       r.RunID,
	}
}

// JobSubmissionResponse is the response returned by POST /v1/jobs.
type JobSubmissionResponse struct {
	Repository string    `json:"repository"`
	JobID      uuid.UUID `json:"job_id"`
	Duplicate  bool      `json:"duplicate"`
}

// AdHocSubmissionRequest is the payload sent by the Web UI for ad-hoc jobs.
type AdHocSubmissionRequest struct {
	Prompt    string    `json:"prompt"`
	ProjectID uuid.UUID `json:"project_id"`
}

// AdHocSubmissionResponse is the response returned for ad-hoc job submissions.
type AdHocSubmissionResponse struct {
	Repository string    `json:"repository"`
	JobID      uuid.UUID `json:"job_id"`
}

// JobMetadata is the authoritative job record stored by the Router API.
type JobMetadata struct {
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Status      JobState   `json:"status"`
	IssueBody   string     `json:"issue_body"`
	IssueAuthor string     `json:"issue_author"`
	BranchName  string     `json:"branch_name,omitempty"`
	PullURL     string     `json:"pr_url,omitempty"`
	HostID      string     `json:"host_id"`
	ContainerID string     `json:"container_id"`
	Prompt      string     `json:"prompt,omitempty"`
	IssueTitle  string     `json:"issue_title"`
	Repository  string     `json:"repository"`
	IssueNumber int        `json:"issue_number"`
	JobID       uuid.UUID  `json:"job_id"`
	AdHoc       bool       `json:"ad_hoc"`
}
