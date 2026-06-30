package auth

import (
	"net/http"
	"time"

	"github.com/asymmetric-effort/convocate/internal/httputil"
	"github.com/asymmetric-effort/convocate/internal/middleware"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	MFAToken string `json:"mfaToken"`
}

type session struct {
	AccessToken  string             `json:"accessToken"`
	RefreshToken string             `json:"refreshToken"`
	ExpiresAt    string             `json:"expiresAt"`
	Principal    httputil.Principal `json:"principal"`
}

func Register(mux *http.ServeMux) {
	// Public (unauthenticated)
	mux.HandleFunc("POST /api/v1/auth/login", handleLogin)
	mux.HandleFunc("GET /api/v1/auth/oidc/github/start", handleOIDCStart)
	mux.HandleFunc("GET /api/v1/auth/oidc/github/callback", handleOIDCCallback)

	// Authenticated
	mux.Handle("POST /api/v1/auth/refresh", middleware.Chain(
		http.HandlerFunc(handleRefresh), middleware.Auth))
	mux.Handle("POST /api/v1/auth/logout", middleware.Chain(
		http.HandlerFunc(handleLogout), middleware.Auth))
	mux.Handle("GET /api/v1/auth/me", middleware.Chain(
		http.HandlerFunc(handleMe), middleware.Auth))
}

func mockSession() session {
	return session{
		AccessToken:  "mock-jwt-token-convocate",
		RefreshToken: "mock-refresh-token",
		ExpiresAt:    time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		Principal: httputil.Principal{
			ID:                "usr-mock-admin",
			Username:          "admin",
			Name:              "Mock Admin",
			Email:             "admin@convocate.local",
			Groups:            []string{"admins"},
			Roles:             []string{"admin"},
			IDP:               "local",
			AuthorizedApplets: []string{"nmgr", "amgr", "pb", "ide", "repo", "ac", "sup"},
		},
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httputil.ReadJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, mockSession())
}

func handleOIDCStart(w http.ResponseWriter, _ *http.Request) {
	http.Redirect(w, &http.Request{}, "https://github.com/login/oauth/authorize?client_id=mock", http.StatusFound)
}

func handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing code parameter")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, mockSession())
}

func handleRefresh(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, mockSession())
}

func handleLogout(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	p, ok := httputil.PrincipalFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "no principal in context")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, p)
}
