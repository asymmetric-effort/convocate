package protocol

import (
	"time"

	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// JobSubmissionRequest is the payload sent by the GitHub Action to POST /v1/jobs.
type JobSubmissionRequest struct {
	Repository  string `json:"repository"`
	IssueNumber int    `json:"issue_number"`
	IssueTitle  string `json:"issue_title"`
	IssueBody   string `json:"issue_body"`
	IssueAuthor string `json:"issue_author"`
	RunID       int64  `json:"run_id"`
}

// IdempotencyKey derives the idempotency key from this submission.
func (r JobSubmissionRequest) IdempotencyKey() IdempotencyKey {
	return IdempotencyKey{
		Repository:  r.Repository,
		IssueNumber: r.IssueNumber,
		RunID:       r.RunID,
	}
}

// JobSubmissionResponse is the response returned by POST /v1/jobs.
type JobSubmissionResponse struct {
	JobID      uuid.UUID `json:"job_id"`
	Duplicate  bool      `json:"duplicate"`
	Repository string    `json:"repository"`
}

// AdHocSubmissionRequest is the payload sent by the Web UI for ad-hoc jobs.
type AdHocSubmissionRequest struct {
	ProjectID uuid.UUID `json:"project_id"`
	Prompt    string    `json:"prompt"`
}

// AdHocSubmissionResponse is the response returned for ad-hoc job submissions.
type AdHocSubmissionResponse struct {
	JobID      uuid.UUID `json:"job_id"`
	Repository string    `json:"repository"`
}

// JobMetadata is the authoritative job record stored by the Router API.
type JobMetadata struct {
	JobID       uuid.UUID  `json:"job_id"`
	Repository  string     `json:"repository"`
	IssueNumber int        `json:"issue_number"`
	IssueTitle  string     `json:"issue_title"`
	IssueBody   string     `json:"issue_body"`
	IssueAuthor string     `json:"issue_author"`
	BranchName  string     `json:"branch_name,omitempty"`
	PullURL     string     `json:"pr_url,omitempty"`
	Status      JobState   `json:"status"`
	HostID      string     `json:"host_id"`
	ContainerID string     `json:"container_id"`
	AdHoc       bool       `json:"ad_hoc"`
	Prompt      string     `json:"prompt,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
