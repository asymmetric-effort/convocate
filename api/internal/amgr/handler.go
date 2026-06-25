package amgr

import (
	"net/http"

	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/middleware"
)

type Handler struct{ store *Store }

func Register(mux *http.ServeMux) {
	h := &Handler{store: NewStore()}
	auth := middleware.Auth

	mux.Handle("GET /api/v1/amgr/agent", middleware.Chain(http.HandlerFunc(h.list), auth, middleware.RBAC("agent-view")))
	mux.Handle("POST /api/v1/amgr/agent", middleware.Chain(http.HandlerFunc(h.create), auth, middleware.RBAC("agent-update")))
	mux.Handle("GET /api/v1/amgr/agent/{agentId}", middleware.Chain(http.HandlerFunc(h.get), auth, middleware.RBAC("agent-view")))
	mux.Handle("PATCH /api/v1/amgr/agent/{agentId}", middleware.Chain(http.HandlerFunc(h.update), auth, middleware.RBAC("agent-update")))
	mux.Handle("DELETE /api/v1/amgr/agent/{agentId}", middleware.Chain(http.HandlerFunc(h.del), auth, middleware.RBAC("agent-update")))
	mux.Handle("POST /api/v1/amgr/agent/{agentId}/start", middleware.Chain(http.HandlerFunc(h.start), auth, middleware.RBAC("agent-update")))
	mux.Handle("POST /api/v1/amgr/agent/{agentId}/stop", middleware.Chain(http.HandlerFunc(h.stop), auth, middleware.RBAC("agent-update")))
	mux.Handle("GET /api/v1/amgr/agent/{agentId}/shell", middleware.Chain(http.HandlerFunc(h.shell), auth, middleware.RBAC("agent-view")))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	offset, limit := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(h.store.List(), offset, limit))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Project string `json:"project"`
		NodeID  string `json:"nodeId"`
		Image   string `json:"image"`
		Command string `json:"command"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	agent := h.store.Create(Agent{Project: req.Project, NodeID: req.NodeID, Owner: "admin:admins"})
	httputil.WriteJSON(w, http.StatusCreated, agent)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	agent, ok := h.store.Get(r.PathValue("agentId"))
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, agent)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Project *string `json:"project"`
		NodeID  *string `json:"nodeId"`
		Expose  *string `json:"expose"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	agent, ok := h.store.Update(r.PathValue("agentId"), req.Project, req.NodeID, req.Expose)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, agent)
}

func (h *Handler) del(w http.ResponseWriter, r *http.Request) {
	if !h.store.Delete(r.PathValue("agentId")) {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	if !h.store.SetStatus(r.PathValue("agentId"), "running") {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) stop(w http.ResponseWriter, r *http.Request) {
	if !h.store.SetStatus(r.PathValue("agentId"), "stopping") {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) shell(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteError(w, http.StatusNotImplemented, "not_implemented", "websocket agent shell stub")
}
