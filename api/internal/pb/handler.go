package pb

import (
	"net/http"
	"time"

	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/middleware"
)

type Handler struct{ store *Store }

func Register(mux *http.ServeMux) {
	h := &Handler{store: NewStore()}
	auth := middleware.Auth

	mux.Handle("GET /api/v1/pb/board", middleware.Chain(http.HandlerFunc(h.listBoards), auth, middleware.RBAC("pb-view")))
	mux.Handle("POST /api/v1/pb/board", middleware.Chain(http.HandlerFunc(h.createBoard), auth, middleware.RBAC("pb-update")))
	mux.Handle("GET /api/v1/pb/board/{boardId}", middleware.Chain(http.HandlerFunc(h.getBoard), auth, middleware.RBAC("pb-view")))
	mux.Handle("PATCH /api/v1/pb/board/{boardId}", middleware.Chain(http.HandlerFunc(h.renameBoard), auth, middleware.RBAC("pb-update")))
	mux.Handle("POST /api/v1/pb/board/{boardId}/save-as-repo", middleware.Chain(http.HandlerFunc(h.saveAsRepo), auth, middleware.RBAC("pb-update")))
	mux.Handle("POST /api/v1/pb/board/{boardId}/implement", middleware.Chain(http.HandlerFunc(h.implement), auth, middleware.RBAC("pb-execute")))
	mux.Handle("POST /api/v1/pb/board/{boardId}/container", middleware.Chain(http.HandlerFunc(h.createContainer), auth, middleware.RBAC("pb-update")))
	mux.Handle("PATCH /api/v1/pb/board/{boardId}/container/{containerId}", middleware.Chain(http.HandlerFunc(h.updateContainer), auth, middleware.RBAC("pb-update")))
	mux.Handle("DELETE /api/v1/pb/board/{boardId}/container/{containerId}", middleware.Chain(http.HandlerFunc(h.deleteContainer), auth, middleware.RBAC("pb-update")))
	mux.Handle("POST /api/v1/pb/board/{boardId}/card", middleware.Chain(http.HandlerFunc(h.createCard), auth, middleware.RBAC("pb-update")))
	mux.Handle("GET /api/v1/pb/board/{boardId}/card/{cardId}", middleware.Chain(http.HandlerFunc(h.getCard), auth, middleware.RBAC("pb-view")))
	mux.Handle("PUT /api/v1/pb/board/{boardId}/card/{cardId}", middleware.Chain(http.HandlerFunc(h.updateCard), auth, middleware.RBAC("pb-update")))
	mux.Handle("DELETE /api/v1/pb/board/{boardId}/card/{cardId}", middleware.Chain(http.HandlerFunc(h.deleteCard), auth, middleware.RBAC("pb-update")))
	mux.Handle("POST /api/v1/pb/board/{boardId}/card/{cardId}/send", middleware.Chain(http.HandlerFunc(h.sendCard), auth, middleware.RBAC("pb-execute")))
	mux.Handle("POST /api/v1/pb/board/{boardId}/edge", middleware.Chain(http.HandlerFunc(h.createEdge), auth, middleware.RBAC("pb-update")))
	mux.Handle("PATCH /api/v1/pb/board/{boardId}/edge/{edgeId}", middleware.Chain(http.HandlerFunc(h.updateEdge), auth, middleware.RBAC("pb-update")))
	mux.Handle("DELETE /api/v1/pb/board/{boardId}/edge/{edgeId}", middleware.Chain(http.HandlerFunc(h.deleteEdge), auth, middleware.RBAC("pb-update")))
}

func (h *Handler) listBoards(w http.ResponseWriter, r *http.Request) {
	offset, limit := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(h.store.ListBoards(), offset, limit))
}

func (h *Handler) createBoard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		RepoID string `json:"repoId"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, h.store.CreateBoard(req.Name, req.RepoID))
}

func (h *Handler) getBoard(w http.ResponseWriter, r *http.Request) {
	b, ok := h.store.GetBoard(r.PathValue("boardId"))
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "board not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, b)
}

func (h *Handler) renameBoard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	b, ok := h.store.RenameBoard(r.PathValue("boardId"), req.Name)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "board not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, b)
}

func (h *Handler) saveAsRepo(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteJSON(w, http.StatusCreated, map[string]string{"id": "repo-mock", "name": "saved-board"})
}

func (h *Handler) implement(w http.ResponseWriter, r *http.Request) {
	boardID := r.PathValue("boardId")
	run := ExecutionRun{ID: "run-001", BoardID: boardID, DispatchedCards: []string{"card-001", "card-002"}, StartedAt: time.Now().UTC().Format(time.RFC3339)}
	httputil.WriteJSON(w, http.StatusAccepted, run)
}

func (h *Handler) createContainer(w http.ResponseWriter, r *http.Request) {
	var req Container
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	c, ok := h.store.CreateContainer(r.PathValue("boardId"), req)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "board not found")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, c)
}

func (h *Handler) updateContainer(w http.ResponseWriter, r *http.Request) {
	var req Container
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	c, ok := h.store.UpdateContainer(r.PathValue("boardId"), r.PathValue("containerId"), req)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "container not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, c)
}

func (h *Handler) deleteContainer(w http.ResponseWriter, r *http.Request) {
	if !h.store.DeleteContainer(r.PathValue("boardId"), r.PathValue("containerId")) {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "container not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createCard(w http.ResponseWriter, r *http.Request) {
	var req Card
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	c, ok := h.store.CreateCard(r.PathValue("boardId"), req)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "board not found")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, c)
}

func (h *Handler) getCard(w http.ResponseWriter, r *http.Request) {
	c, ok := h.store.GetCard(r.PathValue("boardId"), r.PathValue("cardId"))
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "card not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, c)
}

func (h *Handler) updateCard(w http.ResponseWriter, r *http.Request) {
	var req Card
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	req.ID = r.PathValue("cardId")
	c, ok := h.store.UpdateCard(r.PathValue("boardId"), req)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "card not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, c)
}

func (h *Handler) deleteCard(w http.ResponseWriter, r *http.Request) {
	if !h.store.DeleteCard(r.PathValue("boardId"), r.PathValue("cardId")) {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "card not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) sendCard(w http.ResponseWriter, r *http.Request) {
	c, ok := h.store.GetCard(r.PathValue("boardId"), r.PathValue("cardId"))
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "card not found")
		return
	}
	c.Status = "active"
	h.store.UpdateCard(r.PathValue("boardId"), c)
	httputil.WriteJSON(w, http.StatusAccepted, c)
}

func (h *Handler) createEdge(w http.ResponseWriter, r *http.Request) {
	var req Edge
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	e, ok := h.store.CreateEdge(r.PathValue("boardId"), req)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "board not found")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, e)
}

func (h *Handler) updateEdge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type string `json:"type"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	e, ok := h.store.UpdateEdge(r.PathValue("boardId"), r.PathValue("edgeId"), req.Type)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "edge not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, e)
}

func (h *Handler) deleteEdge(w http.ResponseWriter, r *http.Request) {
	if !h.store.DeleteEdge(r.PathValue("boardId"), r.PathValue("edgeId")) {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "edge not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
