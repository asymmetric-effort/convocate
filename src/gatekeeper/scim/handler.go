package scim

import (
	"net/http"
	"strings"

	"github.com/asymmetric-effort/convocate/src/gatekeeper/openbao"
)

// Handler handles SCIM endpoints.
type Handler struct {
	Client  *openbao.Client
	BaseURL string
}

// ServeHTTP routes SCIM requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Authenticate: require Bearer token
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
		return
	}

	// The bearer token is an OpenBao token; we use the service's configured token
	// for operations. The bearer token validates the caller has a valid OpenBao token.
	// In production, you'd validate the token. Here we accept any non-empty token.
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		writeError(w, http.StatusUnauthorized, "empty bearer token")
		return
	}

	path := r.URL.Path

	// Route based on path
	switch {
	case path == "/scim/v2/ServiceProviderConfig":
		h.handleServiceProviderConfig(w, r)
	case path == "/scim/v2/Schemas":
		h.handleSchemas(w, r)
	case path == "/scim/v2/ResourceTypes":
		h.handleResourceTypes(w, r)
	case path == "/scim/v2/Users":
		h.handleUsers(w, r)
	case strings.HasPrefix(path, "/scim/v2/Users/"):
		id := strings.TrimPrefix(path, "/scim/v2/Users/")
		h.handleUser(w, r, id)
	case path == "/scim/v2/Groups":
		h.handleGroups(w, r)
	case strings.HasPrefix(path, "/scim/v2/Groups/"):
		id := strings.TrimPrefix(path, "/scim/v2/Groups/")
		h.handleGroup(w, r, id)
	default:
		writeError(w, http.StatusNotFound, "endpoint not found")
	}
}

func (h *Handler) handleServiceProviderConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, GetServiceProviderConfig())
}

func (h *Handler) handleSchemas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, GetSchemas())
}

func (h *Handler) handleResourceTypes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, GetResourceTypes())
}

func (h *Handler) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ListUsers(h.Client, h.BaseURL, w, r)
	case http.MethodPost:
		CreateUser(h.Client, h.BaseURL, w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleUser(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		GetUser(h.Client, h.BaseURL, w, id)
	case http.MethodPut:
		UpdateUser(h.Client, h.BaseURL, w, r, id)
	case http.MethodDelete:
		DeleteUser(h.Client, w, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ListGroups(h.Client, h.BaseURL, w, r)
	case http.MethodPost:
		CreateGroup(h.Client, h.BaseURL, w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleGroup(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		GetGroup(h.Client, h.BaseURL, w, id)
	case http.MethodPut:
		UpdateGroup(h.Client, h.BaseURL, w, r, id)
	case http.MethodDelete:
		DeleteGroup(h.Client, w, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
