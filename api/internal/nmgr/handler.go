package nmgr

import (
	"net/http"

	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/middleware"
)

type Handler struct{ store *Store }

func Register(mux *http.ServeMux) {
	h := &Handler{store: NewStore()}
	auth := middleware.Auth

	mux.Handle("GET /api/v1/nmgr/node", middleware.Chain(http.HandlerFunc(h.list), auth, middleware.RBAC("node-view")))
	mux.Handle("POST /api/v1/nmgr/node", middleware.Chain(http.HandlerFunc(h.create), auth, middleware.RBAC("node-create")))
	mux.Handle("GET /api/v1/nmgr/node/{nodeId}", middleware.Chain(http.HandlerFunc(h.get), auth, middleware.RBAC("node-view")))
	mux.Handle("PATCH /api/v1/nmgr/node/{nodeId}", middleware.Chain(http.HandlerFunc(h.update), auth, middleware.RBAC("node-update")))
	mux.Handle("DELETE /api/v1/nmgr/node/{nodeId}", middleware.Chain(http.HandlerFunc(h.del), auth, middleware.RBAC("node-delete")))
	mux.Handle("POST /api/v1/nmgr/node/{nodeId}/start", middleware.Chain(http.HandlerFunc(h.start), auth, middleware.RBAC("node-update")))
	mux.Handle("POST /api/v1/nmgr/node/{nodeId}/stop", middleware.Chain(http.HandlerFunc(h.stop), auth, middleware.RBAC("node-update")))
	mux.Handle("GET /api/v1/nmgr/node/{nodeId}/note", middleware.Chain(http.HandlerFunc(h.listNotes), auth, middleware.RBAC("node-view")))
	mux.Handle("POST /api/v1/nmgr/node/{nodeId}/note", middleware.Chain(http.HandlerFunc(h.addNote), auth, middleware.RBAC("node-update")))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	offset, limit := httputil.ParsePagination(r)
	nodes := h.store.List()
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(nodes, offset, limit))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host     string   `json:"host"`
		User     string   `json:"user"`
		Password string   `json:"password,omitempty"`
		Location string   `json:"location"`
		Tags     []string `json:"tags"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	node := h.store.Create(Node{IP: req.Host, Location: req.Location, Tags: req.Tags})
	httputil.WriteJSON(w, http.StatusAccepted, node)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")
	node, ok := h.store.Get(id)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	detail := struct {
		Node
		AgentList []any  `json:"agentList"`
		Notes     []Note `json:"notes"`
	}{Node: node, AgentList: []any{}, Notes: h.store.ListNotes(id)}
	httputil.WriteJSON(w, http.StatusOK, detail)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")
	var req struct {
		Location *string  `json:"location"`
		Tags     []string `json:"tags"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	node, ok := h.store.Update(id, req.Location, req.Tags)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, node)
}

func (h *Handler) del(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")
	if !h.store.Delete(id) {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")
	if !h.store.SetStatus(id, "online") {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) stop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")
	if !h.store.SetStatus(id, "draining") {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) listNotes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")
	notes := h.store.ListNotes(id)
	httputil.WriteJSON(w, http.StatusOK, notes)
}

func (h *Handler) addNote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")
	var req struct {
		Text string `json:"text"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil || req.Text == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "text is required")
		return
	}
	p, _ := httputil.PrincipalFromContext(r.Context())
	author := "system"
	if p != nil {
		author = p.Username
	}
	note := h.store.AddNote(id, Note{Author: author, Text: req.Text})
	httputil.WriteJSON(w, http.StatusCreated, note)
}
