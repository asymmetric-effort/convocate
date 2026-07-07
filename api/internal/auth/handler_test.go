package auth

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/asymmetric-effort/convocate/internal/httputil"
)

func setupMockBao(t *testing.T) *httptest.Server {
	t.Helper()
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/auth/userpass/login/validuser" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"auth": map[string]any{
					"client_token":   "tok-valid",
					"entity_id":      "ent-001",
					"policies":       []string{"default", "node-read"},
					"metadata":       map[string]string{"name": "Valid User", "email": "valid@example.com"},
					"lease_duration": 3600,
				},
			})
		case r.URL.Path == "/v1/auth/userpass/login/mfauser" && r.Method == http.MethodPost:
			// MFA-enforced user: returns mfa_requirement instead of a token
			json.NewEncoder(w).Encode(map[string]any{
				"auth": map[string]any{
					"client_token":   "",
					"entity_id":      "",
					"policies":       nil,
					"metadata":       nil,
					"lease_duration": 0,
					"mfa_requirement": map[string]any{
						"mfa_request_id": "mfa-req-001",
					},
				},
			})
		case r.URL.Path == "/v1/sys/mfa/validate" && r.Method == http.MethodPost:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			reqID, _ := body["mfa_request_id"].(string)
			payload, _ := body["mfa_payload"].(map[string]any)
			codes, _ := payload["test-method-id"].([]any)
			if reqID == "mfa-req-001" && len(codes) == 1 && codes[0] == "123456" {
				json.NewEncoder(w).Encode(map[string]any{
					"auth": map[string]any{
						"client_token":   "tok-mfa-valid",
						"entity_id":      "ent-002",
						"policies":       []string{"default", "admin-policy"},
						"metadata":       map[string]string{"name": "MFA User", "email": "mfa@example.com"},
						"lease_duration": 3600,
					},
				})
			} else {
				w.WriteHeader(http.StatusForbidden)
			}
		case r.URL.Path == "/v1/auth/userpass/login/noentityuser" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"auth": map[string]any{
					"client_token":   "tok-noentity",
					"entity_id":      "",
					"policies":       []string{"default"},
					"metadata":       map[string]string{"name": "No Entity"},
					"lease_duration": 0,
				},
			})
		case r.URL.Path == "/v1/auth/userpass/login/baduser" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == "/v1/identity/entity/id/ent-001":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":        "ent-001",
					"name":      "validuser",
					"metadata":  map[string]string{"name": "Valid User", "email": "valid@example.com"},
					"policies":  []string{"default"},
					"group_ids": []string{"grp-1"},
				},
			})
		case r.URL.Path == "/v1/identity/entity/id/ent-002":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":        "ent-002",
					"name":      "mfauser",
					"metadata":  map[string]string{"name": "MFA User", "email": "mfa@example.com"},
					"policies":  []string{"default"},
					"group_ids": []string{"grp-2"},
				},
			})
		case r.URL.Path == "/v1/auth/token/lookup-self":
			token := r.Header.Get("X-Vault-Token")
			if token == "tok-valid" || token == "tok-mfa-valid" {
				json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"entity_id": "ent-001",
						"policies":  []string{"default", "node-read"},
					},
				})
			} else {
				w.WriteHeader(http.StatusForbidden)
			}
		case r.URL.Path == "/v1/auth/token/revoke-self":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	os.Setenv("OPENBAO_ADDR", bao.URL)
	os.Setenv("OPENBAO_MFA_METHOD_ID", "test-method-id")
	t.Cleanup(func() {
		bao.Close()
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_MFA_METHOD_ID")
	})
	return bao
}

