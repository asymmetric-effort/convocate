package router

import (
	"net/http"
	"time"

	"github.com/asymmetric-effort/convocate/internal/protocol"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// handleJobs handles POST /v1/jobs — job submission from GitHub Actions.
func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Authenticate via bearer token.
	token := extractBearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing bearer token")
		return
	}

	var req protocol.JobSubmissionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Repository == "" || req.RunID == 0 {
		writeError(w, http.StatusBadRequest, "repository and run_id are required")
		return
	}

	// Validate the token against the repository.
	valid, err := s.store.ValidateAPIToken(req.Repository, token)
	if err != nil {
		s.logger.Printf("router: validate token for %s: %v", req.Repository, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !valid {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}

	// Check allowlist.
	allowed, err := s.store.AllowlistContains(req.Repository)
	if err != nil {
		s.logger.Printf("router: allowlist check for %s: %v", req.Repository, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !allowed {
		writeError(w, http.StatusNotFound, "repository not found")
		return
	}

	// Idempotency check.
	idempotencyKey := req.IdempotencyKey()
	existingJobID, err := s.store.LookupJobByKey(idempotencyKey)
	if err != nil {
		s.logger.Printf("router: idempotency lookup: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !existingJobID.IsZero() {
		// Duplicate submission — return the original job ID.
		writeJSON(w, http.StatusOK, protocol.JobSubmissionResponse{
			JobID:      existingJobID,
			Duplicate:  true,
			Repository: req.Repository,
		})
		return
	}

	// Resolve the project → container binding.
	route, err := s.store.GetRouteByRepo(req.Repository)
	if err != nil {
		s.logger.Printf("router: route lookup for %s: %v", req.Repository, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if route == nil {
		writeError(w, http.StatusNotFound, "no project configured for this repository")
		return
	}

	// Create the job.
	jobID, err := uuid.New()
	if err != nil {
		s.logger.Printf("router: generate job ID: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	now := time.Now()
	meta := protocol.JobMetadata{
		JobID:       jobID,
		Repository:  req.Repository,
		IssueNumber: req.IssueNumber,
		IssueTitle:  req.IssueTitle,
		IssueBody:   req.IssueBody,
		IssueAuthor: req.IssueAuthor,
		Status:      protocol.JobClaimed,
		HostID:      route.HostID,
		ContainerID: route.ContainerID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Record the job.
	err = s.store.RecordJob(idempotencyKey, jobID)
	if err != nil {
		s.logger.Printf("router: record job: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	err = s.store.SetJobMetadata(meta)
	if err != nil {
		s.logger.Printf("router: set job metadata: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Dispatch to the host.
	event := protocol.DispatchEvent{
		JobID:       jobID,
		ContainerID: route.ContainerID,
		Repository:  req.Repository,
		IssueNumber: req.IssueNumber,
		IssueTitle:  req.IssueTitle,
		IssueBody:   req.IssueBody,
		IssueAuthor: req.IssueAuthor,
	}
	err = s.dispatchToHost(route.HostID, event)
	if err != nil {
		s.logger.Printf("router: dispatch to host %s: %v", route.HostID, err)
		// Job was recorded but dispatch failed — mark as failed_dispatch.
		meta.Status = protocol.JobFailed
		meta.UpdatedAt = time.Now()
		s.store.SetJobMetadata(meta)
		writeError(w, http.StatusServiceUnavailable, "dispatch failed")
		return
	}

	writeJSON(w, http.StatusOK, protocol.JobSubmissionResponse{
		JobID:      jobID,
		Duplicate:  false,
		Repository: req.Repository,
	})
}
