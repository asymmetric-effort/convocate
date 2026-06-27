package ide

import (
	"net/http"

	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/llm"
	"github.com/asymmetric-effort/convocate/internal/middleware"
)

type Handler struct{ store *Store }

func Register(mux *http.ServeMux) {
	h := &Handler{store: NewStore()}
	auth := middleware.Auth

	mux.Handle("GET /api/v1/ide/project", middleware.Chain(http.HandlerFunc(h.listProjects), auth, middleware.RBAC("ide-view")))
	mux.Handle("POST /api/v1/ide/project", middleware.Chain(http.HandlerFunc(h.createProject), auth, middleware.RBAC("ide-update")))
	mux.Handle("GET /api/v1/ide/project/{projectId}/tree", middleware.Chain(http.HandlerFunc(h.tree), auth, middleware.RBAC("ide-view")))
	mux.Handle("GET /api/v1/ide/project/{projectId}/file/{path...}", middleware.Chain(http.HandlerFunc(h.getFile), auth, middleware.RBAC("ide-view")))
	mux.Handle("PUT /api/v1/ide/project/{projectId}/file/{path...}", middleware.Chain(http.HandlerFunc(h.putFile), auth, middleware.RBAC("ide-update")))
	mux.Handle("DELETE /api/v1/ide/project/{projectId}/file/{path...}", middleware.Chain(http.HandlerFunc(h.deleteFile), auth, middleware.RBAC("ide-update")))
	mux.Handle("POST /api/v1/ide/project/{projectId}/rename-file", middleware.Chain(http.HandlerFunc(h.renameFile), auth, middleware.RBAC("ide-update")))
	mux.Handle("POST /api/v1/ide/project/{projectId}/render-board", middleware.Chain(http.HandlerFunc(h.renderBoard), auth, middleware.RBAC("ide-update")))
}

func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request) {
	offset, limit := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(h.store.ListProjects(), offset, limit))
}

func (h *Handler) createProject(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name string `json:"name"` }
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, h.store.CreateProject(req.Name))
}

func (h *Handler) tree(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, h.store.ListTree(r.PathValue("projectId")))
}

func (h *Handler) getFile(w http.ResponseWriter, r *http.Request) {
	f, ok := h.store.GetFile(r.PathValue("projectId"), r.PathValue("path"))
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "file not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, f)
}

func (h *Handler) putFile(w http.ResponseWriter, r *http.Request) {
	var req struct{ Content string `json:"content"` }
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	f := h.store.PutFile(r.PathValue("projectId"), r.PathValue("path"), req.Content)
	httputil.WriteJSON(w, http.StatusOK, f)
}

func (h *Handler) deleteFile(w http.ResponseWriter, r *http.Request) {
	if !h.store.DeleteFile(r.PathValue("projectId"), r.PathValue("path")) {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "file not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) renameFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldPath string `json:"oldPath"`
		NewPath string `json:"newPath"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	f, ok := h.store.RenameFile(r.PathValue("projectId"), req.OldPath, req.NewPath)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "file not found or destination exists")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, f)
}

func (h *Handler) renderBoard(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	// Find the project's spec file
	spec, ok := h.store.GetFile(projectID, "SPECIFICATION.md")
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "SPECIFICATION.md not found in project")
		return
	}

	board, err := llm.DecomposeSpec(spec.Content)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "llm_error", err.Error())
		return
	}

	board.BoardSummary.ID = "brd-rendered"
	board.BoardSummary.Name = "Rendered Board"

	httputil.WriteJSON(w, http.StatusAccepted, board)
}
