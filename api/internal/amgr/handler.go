package amgr

import (
	"context"
	"net/http"
	"time"

	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/k8s"
	"github.com/asymmetric-effort/convocate/internal/middleware"
	"github.com/asymmetric-effort/convocate/internal/types"
)

type Handler struct {
	store  *Store
	useK8s bool
}

func Register(mux *http.ServeMux) {
	h := &Handler{
		store:  NewStore(),
		useK8s: k8s.Client != nil,
	}
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
	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		agents, err := k8s.ListAgentPods(ctx)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "k8s_error", err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(agents, offset, limit))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(h.store.List(), offset, limit))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req types.CreateAgentRequest
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		p, _ := httputil.PrincipalFromContext(r.Context())
		owner := "system"
		if p != nil {
			owner = p.Username
		}
		agent, err := k8s.CreateAgentPod(ctx, req, owner)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "create_failed", err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, agent)
		return
	}
	agent := h.store.Create(Agent{Project: req.Project, NodeID: req.NodeID, Owner: "admin:admins"})
	httputil.WriteJSON(w, http.StatusCreated, agent)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("agentId")
	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		agent, err := k8s.GetAgentPod(ctx, id)
		if err != nil {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, agent)
		return
	}
	agent, ok := h.store.Get(id)
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
	if h.useK8s {
		httputil.WriteError(w, http.StatusNotImplemented, "not_implemented", "agent reconfiguration via K8s not yet implemented")
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
	id := r.PathValue("agentId")
	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := k8s.DeleteAgentPod(ctx, id); err != nil {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !h.store.Delete(id) {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	if h.useK8s {
		httputil.WriteError(w, http.StatusNotImplemented, "not_implemented", "use create to start a new agent pod")
		return
	}
	if !h.store.SetStatus(r.PathValue("agentId"), "running") {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) stop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("agentId")
	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := k8s.DeleteAgentPod(ctx, id); err != nil {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !h.store.SetStatus(id, "stopping") {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) shell(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteError(w, http.StatusNotImplemented, "not_implemented", "websocket agent shell stub")
}
