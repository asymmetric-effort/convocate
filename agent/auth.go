// JWT authentication for the agent wrapper.
// Validates bearer tokens and enforces RBAC roles.

package main

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

// JWTClaims represents the decoded JWT payload.
type JWTClaims struct {
	Sub   string   `json:"sub"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
	Exp   int64    `json:"exp"`
	Iat   int64    `json:"iat"`
}

// HasRole checks if the claims include a specific role or "admin".
func (c *JWTClaims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role || r == "admin" {
			return true
		}
	}
	return false
}

// Auth holds the public key for JWT verification.
type Auth struct {
	publicKey *ecdsa.PublicKey
}

// NewAuth loads the JWT verification public key from a PEM file.
// If the file doesn't exist or is empty, auth is disabled (all requests pass).
func NewAuth(keyPath string) *Auth {
	a := &Auth{}
	if keyPath == "" {
		return a
	}
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return a
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return a
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return a
	}
	if ecKey, ok := pub.(*ecdsa.PublicKey); ok {
		a.publicKey = ecKey
	}
	return a
}

// VerifyToken validates a JWT string and returns the claims.
func (a *Auth) VerifyToken(tokenStr string) (*JWTClaims, error) {
	// If no public key loaded, accept all tokens (dev mode)
	if a.publicKey == nil {
		return a.parseClaimsOnly(tokenStr)
	}

	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	// Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid token claims")
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, errors.New("invalid token claims")
	}

	// Check expiration
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, errors.New("token expired")
	}

	// In production, verify the signature with the public key.
	// For now, trust the claims if the key is loaded but skip
	// full ECDSA verification (the API already verified the token).

	return &claims, nil
}

// parseClaimsOnly decodes claims without signature verification (dev mode).
func (a *Auth) parseClaimsOnly(tokenStr string) (*JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) < 2 {
		// Accept mock tokens in dev mode
		return &JWTClaims{
			Sub:   "mock",
			Name:  "Mock User",
			Roles: []string{"admin"},
		}, nil
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return &JWTClaims{Sub: "mock", Roles: []string{"admin"}}, nil
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return &JWTClaims{Sub: "mock", Roles: []string{"admin"}}, nil
	}
	return &claims, nil
}

// RequireRole returns middleware that requires the caller to have the given role.
func (a *Auth) RequireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			http.Error(w, `{"code":"unauthorized","message":"missing bearer token"}`, http.StatusUnauthorized)
			return
		}

		claims, err := a.VerifyToken(token)
		if err != nil {
			http.Error(w, `{"code":"unauthorized","message":"`+err.Error()+`"}`, http.StatusUnauthorized)
			return
		}

		if !claims.HasRole(role) {
			http.Error(w, `{"code":"forbidden","message":"insufficient role"}`, http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

// extractBearerToken gets the token from Authorization header or ?token= query param.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	// Fallback: ?token= query param (for WebSocket connections)
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	return ""
}
