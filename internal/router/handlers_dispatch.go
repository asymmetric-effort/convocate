package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/asymmetric-effort/convocate/internal/protocol"
)

// handleDispatch handles GET /v1/dispatch?host=<id> — long-poll for
// dispatch events targeted at a specific host.
func (s *Server) handleDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	hostID := r.URL.Query().Get("host")
	if hostID == "" {
		writeError(w, http.StatusBadRequest, "host parameter required")
		return
	}

	ch := s.SubscribeDispatch(hostID)
	defer s.UnsubscribeDispatch(hostID)

	// Check if the client supports SSE.
	flusher, canFlush := w.(http.Flusher)

	if canFlush {
		// SSE mode.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		for {
			select {
			case event, ok := <-ch:
				if !ok {
					return
				}
				data, err := json.Marshal(event)
				if err != nil {
					s.logger.Printf("router: marshal dispatch event: %v", err)
					continue
				}
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	} else {
		// Long-poll mode — wait for one event or timeout.
		select {
		case event, ok := <-ch:
			if !ok {
				writeError(w, http.StatusGone, "subscription closed")
				return
			}
			writeJSON(w, http.StatusOK, event)
		case <-time.After(30 * time.Second):
			w.WriteHeader(http.StatusNoContent)
		case <-r.Context().Done():
			return
		}
	}
}

// handleStatus handles POST /v1/status — per-job status transitions
// from Dispatch Services.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req protocol.StatusTransitionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, protocol.StatusTransitionResponse{
			Accepted: false,
			Error:    err.Error(),
		})
		return
	}

	// Update job metadata.
	meta, err := s.store.GetJobMetadata(req.JobID)
	if err != nil {
		s.logger.Printf("router: get job metadata %s: %v", req.JobID, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if meta == nil {
		writeJSON(w, http.StatusNotFound, protocol.StatusTransitionResponse{
			Accepted: false,
			Error:    "job not found",
		})
		return
	}

	// Verify the transition is from the current state.
	if meta.Status != req.FromState {
		writeJSON(w, http.StatusConflict, protocol.StatusTransitionResponse{
			Accepted: false,
			Error:    fmt.Sprintf("job is in state %q, not %q", meta.Status, req.FromState),
		})
		return
	}

	meta.Status = req.ToState
	meta.UpdatedAt = req.Timestamp
	if req.PullURL != "" {
		meta.PullURL = req.PullURL
	}
	if req.ToState == protocol.JobComplete || req.ToState == protocol.JobFailed || req.ToState == protocol.JobTerminated {
		completedAt := req.Timestamp
		meta.CompletedAt = &completedAt
	}

	err = s.store.SetJobMetadata(*meta)
	if err != nil {
		s.logger.Printf("router: set job metadata: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, protocol.StatusTransitionResponse{Accepted: true})
}

// handleHeartbeat handles POST /v1/heartbeat — host health reports from
// Dispatch Services.
func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req protocol.HeartbeatRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	err := s.store.CacheHeartbeat(req)
	if err != nil {
		s.logger.Printf("router: cache heartbeat for %s: %v", req.HostID, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, protocol.HeartbeatResponse{Accepted: true})
}

// handleHealth handles GET /v1/health.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, protocol.HealthResponse{
		Status:  "ok",
		Version: s.version,
	})
}
