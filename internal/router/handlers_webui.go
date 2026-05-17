package router

import (
	"net/http"
	"time"

	"github.com/asymmetric-effort/convocate/internal/openbao"
	"github.com/asymmetric-effort/convocate/internal/protocol"
	"github.com/asymmetric-effort/convocate/internal/uuid"
)

// handleProjects handles GET /ui/api/projects — list all projects.
func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// In MVP, we don't have a list-all-projects index. This would need a
	// SCAN over project keys. For now, return an empty list placeholder.
	writeJSON(w, http.StatusOK, []protocol.ProjectInfo{})
}

// handleCreateProject handles POST /ui/api/projects/create.
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requireJSONContentType(w, r) {
		return
	}

	var req protocol.CreateProjectRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Repository == "" || req.SSHPrivateKey == "" || req.GitHubPAT == "" {
		writeError(w, http.StatusBadRequest, "repository, ssh_private_key, and github_pat are required")
		return
	}

	// Check if repo is already allowlisted.
	already, err := s.store.AllowlistContains(req.Repository)
	if err != nil {
		s.logger.Printf("router: allowlist check: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if already {
		writeError(w, http.StatusConflict, "project already exists for this repository")
		return
	}

	projectID := uuid.New()

	// Step 1: Add to allowlist.
	err = s.store.AllowlistAdd(req.Repository)
	if err != nil {
		s.logger.Printf("router: allowlist add: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Step 2: Store secrets in OpenBao.
	secrets := openbaoProjectSecrets(req)
	err = s.bao.StoreProjectSecrets(projectID, secrets)
	if err != nil {
		s.logger.Printf("router: store secrets: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to store secrets")
		return
	}

	// Step 3: Write OpenBao policy for this project.
	err = s.bao.WriteProjectPolicy(projectID)
	if err != nil {
		s.logger.Printf("router: write project policy: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to write policy")
		return
	}

	// Step 4: Mint API token.
	apiToken := generateAPIToken()
	err = s.store.SetAPIToken(req.Repository, apiToken)
	if err != nil {
		s.logger.Printf("router: store API token: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Step 5: Select host and create routing entry.
	// In MVP, we use a placeholder host ID. The actual host selection
	// happens when hosts register via heartbeat.
	hostID := "pending"
	containerID := "pending-" + projectID.String()[:8]

	route := protocol.ProjectRouteEntry{
		ProjectID:   projectID,
		Repository:  req.Repository,
		HostID:      hostID,
		ContainerID: containerID,
	}
	err = s.store.SetRoute(route)
	if err != nil {
		s.logger.Printf("router: set route: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Store project info.
	info := protocol.ProjectInfo{
		ProjectID:      projectID,
		Repository:     req.Repository,
		HostID:         hostID,
		ContainerID:    containerID,
		ContainerState: protocol.ContainerProvisioning,
		CreatedAt:      time.Now(),
	}
	err = s.store.SetProjectInfo(&info)
	if err != nil {
		s.logger.Printf("router: set project info: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, protocol.CreateProjectResponse{
		ProjectID:      projectID,
		Repository:     req.Repository,
		APIToken:       apiToken,
		HostID:         hostID,
		ContainerID:    containerID,
		ContainerState: protocol.ContainerProvisioning,
	})
}

// handleDeleteProject handles POST /ui/api/projects/delete.
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requireJSONContentType(w, r) {
		return
	}

	var req protocol.DeleteProjectRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.ProjectID.IsZero() {
		writeError(w, http.StatusBadRequest, "project_id is required")
		return
	}

	// Look up the project.
	info, err := s.store.GetProjectInfo(req.ProjectID)
	if err != nil {
		s.logger.Printf("router: get project info: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if info == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Step 1: Remove from allowlist (stops new submissions).
	err = s.store.AllowlistRemove(info.Repository)
	if err != nil {
		s.logger.Printf("router: allowlist remove: %v", err)
	}

	// Step 2: Revoke API token.
	err = s.store.DeleteAPIToken(info.Repository)
	if err != nil {
		s.logger.Printf("router: delete API token: %v", err)
	}

	// Step 3: Delete secrets from OpenBao.
	err = s.bao.DeleteProjectSecrets(req.ProjectID)
	if err != nil {
		s.logger.Printf("router: delete secrets: %v", err)
	}

	// Step 4: Delete OpenBao policy.
	err = s.bao.DeleteProjectPolicy(req.ProjectID)
	if err != nil {
		s.logger.Printf("router: delete policy: %v", err)
	}

	// Step 5: Remove routing entry.
	err = s.store.DeleteRoute(req.ProjectID, info.Repository)
	if err != nil {
		s.logger.Printf("router: delete route: %v", err)
	}

	// Step 6: Remove project info.
	err = s.store.DeleteProjectInfo(req.ProjectID, info.Repository)
	if err != nil {
		s.logger.Printf("router: delete project info: %v", err)
	}

	// Step 7: Remove container map entry.
	if info.ContainerID != "" {
		err = s.store.DeleteContainer(info.ContainerID)
		if err != nil {
			s.logger.Printf("router: delete container: %v", err)
		}
	}

	writeJSON(w, http.StatusOK, protocol.DeleteProjectResponse{
		ProjectID:  req.ProjectID,
		Repository: info.Repository,
		Deleted:    true,
	})
}

// handleClusterAuth handles POST /ui/api/auth — set cluster-wide Claude auth.
func (s *Server) handleClusterAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		mode, _, err := s.store.GetClusterAuth()
		if err != nil {
			s.logger.Printf("router: get cluster auth: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, protocol.SetClusterAuthResponse{
			Mode:    mode,
			Updated: false,
		})
		return
	}

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requireJSONContentType(w, r) {
		return
	}

	var req protocol.SetClusterAuthRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if !req.Mode.Valid() {
		writeError(w, http.StatusBadRequest, "invalid auth mode")
		return
	}

	var credential string
	switch req.Mode {
	case protocol.AuthModeAnthropicKey:
		if req.APIKey == "" {
			writeError(w, http.StatusBadRequest, "api_key required for anthropic_api_key mode")
			return
		}
		credential = req.APIKey
	case protocol.AuthModeClaudeSession:
		if req.SessionToken == "" {
			writeError(w, http.StatusBadRequest, "session_token required for claude_session mode")
			return
		}
		credential = req.SessionToken
	}

	// Store in Redis (Router API namespace) for fast lookups.
	err := s.store.SetClusterAuth(req.Mode, credential)
	if err != nil {
		s.logger.Printf("router: set cluster auth: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Also store in OpenBao as shared credential.
	// sharedSecretName is the OpenBao path segment for this auth mode — not a credential value.
	sharedSecretName := "anthropic_api_key" //nolint:gosec // Not a hardcoded credential; this is a vault path name.
	if req.Mode == protocol.AuthModeClaudeSession {
		sharedSecretName = "claudeai_session"
	}
	err = s.bao.StoreSharedCredential(sharedSecretName, credential)
	if err != nil {
		s.logger.Printf("router: store shared credential: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to store credential")
		return
	}

	writeJSON(w, http.StatusOK, protocol.SetClusterAuthResponse{
		Mode:    req.Mode,
		Updated: true,
	})
}

// handleAdHocSubmit handles POST /ui/api/adhoc — ad-hoc job submission.
func (s *Server) handleAdHocSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requireJSONContentType(w, r) {
		return
	}

	var req protocol.AdHocSubmissionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.ProjectID.IsZero() || req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "project_id and prompt are required")
		return
	}

	// Look up the project.
	route, err := s.store.GetRoute(req.ProjectID)
	if err != nil {
		s.logger.Printf("router: get route: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if route == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	jobID := uuid.New()

	now := time.Now()
	meta := protocol.JobMetadata{
		JobID:       jobID,
		Repository:  route.Repository,
		Status:      protocol.JobClaimed,
		HostID:      route.HostID,
		ContainerID: route.ContainerID,
		AdHoc:       true,
		Prompt:      req.Prompt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Record with ad-hoc idempotency key.
	idempotencyKey := protocol.AdHocIdempotencyKey(route.Repository, now.UnixNano())
	err = s.store.RecordJob(idempotencyKey, jobID)
	if err != nil {
		s.logger.Printf("router: record ad-hoc job: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	err = s.store.SetJobMetadata(&meta)
	if err != nil {
		s.logger.Printf("router: set job metadata: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Dispatch.
	event := protocol.DispatchEvent{
		JobID:       jobID,
		ContainerID: route.ContainerID,
		Repository:  route.Repository,
		AdHoc:       true,
		Prompt:      req.Prompt,
	}
	err = s.dispatchToHost(route.HostID, &event)
	if err != nil {
		s.logger.Printf("router: dispatch ad-hoc to host %s: %v", route.HostID, err)
		writeError(w, http.StatusServiceUnavailable, "dispatch failed")
		return
	}

	writeJSON(w, http.StatusOK, protocol.AdHocSubmissionResponse{
		JobID:      jobID,
		Repository: route.Repository,
	})
}

// handleUpgradeContainer handles POST /ui/api/projects/upgrade.
func (s *Server) handleUpgradeContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requireJSONContentType(w, r) {
		return
	}

	var req protocol.UpgradeContainerRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.ProjectID.IsZero() {
		writeError(w, http.StatusBadRequest, "project_id is required")
		return
	}

	// In MVP, container upgrade is a placeholder — the actual stop-old/
	// provision-new logic requires Dispatch Service coordination.
	writeJSON(w, http.StatusOK, protocol.UpgradeContainerResponse{
		ProjectID:      req.ProjectID,
		ContainerID:    "upgraded-placeholder",
		ContainerState: protocol.ContainerProvisioning,
	})
}

// handleUpgradeAllIdle handles POST /ui/api/projects/upgrade-all-idle.
func (s *Server) handleUpgradeAllIdle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requireJSONContentType(w, r) {
		return
	}

	// Placeholder — would iterate all projects and upgrade those with
	// no in-flight jobs.
	writeJSON(w, http.StatusOK, protocol.UpgradeAllIdleResponse{
		Upgraded: []uuid.UUID{},
		Skipped:  []uuid.UUID{},
	})
}

// handleJobsList handles GET /ui/api/jobs — list jobs.
func (s *Server) handleJobsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// Placeholder — would scan job metadata keys.
	writeJSON(w, http.StatusOK, []protocol.JobMetadata{})
}

// handleHostsList handles GET /ui/api/hosts — list agent hosts with health.
func (s *Server) handleHostsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// Placeholder — would iterate cached heartbeats.
	writeJSON(w, http.StatusOK, []protocol.HostHealthInfo{})
}

// openbaoProjectSecrets converts a CreateProjectRequest to OpenBao secrets.
func openbaoProjectSecrets(req protocol.CreateProjectRequest) openbao.ProjectSecrets {
	return openbao.ProjectSecrets{
		SSHPrivateKey: req.SSHPrivateKey,
		GitHubPAT:     req.GitHubPAT,
		CustomSecrets: req.CustomSecrets,
	}
}
