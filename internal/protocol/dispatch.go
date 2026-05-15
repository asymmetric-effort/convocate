package protocol

import (
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// DispatchEvent is the payload delivered to a Dispatch Service via
// GET /v1/dispatch?host=<id> (long-poll or SSE).
type DispatchEvent struct {
	ContainerID string    `json:"container_id"`
	Repository  string    `json:"repository"`
	IssueTitle  string    `json:"issue_title"`
	IssueBody   string    `json:"issue_body"`
	IssueAuthor string    `json:"issue_author"`
	Prompt      string    `json:"prompt,omitempty"`
	IssueNumber int       `json:"issue_number"`
	JobID       uuid.UUID `json:"job_id"`
	AdHoc       bool      `json:"ad_hoc"`
}
