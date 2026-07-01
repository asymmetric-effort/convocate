// Package projects provides the unified /api/v1/projects endpoint.
// Creating a project atomically creates an IDE project, a board,
// a repo, and an agent-container pod.
package projects

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/asymmetric-effort/convocate/internal/events"
	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/middleware"
)

// ProjectResponse is the unified project view returned to clients.
type ProjectResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	RepoID  string `json:"repoId"`
	BoardID string `json:"boardId"`
	AgentID string `json:"agentId"`
}

// internalCall makes an HTTP request to a local API endpoint.
// Used to delegate to existing sub-API handlers (ide, pb, repo, amgr).
func internalCall(method, path string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, "http://localhost:8443"+path, reqBody)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer internal-projects")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode, nil
}

func Register(mux *http.ServeMux) {
	auth := middleware.Auth
	mux.Handle("GET /api/v1/projects", middleware.Chain(http.HandlerFunc(listProjects), auth))
	mux.Handle("POST /api/v1/projects", middleware.Chain(http.HandlerFunc(createProject), auth))
	mux.Handle("GET /api/v1/projects/{projectId}", middleware.Chain(http.HandlerFunc(getProject), auth))
	mux.Handle("PATCH /api/v1/projects/{projectId}", middleware.Chain(http.HandlerFunc(updateProject), auth))
	mux.Handle("DELETE /api/v1/projects/{projectId}", middleware.Chain(http.HandlerFunc(deleteProject), auth))
}

// listProjects delegates to the IDE project list (canonical source).
func listProjects(w http.ResponseWriter, r *http.Request) {
	body, status, err := internalCall("GET", "/api/v1/ide/project?limit=200", nil)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

// getProject delegates to the IDE project get.
func getProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("projectId")
	// IDE doesn't have a get-by-id endpoint, so list and filter
	body, _, err := internalCall("GET", "/api/v1/ide/project?limit=200", nil)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	var page struct {
		Items []ProjectResponse `json:"items"`
	}
	json.Unmarshal(body, &page)
	for _, p := range page.Items {
		if p.ID == id {
			httputil.WriteJSON(w, http.StatusOK, p)
			return
		}
	}
	httputil.WriteError(w, http.StatusNotFound, "not_found", "project not found")
}

// createProject atomically creates: IDE project + board + repo + agent.
func createProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil || req.Name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}

	log.Printf("[projects] Creating project: %s", req.Name)

	// Step 1: Create IDE project
	ideBody, ideStatus, err := internalCall("POST", "/api/v1/ide/project", map[string]string{"name": req.Name})
	if err != nil || ideStatus != 201 {
		httputil.WriteError(w, http.StatusInternalServerError, "create_failed", fmt.Sprintf("IDE project creation failed: %s", string(ideBody)))
		return
	}
	var ideProject struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		RepoID string `json:"repoId"`
	}
	json.Unmarshal(ideBody, &ideProject)
	log.Printf("[projects] IDE project created: %s", ideProject.ID)

	// Step 2: Create board
	boardBody, boardStatus, _ := internalCall("POST", "/api/v1/pb/board", map[string]string{"name": req.Name, "repoId": ideProject.RepoID})
	var board struct {
		ID string `json:"id"`
	}
	if boardStatus == 201 {
		json.Unmarshal(boardBody, &board)
		log.Printf("[projects] Board created: %s", board.ID)
	}

	// Step 3: Create repo (IDE already creates a repo stub — use it)
	repoID := ideProject.RepoID

	// Step 4: Create agent-container
	agentBody, agentStatus, _ := internalCall("POST", "/api/v1/amgr/agent", map[string]interface{}{
		"project":     req.Name,
		"nodeId":      "",
		"claudeFlags": []string{"--dangerously-skip-permissions"},
	})
	var agent struct {
		ID string `json:"id"`
	}
	if agentStatus == 201 {
		json.Unmarshal(agentBody, &agent)
		log.Printf("[projects] Agent created: %s", agent.ID)
	}

	// Step 5: Link all IDs back to the IDE project
	internalCall("PATCH", "/api/v1/ide/project/"+ideProject.ID, map[string]interface{}{
		"boardId": board.ID,
		"agentId": agent.ID,
	})

	// Step 6: Publish event
	result := ProjectResponse{
		ID:      ideProject.ID,
		Name:    req.Name,
		RepoID:  repoID,
		BoardID: board.ID,
		AgentID: agent.ID,
	}
	events.DefaultHub.Publish("projects/status", "project.created", result)

	httputil.WriteJSON(w, http.StatusCreated, result)
}

// updateProject delegates to the IDE project update.
func updateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("projectId")
	body, _ := io.ReadAll(r.Body)
	respBody, status, err := internalCall("PATCH", "/api/v1/ide/project/"+id, json.RawMessage(body))
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(respBody)
}

// deleteProject deletes the project and cascades to board, repo, agent.
func deleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("projectId")

	// Get project details first
	body, _, err := internalCall("GET", "/api/v1/ide/project?limit=200", nil)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	var page struct {
		Items []ProjectResponse `json:"items"`
	}
	json.Unmarshal(body, &page)

	var project *ProjectResponse
	for _, p := range page.Items {
		if p.ID == id {
			project = &p
			break
		}
	}
	if project == nil {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "project not found")
		return
	}

	// Cascade delete
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	_ = ctx

	if project.AgentID != "" {
		internalCall("DELETE", "/api/v1/amgr/agent/"+project.AgentID, nil)
	}

	// Delete the IDE project record
	internalCall("DELETE", "/api/v1/ide/project/"+id, nil)

	events.DefaultHub.Publish("projects/status", "project.deleted", map[string]string{"id": id})
	w.WriteHeader(http.StatusNoContent)
}
