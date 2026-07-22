package main

import (
	"encoding/json"
	"net/http"

	"github.com/asymmetric-effort/convocate/src/saml-scim-agent/openbao"
)

// HealthHandler serves the /health endpoint.
type HealthHandler struct {
	Client *openbao.Client
}

// ServeHTTP handles health check requests.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := h.Client.CheckHealth()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "unavailable",
			"error":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
