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
	mux.Handle("POST /api/v1/ac/user/{userId}/mfa/enroll", middleware.Chain(http.HandlerFunc(h.enrollMFA), auth, update))
	mux.Handle("DELETE /api/v1/ac/user/{userId}/mfa", middleware.Chain(http.HandlerFunc(h.destroyMFA), auth, update))
	mux.Handle("GET /api/v1/ac/user/{userId}/mfa/status", middleware.Chain(http.HandlerFunc(h.mfaStatus), auth, update))
	mux.Handle("GET /api/v1/ac/settings", middleware.Chain(http.HandlerFunc(h.getSettings), auth, view))
	mux.Handle("PUT /api/v1/ac/settings", middleware.Chain(http.HandlerFunc(h.putSettings), auth, update))
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers()
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "backend_error", err.Error())
		return
	}
	offset, limit := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(users, offset, limit))
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var req User
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	u, err := h.store.CreateUser(req)
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "backend_error", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, u)
}

func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	var req User
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	u, ok, err := h.store.UpdateUser(r.PathValue("userId"), req)
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "backend_error", err.Error())
		return
	}
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	ok, err := h.store.DeleteUser(r.PathValue("userId"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "backend_error", err.Error())
		return
	}
	if !ok {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.ListGroups()
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "backend_error", err.Error())
		return
	}
	offset, limit := httputil.ParsePagination(r)
	httputil.WriteJSON(w, http.StatusOK, httputil.Paginate(groups, offset, limit))
}

func (h *Handler) createGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	g, err := h.store.CreateGroup(req.Name)
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "backend_error", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, g)
}

func (h *Handler) deleteGroup(w http.ResponseWriter, r *http.Request) {
	ok, err := h.store.DeleteGroup(r.PathValue("groupId"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "backend_error", err.Error())
		return
	}
	if !ok {
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
	g, ok, err := h.store.SetGroupUsers(r.PathValue("groupId"), req.UserIDs)
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "backend_error", err.Error())
		return
	}
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
	g, ok, err := h.store.SetGroupRoles(r.PathValue("groupId"), req.Roles)
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "backend_error", err.Error())
		return
	}
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
	gs, err := h.store.GetSettings()
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "backend_error", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, gs)
}

func (h *Handler) putSettings(w http.ResponseWriter, r *http.Request) {
	var req GlobalSettings
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	gs, err := h.store.SetSettings(req)
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "backend_error", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, gs)
}

func (h *Handler) enrollMFA(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("userId")
	result, err := h.store.EnrollMFA(entityID)
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "backend_error", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) destroyMFA(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("userId")
	if err := h.store.DestroyMFA(entityID); err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "backend_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) mfaStatus(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("userId")
	enrolled, err := h.store.GetMFAStatus(entityID)
	if err != nil {
		httputil.WriteError(w, http.StatusBadGateway, "backend_error", err.Error())
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]bool{"enrolled": enrolled})
}
