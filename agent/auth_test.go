package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// testAuthSetup holds test auth infrastructure for creating valid tokens.
type testAuthSetup struct {
	Auth     *Auth
	PrivKey  *ecdsa.PrivateKey
	SAToken  string
	JWTToken string // a valid signed JWT with admin role
}

// newTestAuth creates a full Auth setup with a real ECDSA key and SA token.
// Returns the Auth, SA token value, and a signed JWT token string.
func newTestAuth(t *testing.T) testAuthSetup {
	t.Helper()
	dir := t.TempDir()

	// Generate ECDSA P-256 key pair
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// Write public key PEM
	pubDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	keyPath := dir + "/pub.pem"
	os.WriteFile(keyPath, pubPEM, 0644)

	// Write SA token
	saToken := "test-sa-token-" + dir
	saPath := dir + "/sa-token"
	os.WriteFile(saPath, []byte(saToken), 0644)

	// Create Auth
	a := NewAuth(keyPath, saPath)

	// Sign a JWT
	claims := JWTClaims{
		Sub:   "test-user",
		Name:  "Test User",
		Roles: []string{"admin"},
		Exp:   time.Now().Add(time.Hour).Unix(),
		Iat:   time.Now().Unix(),
	}
	token := signTestJWT(t, privKey, claims)

	return testAuthSetup{Auth: a, PrivKey: privKey, SAToken: saToken, JWTToken: token}
}

// signTestJWT signs a JWT using the given private key and claims.
func signTestJWT(t *testing.T, key *ecdsa.PrivateKey, claims JWTClaims) string {
	t.Helper()
	headerJSON, _ := json.Marshal(map[string]string{"alg": "ES256", "typ": "JWT"})
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := headerB64 + "." + claimsB64

	hash := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, key, hash[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)

	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return signingInput + "." + sigB64
}

// addAuthHeaders adds valid SA token and JWT bearer headers to a request.
func (ts testAuthSetup) addAuthHeaders(r *http.Request) {
	r.Header.Set("X-K8s-SA-Token", ts.SAToken)
	r.Header.Set("Authorization", "Bearer "+ts.JWTToken)
}

// authWSQuery returns query params for WebSocket connections with valid auth.
func (ts testAuthSetup) authWSQuery() string {
	return "token=" + ts.JWTToken
}

// wsUpgradeReq builds a raw WebSocket upgrade HTTP request with auth headers.
func (ts testAuthSetup) wsUpgradeReq(path, host string) string {
	return "GET " + path + "?" + ts.authWSQuery() + " HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"X-K8s-SA-Token: " + ts.SAToken + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
}

func TestNewAuth_NoKey(t *testing.T) {
	a := NewAuth("", "")
	if a.publicKey != nil {
		t.Error("publicKey should be nil when no key path provided")
	}
	if a.expectedSAToken != "" {
		t.Error("expectedSAToken should be empty when no path provided")
	}
}

func TestNewAuth_InvalidPath(t *testing.T) {
	a := NewAuth("/nonexistent/path", "")
	if a.publicKey != nil {
		t.Error("publicKey should be nil for missing file")
	}
}

func TestAuth_VerifyToken_NoKey(t *testing.T) {
	a := NewAuth("", "") // no key — should fail closed
	_, err := a.VerifyToken("mock-token")
	if err == nil {
		t.Fatal("VerifyToken should fail when no public key is configured")
	}
	if err.Error() != "no public key configured" {
		t.Errorf("error = %q, want %q", err.Error(), "no public key configured")
	}
}

func TestAuth_VerifyToken_InvalidFormat(t *testing.T) {
	s := newTestAuth(t)
	_, err := s.Auth.VerifyToken("not-a-jwt")
	if err == nil || err.Error() != "invalid token format" {
		t.Errorf("expected 'invalid token format', got %v", err)
	}
}

func TestAuth_VerifyToken_InvalidSignatureEncoding(t *testing.T) {
	s := newTestAuth(t)
	// Valid header.claims but garbage signature
	_, err := s.Auth.VerifyToken("eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJ0ZXN0In0.!!!invalid!!!")
	if err == nil || err.Error() != "invalid signature encoding" {
		t.Errorf("expected 'invalid signature encoding', got %v", err)
	}
}

