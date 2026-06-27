package nmgr

import (
	"context"
	"net/http"
	"time"

	"github.com/asymmetric-effort/convocate/internal/db"
	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/k8s"
	"github.com/asymmetric-effort/convocate/internal/middleware"
	"github.com/asymmetric-effort/convocate/internal/types"
)

type Handler struct {
	store    *Store
	useK8s   bool
}

func Register(mux *http.ServeMux) {
	h := &Handler{
		store:  NewStore(),
		useK8s: k8s.Client != nil,
	}
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

	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		nodes, err := k8s.ListNodes(ctx)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "k8s_error", err.Error())
			return
		}
		for i := range nodes {
			count, _ := k8s.CountAgentPodsOnNode(ctx, nodes[i].ID)
			nodes[i].Agents = count
		}
		httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(nodes, offset, limit))
		return
	}

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

	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		node, err := k8s.GetNode(ctx, id)
		if err != nil {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		agents, _ := k8s.ListAgentPodsOnNode(ctx, id)
		node.Agents = len(agents)
		notes := h.getNotesFromDB(id)
		detail := types.NodeDetail{Node: *node, AgentList: agents, Notes: notes}
		httputil.WriteJSON(w, http.StatusOK, detail)
		return
	}

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
	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := k8s.UncordonNode(ctx, id); err != nil {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !h.store.SetStatus(id, "online") {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) stop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")
	if h.useK8s {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := k8s.CordonNode(ctx, id); err != nil {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !h.store.SetStatus(id, "draining") {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) listNotes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("nodeId")
	if db.Pool != nil {
		httputil.WriteJSON(w, http.StatusOK, h.getNotesFromDB(id))
		return
	}
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

	if db.Pool != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		var createdAt time.Time
		err := db.Pool.QueryRow(ctx,
			"INSERT INTO node_notes (node_id, author, text) VALUES ($1, $2, $3) RETURNING created_at",
			id, author, req.Text).Scan(&createdAt)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, types.Note{
			Author: author, CreatedAt: createdAt.UTC().Format(time.RFC3339), Text: req.Text,
		})
		return
	}

	note := h.store.AddNote(id, Note{Author: author, Text: req.Text})
	httputil.WriteJSON(w, http.StatusCreated, note)
}

func (h *Handler) getNotesFromDB(nodeID string) []types.Note {
	if db.Pool == nil {
		mockNotes := h.store.ListNotes(nodeID)
		var notes []types.Note
		for _, n := range mockNotes {
			notes = append(notes, types.Note{Author: n.Author, CreatedAt: n.CreatedAt, Text: n.Text})
		}
		return notes
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := db.Pool.Query(ctx,
		"SELECT author, created_at, text FROM node_notes WHERE node_id = $1 ORDER BY created_at", nodeID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var notes []types.Note
	for rows.Next() {
		var n types.Note
		var t time.Time
		if err := rows.Scan(&n.Author, &t, &n.Text); err == nil {
			n.CreatedAt = t.UTC().Format(time.RFC3339)
			notes = append(notes, n)
		}
	}
	if notes == nil {
		notes = []types.Note{}
	}
	return notes
}
