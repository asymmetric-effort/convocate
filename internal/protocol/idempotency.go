package protocol

import "fmt"

// IdempotencyKey uniquely identifies a job submission. run_attempt is
// deliberately excluded so that workflow-internal retries (same run,
// higher attempt number) dedupe to the original job ID.
type IdempotencyKey struct {
	Repository  string `json:"repository"`
	IssueNumber int    `json:"issue_number"`
	RunID       int64  `json:"run_id"`
}

// String returns a deterministic key representation for map lookups and
// Redis storage.
func (k IdempotencyKey) String() string {
	return fmt.Sprintf("%s:%d:%d", k.Repository, k.IssueNumber, k.RunID)
}

// Valid reports whether all required fields are populated.
func (k IdempotencyKey) Valid() bool {
	return k.Repository != "" && k.IssueNumber >= 0 && k.RunID > 0
}

// AdHocIdempotencyKey creates an idempotency key for ad-hoc submissions
// (no originating GitHub Issue). issue_number is 0 and run_id is the
// web-ui-submission-id.
func AdHocIdempotencyKey(repository string, submissionID int64) IdempotencyKey {
	return IdempotencyKey{
		Repository:  repository,
		IssueNumber: 0,
		RunID:       submissionID,
	}
}