func TestAuth_VerifyToken_InvalidSignature(t *testing.T) {
	s := newTestAuth(t)
	// Use a valid format but wrong signature (64 zero bytes)
	zerSig := base64.RawURLEncoding.EncodeToString(make([]byte, 64))
	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256","typ":"JWT"}`))
	claimsB64 := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"test","exp":9999999999}`))
	token := headerB64 + "." + claimsB64 + "." + zerSig
	_, err := s.Auth.VerifyToken(token)
	if err == nil || err.Error() != "invalid signature" {
		t.Errorf("expected 'invalid signature', got %v", err)
	}
}

func TestAuth_VerifyToken_ExpiredToken(t *testing.T) {
	s := newTestAuth(t)
	// Sign a token with expired claims
	token := signTestJWT(t, s.PrivKey, JWTClaims{
		Sub:   "test",
		Roles: []string{"admin"},
		Exp:   time.Now().Add(-1 * time.Hour).Unix(),
	})
	_, err := s.Auth.VerifyToken(token)
	if err == nil || err.Error() != "token expired" {
		t.Errorf("expected 'token expired', got %v", err)
	}
}

func TestAuth_VerifyToken_ValidToken(t *testing.T) {
	s := newTestAuth(t)
	claims, err := s.Auth.VerifyToken(s.JWTToken)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}
	if claims.Sub != "test-user" {
		t.Errorf("Sub = %q, want %q", claims.Sub, "test-user")
	}
	if !claims.HasRole("admin") {
		t.Error("expected admin role")
	}
}

func TestJWTClaims_HasRole(t *testing.T) {
	tests := []struct {
		roles []string
		check string
		want  bool
	}{
		{[]string{"agent-view"}, "agent-view", true},
		{[]string{"agent-view"}, "agent-update", false},
		{[]string{"admin"}, "agent-view", true},
		{[]string{"admin"}, "anything", true},
		{[]string{}, "agent-view", false},
		{[]string{"node-view", "agent-update"}, "agent-update", true},
	}
	for _, tt := range tests {
		c := &JWTClaims{Roles: tt.roles}
		got := c.HasRole(tt.check)
		if got != tt.want {
			t.Errorf("HasRole(%q) with roles %v = %v, want %v", tt.check, tt.roles, got, tt.want)
		}
	}
}

func TestAuth_RequireRole_MissingToken(t *testing.T) {
	// Need SA token configured for RequireRole to pass SA check
	dir := t.TempDir()
	tokenPath := dir + "/token"
	os.WriteFile(tokenPath, []byte("sa-token"), 0644)
	a := NewAuth("", tokenPath)
	handler := a.RequireRole("agent-view", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("X-K8s-SA-Token", "sa-token")
	// No Authorization header
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuth_RequireRole_NoKey_FailsClosed(t *testing.T) {
	// No public key configured — should reject even with a bearer token
	dir := t.TempDir()
	tokenPath := dir + "/token"
	os.WriteFile(tokenPath, []byte("sa-token"), 0644)
	a := NewAuth("", tokenPath)
	handler := a.RequireRole("agent-view", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	req.Header.Set("X-K8s-SA-Token", "sa-token")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// ---------------------------------------------------------------------------
// K8s SA token tests
// ---------------------------------------------------------------------------

func TestAuth_VerifySAToken_NoToken_FailsClosed(t *testing.T) {
	a := NewAuth("", "") // no SA token configured
	req := httptest.NewRequest("GET", "/", nil)
	if a.VerifySAToken(req) {
		t.Error("VerifySAToken should deny when no SA token is configured")
	}
}

func TestAuth_VerifySAToken_Valid(t *testing.T) {
	// Write a test SA token file
	dir := t.TempDir()
	tokenPath := dir + "/token"
	os.WriteFile(tokenPath, []byte("test-sa-token-xyz"), 0644)

	a := NewAuth("", tokenPath)
	if a.expectedSAToken != "test-sa-token-xyz" {
		t.Errorf("expectedSAToken = %q, want %q", a.expectedSAToken, "test-sa-token-xyz")
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-K8s-SA-Token", "test-sa-token-xyz")
	if !a.VerifySAToken(req) {
		t.Error("VerifySAToken should pass with correct token")
	}
}

func TestAuth_VerifySAToken_Invalid(t *testing.T) {
	dir := t.TempDir()
	tokenPath := dir + "/token"
	os.WriteFile(tokenPath, []byte("correct-token"), 0644)

	a := NewAuth("", tokenPath)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-K8s-SA-Token", "wrong-token")
	if a.VerifySAToken(req) {
		t.Error("VerifySAToken should fail with wrong token")
	}
}

func TestAuth_VerifySAToken_Missing(t *testing.T) {
	dir := t.TempDir()
	tokenPath := dir + "/token"
	os.WriteFile(tokenPath, []byte("correct-token"), 0644)

	a := NewAuth("", tokenPath)

	req := httptest.NewRequest("GET", "/", nil)
	// No X-K8s-SA-Token header
	if a.VerifySAToken(req) {
		t.Error("VerifySAToken should fail with missing token")
	}
}

func TestAuth_RequireRole_SATokenRejection(t *testing.T) {
	dir := t.TempDir()
	tokenPath := dir + "/token"
	os.WriteFile(tokenPath, []byte("correct-token"), 0644)

	a := NewAuth("", tokenPath)
	handler := a.RequireRole("agent-view", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Valid JWT but missing SA token
	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	// No X-K8s-SA-Token
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (SA token missing)", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuth_RequireRole_SATokenValid_NoKey_Rejected(t *testing.T) {
	dir := t.TempDir()
	tokenPath := dir + "/token"
	os.WriteFile(tokenPath, []byte("correct-sa-token"), 0644)

	a := NewAuth("", tokenPath)
	handler := a.RequireRole("agent-view", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// SA token is valid but no public key for JWT — should be rejected
	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Bearer mock-token")
	req.Header.Set("X-K8s-SA-Token", "correct-sa-token")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (no public key configured)", rec.Code, http.StatusUnauthorized)
	}
}

func TestExtractBearerToken_Header(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer my-token-123")
	got := extractBearerToken(req)
	if got != "my-token-123" {
		t.Errorf("got %q, want %q", got, "my-token-123")
	}
}

func TestExtractBearerToken_QueryParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/?token=query-token", nil)
	got := extractBearerToken(req)
	if got != "query-token" {
		t.Errorf("got %q, want %q", got, "query-token")
	}
}

func TestExtractBearerToken_Empty(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	got := extractBearerToken(req)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
