package ac

import (
	"net/http"

	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/middleware"
)

type Handler struct{ store *Store }

func Register(mux *http.ServeMux) {
	h := &Handler{store: NewStore()}
	auth := middleware.Auth
	view := middleware.RBAC("access-view")
	update := middleware.RBAC("access-update")

	mux.Handle("GET /api/v1/ac/user", middleware.Chain(http.HandlerFunc(h.listUsers), auth, view))
	mux.Handle("POST /api/v1/ac/user", middleware.Chain(http.HandlerFunc(h.createUser), auth, update))
	mux.Handle("PATCH /api/v1/ac/user/{userId}", middleware.Chain(http.HandlerFunc(h.updateUser), auth, update))
	mux.Handle("DELETE /api/v1/ac/user/{userId}", middleware.Chain(http.HandlerFunc(h.deleteUser), auth, update))
	mux.Handle("GET /api/v1/ac/group", middleware.Chain(http.HandlerFunc(h.listGroups), auth, view))
	mux.Handle("POST /api/v1/ac/group", middleware.Chain(http.HandlerFunc(h.createGroup), auth, update))
	mux.Handle("DELETE /api/v1/ac/group/{groupId}", middleware.Chain(http.HandlerFunc(h.deleteGroup), auth, update))
	mux.Handle("PUT /api/v1/ac/group/{groupId}/user", middleware.Chain(http.HandlerFunc(h.setGroupUsers), auth, update))
	mux.Handle("PUT /api/v1/ac/group/{groupId}/role", middleware.Chain(http.HandlerFunc(h.setGroupRoles), auth, update))
	mux.Handle("GET /api/v1/ac/role", middleware.Chain(http.HandlerFunc(h.listRoles), auth, view))
	mux.Handle("GET /api/v1/ac/settings", middleware.Chain(http.HandlerFunc(h.getSettings), auth, view))
	mux.Handle("PUT /api/v1/ac/settings", middleware.Chain(http.HandlerFunc(h.putSettings), auth, update))
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	offset, limit := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(h.store.ListUsers(), offset, limit))
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var req User
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, h.store.CreateUser(req))
}

func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	var req User
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	u, ok := h.store.UpdateUser(r.PathValue("userId"), req)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	if !h.store.DeleteUser(r.PathValue("userId")) {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listGroups(w http.ResponseWriter, r *http.Request) {
	offset, limit := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(h.store.ListGroups(), offset, limit))
}

func (h *Handler) createGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, h.store.CreateGroup(req.Name))
}

func (h *Handler) deleteGroup(w http.ResponseWriter, r *http.Request) {
	if !h.store.DeleteGroup(r.PathValue("groupId")) {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "group not found or is builtin")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setGroupUsers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserIDs []string `json:"userIds"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	g, ok := h.store.SetGroupUsers(r.PathValue("groupId"), req.UserIDs)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "group not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, g)
}

func (h *Handler) setGroupRoles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Roles []string `json:"roles"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	g, ok := h.store.SetGroupRoles(r.PathValue("groupId"), req.Roles)
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "group not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, g)
}

func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	offset, limit := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(h.store.ListRoles(), offset, limit))
}

func (h *Handler) getSettings(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, h.store.GetSettings())
}

func (h *Handler) putSettings(w http.ResponseWriter, r *http.Request) {
	var req GlobalSettings
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, h.store.SetSettings(req))
}