func TestHandleLogin_Success(t *testing.T) {
	setupMockBao(t)

	body, _ := json.Marshal(loginRequest{Username: "validuser", Password: "secret"})
	r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleLogin(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var sess session
	json.Unmarshal(w.Body.Bytes(), &sess)
	if sess.AccessToken != "tok-valid" {
		t.Errorf("AccessToken = %q, want %q", sess.AccessToken, "tok-valid")
	}
	if sess.Principal.Username != "validuser" {
		t.Errorf("Username = %q, want %q", sess.Principal.Username, "validuser")
	}
	if sess.Principal.Name != "Valid User" {
		t.Errorf("Name = %q, want %q", sess.Principal.Name, "Valid User")
	}
	if sess.Principal.IDP != "openbao" {
		t.Errorf("IDP = %q, want %q", sess.Principal.IDP, "openbao")
	}
	if sess.ExpiresAt == "" {
		t.Error("ExpiresAt is empty")
	}
}

func TestHandleLogin_BadCredentials(t *testing.T) {
	setupMockBao(t)

	body, _ := json.Marshal(loginRequest{Username: "baduser", Password: "wrong"})
	r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleLogin(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleLogin_EmptyFields(t *testing.T) {
	setupMockBao(t)

	tests := []struct {
		name string
		body loginRequest
	}{
		{"empty username", loginRequest{Username: "", Password: "pass"}},
		{"empty password", loginRequest{Username: "user", Password: ""}},
		{"both empty", loginRequest{Username: "", Password: ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handleLogin(w, r)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestHandleLogin_InvalidJSON(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader([]byte("not json")))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleLogin(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleLogin_NoEntityFallback(t *testing.T) {
	setupMockBao(t)

	body, _ := json.Marshal(loginRequest{Username: "noentityuser", Password: "pass"})
	r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleLogin(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var sess session
	json.Unmarshal(w.Body.Bytes(), &sess)
	if sess.Principal.Username != "noentityuser" {
		t.Errorf("Username = %q, want %q", sess.Principal.Username, "noentityuser")
	}
	if sess.Principal.Name != "No Entity" {
		t.Errorf("Name = %q, want %q", sess.Principal.Name, "No Entity")
	}
}

func TestHandleLogin_MFARequired_NoCode(t *testing.T) {
	setupMockBao(t)

	body, _ := json.Marshal(loginRequest{Username: "mfauser", Password: "secret"})
	r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleLogin(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusUnauthorized, w.Body.String())
	}

	var errResp map[string]string
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp["message"] != "MFA code required" {
		t.Errorf("message = %q, want %q", errResp["message"], "MFA code required")
	}
}

func TestHandleLogin_MFARequired_ValidCode(t *testing.T) {
	setupMockBao(t)

	body, _ := json.Marshal(loginRequest{Username: "mfauser", Password: "secret", MFAToken: "123456"})
	r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleLogin(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var sess session
	json.Unmarshal(w.Body.Bytes(), &sess)
	if sess.AccessToken != "tok-mfa-valid" {
		t.Errorf("AccessToken = %q, want %q", sess.AccessToken, "tok-mfa-valid")
	}
	if sess.Principal.Username != "mfauser" {
		t.Errorf("Username = %q, want %q", sess.Principal.Username, "mfauser")
	}
	if sess.Principal.Name != "MFA User" {
		t.Errorf("Name = %q, want %q", sess.Principal.Name, "MFA User")
	}
}

func TestHandleLogin_MFARequired_InvalidCode(t *testing.T) {
	setupMockBao(t)

	body, _ := json.Marshal(loginRequest{Username: "mfauser", Password: "secret", MFAToken: "000000"})
	r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleLogin(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusUnauthorized, w.Body.String())
	}
}

func TestHandleLogin_MFARequired_NoMethodID(t *testing.T) {
	setupMockBao(t)
	os.Unsetenv("OPENBAO_MFA_METHOD_ID")

	body, _ := json.Marshal(loginRequest{Username: "mfauser", Password: "secret", MFAToken: "123456"})
	r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleLogin(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

func TestHandleMe_WithPrincipal(t *testing.T) {
	principal := &httputil.Principal{
		ID:                "usr-001",
		Username:          "alice",
		Name:              "Alice",
		Email:             "alice@example.com",
		Roles:             []string{"admin"},
		IDP:               "openbao",
		AuthorizedApplets: []string{"nmgr"},
	}

	r := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	handleMe(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var got httputil.Principal
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.ID != "usr-001" {
		t.Errorf("ID = %q, want %q", got.ID, "usr-001")
	}
	if got.Username != "alice" {
		t.Errorf("Username = %q, want %q", got.Username, "alice")
	}
}

func TestHandleMe_NoPrincipal(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	w := httptest.NewRecorder()

	handleMe(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleLogout(t *testing.T) {
	setupMockBao(t)

	r := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	r.Header.Set("Authorization", "Bearer tok-valid")
	w := httptest.NewRecorder()

	handleLogout(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestHandleLogout_MockTokenSkipsRevoke(t *testing.T) {
	// No OpenBao server needed - mock-token should skip revocation
	r := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	r.Header.Set("Authorization", "Bearer mock-token")
	w := httptest.NewRecorder()

	handleLogout(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestHandleLogout_EmptyToken(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	r.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()

	handleLogout(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestHandleRefresh_Success(t *testing.T) {
	setupMockBao(t)

	r := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
	r.Header.Set("Authorization", "Bearer tok-valid")
	w := httptest.NewRecorder()

	handleRefresh(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var sess session
	json.Unmarshal(w.Body.Bytes(), &sess)
	if sess.AccessToken != "tok-valid" {
		t.Errorf("AccessToken = %q, want %q", sess.AccessToken, "tok-valid")
	}
}

func TestHandleRefresh_InvalidToken(t *testing.T) {
	setupMockBao(t)

	r := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
	r.Header.Set("Authorization", "Bearer bad-token")
	w := httptest.NewRecorder()

	handleRefresh(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleOIDCCallback_MissingCode(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/auth/oidc/github/callback", nil)
	w := httptest.NewRecorder()

	handleOIDCCallback(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleOIDCCallback_WithCode(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/auth/oidc/github/callback?code=abc123", nil)
	w := httptest.NewRecorder()

	handleOIDCCallback(w, r)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotImplemented)
	}
}

func TestHandleOIDCStart(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/auth/oidc/github/start", nil)
	w := httptest.NewRecorder()

	handleOIDCStart(w, r)

	if w.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusFound)
	}
}

func TestHandleRefresh_EntityLookupFails(t *testing.T) {
	// Token lookup succeeds but entity lookup fails, resulting in nil principal
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/token/lookup-self":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"entity_id": "ent-fail",
					"policies":  []string{"default"},
				},
			})
		case "/v1/identity/entity/id/ent-fail":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	r := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
	r.Header.Set("Authorization", "Bearer some-token")
	w := httptest.NewRecorder()

	handleRefresh(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleRefresh_NoEntityID(t *testing.T) {
	// Token lookup succeeds but has no entity_id
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"entity_id": "",
				"policies":  []string{"default"},
			},
		})
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	r := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
	r.Header.Set("Authorization", "Bearer some-token")
	w := httptest.NewRecorder()

	handleRefresh(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestInitJWT_WithValidPEM(t *testing.T) {
	// Generate a valid EC private key in PEM format
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	derBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: derBytes})

	os.Setenv("JWT_EC_PRIVATE_KEY", string(pemBlock))
	defer os.Unsetenv("JWT_EC_PRIVATE_KEY")

	InitJWT()

	if signingKey == nil {
		t.Fatal("signingKey is nil after InitJWT with valid PEM")
	}
	if verifyKey == nil {
		t.Fatal("verifyKey is nil after InitJWT with valid PEM")
	}
}

func TestInitJWT_WithInvalidPEM(t *testing.T) {
	os.Setenv("JWT_EC_PRIVATE_KEY", "not-a-valid-pem")
	defer os.Unsetenv("JWT_EC_PRIVATE_KEY")

	// Should not panic, should fall back to ephemeral key
	InitJWT()

	if signingKey == nil {
		t.Fatal("signingKey is nil after InitJWT fallback")
	}
}

func TestInitJWT_WithBadASN1PEM(t *testing.T) {
	// Valid PEM block but invalid ASN1 content
	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte("not-asn1")})
	os.Setenv("JWT_EC_PRIVATE_KEY", string(pemBlock))
	defer os.Unsetenv("JWT_EC_PRIVATE_KEY")

	InitJWT()

	if signingKey == nil {
		t.Fatal("signingKey is nil after InitJWT with bad ASN1")
	}
}

func TestParseECPrivateKey_NoPEMBlock(t *testing.T) {
	_, err := parseECPrivateKey([]byte("not pem data"))
	if err == nil {
		t.Fatal("expected error for non-PEM data")
	}
}

func TestParseECPrivateKey_BadASN1(t *testing.T) {
	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte("garbage")})
	_, err := parseECPrivateKey(pemBlock)
	if err == nil {
		t.Fatal("expected error for bad ASN1")
	}
}

func TestParseECPrivateKey_Valid(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	derBytes, _ := x509.MarshalECPrivateKey(key)
	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: derBytes})

	parsed, err := parseECPrivateKey(pemBlock)
	if err != nil {
		t.Fatalf("parseECPrivateKey failed: %v", err)
	}
	if parsed.D == nil {
		t.Fatal("parsed key has nil D")
	}
}

func TestVerifyJWT_InvalidFormat_NoDots(t *testing.T) {
	InitJWT()
	_, err := VerifyJWT("nodots")
	if err == nil {
		t.Fatal("expected error for token without dots")
	}
}

func TestVerifyJWT_InvalidFormat_OneDot(t *testing.T) {
	InitJWT()
	_, err := VerifyJWT("one.dot")
	if err == nil {
		t.Fatal("expected error for token with only one dot")
	}
}

func TestVerifyJWT_BadSignatureEncoding(t *testing.T) {
	InitJWT()
	_, err := VerifyJWT("header.claims.!!!bad-base64!!!")
	if err == nil {
		t.Fatal("expected error for bad signature encoding")
	}
}

func TestVerifyJWT_WrongSignatureLength(t *testing.T) {
	InitJWT()
	// Valid base64 but wrong length (not 64 bytes)
	shortSig := "AQID" // 3 bytes
	_, err := VerifyJWT("header.claims." + shortSig)
	if err == nil {
		t.Fatal("expected error for wrong signature length")
	}
}

func TestRegister(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	// Verify login route is registered by making a request
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader([]byte("{}")))
	mux.ServeHTTP(w, r)

	// Should get a 401 for empty credentials, not a 404
	if w.Code == http.StatusNotFound {
		t.Error("login route not registered")
	}
}
