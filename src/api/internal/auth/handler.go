package auth

import (
	"log"
	"net/http"
	"strings"
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

	// Authenticated
	mux.Handle("POST /api/v1/auth/refresh", middleware.Chain(
		http.HandlerFunc(handleRefresh), middleware.Auth))
	mux.Handle("POST /api/v1/auth/logout", middleware.Chain(
		http.HandlerFunc(handleLogout), middleware.Auth))
	mux.Handle("GET /api/v1/auth/me", middleware.Chain(
		http.HandlerFunc(handleMe), middleware.Auth))
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

	// Route through SAML/SCIM agent when configured
	if agentURL := samlAgentURL(); agentURL != "" {
		principal, err := samlLogin(agentURL, req.Username, req.Password)
		if err != nil {
			log.Printf("SAML login failed for user %q: %v", req.Username, err)
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication failed")
			return
		}
		// Sign JWT with the SAML-derived principal
		token, expiresAt, err := SignJWT(principal.ID, principal.Username, principal.Name, principal.Email, principal.Roles, principal.AuthorizedApplets, 24*time.Hour)
		if err != nil {
			log.Printf("JWT signing failed: %v", err)
			httputil.WriteError(w, http.StatusInternalServerError, "server_error", "internal error")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, session{
			AccessToken:  token,
			RefreshToken: "",
			ExpiresAt:    expiresAt.UTC().Format(time.RFC3339),
			Principal:    *principal,
		})
		return
	}

	// Authenticate via OpenBao userpass
	loginResp, err := openbaoLogin(req.Username, req.Password)
	if err != nil {
		log.Printf("OpenBao login failed for user %q: %v", req.Username, err)
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "User login failed.")
		return
	}

	// Handle MFA two-step flow: userpass login may return mfa_requirement instead of a token
	if loginResp.Auth.MFARequirement != nil && loginResp.Auth.MFARequirement.MFARequestID != "" {
		if req.MFAToken == "" {
			httputil.WriteError(w, http.StatusUnauthorized, "mfa_required", "MFA code required")
			return
		}
		methodID := openbaoMFAMethodID()
		if methodID == "" {
			log.Printf("OPENBAO_MFA_METHOD_ID not configured")
			httputil.WriteError(w, http.StatusInternalServerError, "server_error", "MFA not configured")
			return
		}
		mfaResp, err := openbaoMFAValidate(loginResp.Auth.MFARequirement.MFARequestID, methodID, req.MFAToken)
		if err != nil {
			log.Printf("MFA validation failed for user %q: %v", req.Username, err)
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "MFA validation failed.")
			return
		}
		loginResp = mfaResp
	}

	token := loginResp.Auth.ClientToken
	entityID := loginResp.Auth.EntityID

	// Look up entity for metadata
	var principal *httputil.Principal
	if entityID != "" {
		entity, err := openbaoLookupEntity(token, entityID)
		if err == nil {
			principal = buildPrincipalFromEntity(entity, loginResp.Auth.Policies)
		}
	}

	// Fallback: build principal from login response metadata if entity lookup failed
	if principal == nil {
		roles := make([]string, 0, len(loginResp.Auth.Policies))
		for _, p := range loginResp.Auth.Policies {
			if p != "" {
				roles = append(roles, p)
			}
		}
		principal = &httputil.Principal{
			ID:                entityID,
			Username:          req.Username,
			Name:              loginResp.Auth.Metadata["name"],
			Email:             loginResp.Auth.Metadata["email"],
			Roles:             roles,
			IDP:               "openbao",
			AuthorizedApplets: rolesToApplets(roles),
		}
	}

	expiresAt := time.Now().Add(time.Duration(loginResp.Auth.LeaseDuration) * time.Second)
	if loginResp.Auth.LeaseDuration == 0 {
		expiresAt = time.Now().Add(24 * time.Hour)
	}

	httputil.WriteJSON(w, http.StatusOK, session{
		AccessToken:  token,
		RefreshToken: "",
		ExpiresAt:    expiresAt.UTC().Format(time.RFC3339),
		Principal:    *principal,
	})
}

func handleRefresh(w http.ResponseWriter, r *http.Request) {
	// Extract the current token and validate it is still valid
	auth := r.Header.Get("Authorization")
	token := strings.TrimPrefix(auth, "Bearer ")

	lookupResp, err := openbaoTokenLookupSelf(token)
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "token refresh failed")
		return
	}

	// Look up entity for current metadata
	var principal *httputil.Principal
	if lookupResp.Data.EntityID != "" {
		entity, err := openbaoLookupEntity(token, lookupResp.Data.EntityID)
		if err == nil {
			principal = buildPrincipalFromEntity(entity, lookupResp.Data.Policies)
		}
	}

	if principal == nil {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "token refresh failed")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, session{
		AccessToken:  token,
		RefreshToken: "",
		ExpiresAt:    time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		Principal:    *principal,
	})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	// Extract the token and revoke it in OpenBao
	auth := r.Header.Get("Authorization")
	token := strings.TrimPrefix(auth, "Bearer ")

	// Best-effort revocation; respond 204 regardless
	if token != "" {
		_ = openbaoRevokeSelf(token)
	}

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
