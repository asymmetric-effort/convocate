package repo

import (
	"net/http"

	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/middleware"
)

type Handler struct{ store *Store }

func Register(mux *http.ServeMux) {
	h := &Handler{store: NewStore()}
	auth := middleware.Auth

	mux.Handle("GET /api/v1/repo/repo", middleware.Chain(http.HandlerFunc(h.listRepos), auth, middleware.RBAC("repo-view")))
	mux.Handle("POST /api/v1/repo/repo", middleware.Chain(http.HandlerFunc(h.createRepo), auth, middleware.RBAC("repo-update")))
	mux.Handle("GET /api/v1/repo/repo/{repoId}/file", middleware.Chain(http.HandlerFunc(h.listFiles), auth, middleware.RBAC("repo-view")))
	mux.Handle("GET /api/v1/repo/repo/{repoId}/pr", middleware.Chain(http.HandlerFunc(h.listPRs), auth, middleware.RBAC("repo-view")))
	mux.Handle("GET /api/v1/repo/repo/{repoId}/pr/{prId}", middleware.Chain(http.HandlerFunc(h.getPR), auth, middleware.RBAC("repo-view")))
	mux.Handle("POST /api/v1/repo/repo/{repoId}/pr/{prId}/merge", middleware.Chain(http.HandlerFunc(h.mergePR), auth, middleware.RBAC("repo-merge")))
}

func (h *Handler) listRepos(w http.ResponseWriter, r *http.Request) {
	offset, limit := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(h.store.ListRepos(), offset, limit))
}

func (h *Handler) createRepo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		Visibility string `json:"visibility"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, h.store.CreateRepo(req.Name, req.Visibility))
}

func (h *Handler) listFiles(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, h.store.ListFiles(r.PathValue("repoId")))
}

func (h *Handler) listPRs(w http.ResponseWriter, r *http.Request) {
	offset, limit := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(h.store.ListPRs(r.PathValue("repoId")), offset, limit))
}

func (h *Handler) getPR(w http.ResponseWriter, r *http.Request) {
	pr, ok := h.store.GetPR(r.PathValue("repoId"), r.PathValue("prId"))
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "pull request not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, pr)
}

func (h *Handler) mergePR(w http.ResponseWriter, r *http.Request) {
	pr, ok := h.store.MergePR(r.PathValue("repoId"), r.PathValue("prId"))
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "pull request not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, pr)
}
