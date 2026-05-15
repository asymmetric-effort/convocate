package mock

// Package mock provides shared test doubles used by integration and e2e tests.
// Individual package-level mocks (MockConn, mockContainerManager, etc.) live
// in their respective test files. This package holds cross-cutting mocks.

// ClaudeResponse represents a canned response from mock-claude.
type ClaudeResponse struct {
	Prompt   string
	Response string
}

// DefaultResponses returns a set of deterministic prompt->response pairs
// used by the mock-claude binary and e2e tests.
func DefaultResponses() []ClaudeResponse {
	return []ClaudeResponse{
		{
			Prompt:   "Background task:",
			Response: "I'll implement the requested changes. Creating feature branch and working on the solution.",
		},
		{
			Prompt:   "fix",
			Response: "I've identified the bug and applied a fix. The issue was in the error handling path. Tests pass.",
		},
		{
			Prompt:   "feature",
			Response: "I've implemented the requested feature. All tests pass and the changes are ready for review.",
		},
	}
}
