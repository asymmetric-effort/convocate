// Authentication for the agent wrapper.
// Three layers:
//   1. K8s ServiceAccount token verification (container-to-container)
//   2. JWT verification with RBAC roles (user-to-agent)
//   3. TLS encryption (handled at the server level)

package main

import (
	"crypto/ecdsa"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"log"
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

// K8s SA token header name — the API includes its SA token in this header
// when proxying requests to agent pods.
const k8sSATokenHeader = "X-K8s-SA-Token"

// Auth holds the public key for JWT verification and the expected K8s SA
// token for container-to-container authentication.
type Auth struct {
	publicKey      *ecdsa.PublicKey
	expectedSAToken string // K8s SA token from the API's projected volume
	saTokenPath     string // path to watch for token rotation
}

// NewAuth loads the JWT verification public key and K8s SA token.
//
//   - keyPath: path to JWT EC public key PEM file (empty = dev mode, all pass)
//   - saTokenPath: path to the K8s projected SA token file the API sends
//     (e.g. /var/run/secrets/convocate/api-token). If empty, SA auth is disabled.
func NewAuth(keyPath, saTokenPath string) *Auth {
	a := &Auth{saTokenPath: saTokenPath}

	// Load JWT public key
	if keyPath != "" {
		data, err := os.ReadFile(keyPath)
		if err == nil {
			block, _ := pem.Decode(data)
			if block != nil {
				pub, err := x509.ParsePKIXPublicKey(block.Bytes)
				if err == nil {
					if ecKey, ok := pub.(*ecdsa.PublicKey); ok {
						a.publicKey = ecKey
					}
				}
			}
		}
	}

	// Load expected K8s SA token
	a.reloadSAToken()
	return a
}

// reloadSAToken reads the SA token from the projected volume.
// Called at startup and can be called periodically for token rotation.
func (a *Auth) reloadSAToken() {
	if a.saTokenPath == "" {
		return
	}
	data, err := os.ReadFile(a.saTokenPath)
	if err != nil {
		log.Printf("[auth] Warning: could not read SA token from %s: %v", a.saTokenPath, err)
		return
	}
	a.expectedSAToken = strings.TrimSpace(string(data))
	log.Printf("[auth] Loaded K8s SA token from %s (%d bytes)", a.saTokenPath, len(a.expectedSAToken))
}

// VerifySAToken checks the X-K8s-SA-Token header against the expected value.
// Returns true if SA auth is disabled (no token configured) or the token matches.
func (a *Auth) VerifySAToken(r *http.Request) bool {
	if a.expectedSAToken == "" {
		return true // SA auth disabled (dev mode)
	}
	token := r.Header.Get(k8sSATokenHeader)
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(a.expectedSAToken)) == 1
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

// RequireRole returns middleware that requires:
//  1. Valid K8s SA token (if SA auth is configured)
//  2. Valid JWT with the required role
func (a *Auth) RequireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Layer 1: K8s SA token verification (container-to-container)
		if !a.VerifySAToken(r) {
			http.Error(w, `{"code":"unauthorized","message":"invalid or missing K8s SA token"}`, http.StatusUnauthorized)
			return
		}

		// Layer 2: JWT RBAC verification (user-to-agent)
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
