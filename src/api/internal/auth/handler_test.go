package auth

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/asymmetric-effort/convocate/internal/httputil"
)

// initJWTWithKey generates a key, sets the env var, and calls InitJWT.
func initJWTWithKey(t *testing.T) {
	t.Helper()
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
	t.Cleanup(func() { os.Unsetenv("JWT_EC_PRIVATE_KEY") })
	InitJWT()
}

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

func TestHandleLogout_AnyTokenAttemptsRevoke(t *testing.T) {
	// All tokens now attempt revocation (best-effort). Logout responds 204
	// regardless of revocation outcome.
	r := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	r.Header.Set("Authorization", "Bearer some-token")
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
	origSigning, origVerify := signingKey, verifyKey
	defer func() { signingKey, verifyKey = origSigning, origVerify }()

	signingKey, verifyKey = nil, nil
	os.Setenv("JWT_EC_PRIVATE_KEY", "not-a-valid-pem")
	defer os.Unsetenv("JWT_EC_PRIVATE_KEY")

	// Should not panic; keys should remain nil (no ephemeral fallback)
	InitJWT()

	if signingKey != nil {
		t.Fatal("signingKey should be nil after InitJWT with invalid PEM")
	}
}

func TestInitJWT_WithBadASN1PEM(t *testing.T) {
	origSigning, origVerify := signingKey, verifyKey
	defer func() { signingKey, verifyKey = origSigning, origVerify }()

	signingKey, verifyKey = nil, nil
	// Valid PEM block but invalid ASN1 content
	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte("not-asn1")})
	os.Setenv("JWT_EC_PRIVATE_KEY", string(pemBlock))
	defer os.Unsetenv("JWT_EC_PRIVATE_KEY")

	InitJWT()

	if signingKey != nil {
		t.Fatal("signingKey should be nil after InitJWT with bad ASN1")
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
	initJWTWithKey(t)
	_, err := VerifyJWT("nodots")
	if err == nil {
		t.Fatal("expected error for token without dots")
	}
}

func TestVerifyJWT_InvalidFormat_OneDot(t *testing.T) {
	initJWTWithKey(t)
	_, err := VerifyJWT("one.dot")
	if err == nil {
		t.Fatal("expected error for token with only one dot")
	}
}

func TestVerifyJWT_BadSignatureEncoding(t *testing.T) {
	initJWTWithKey(t)
	_, err := VerifyJWT("header.claims.!!!bad-base64!!!")
	if err == nil {
		t.Fatal("expected error for bad signature encoding")
	}
}

func TestVerifyJWT_WrongSignatureLength(t *testing.T) {
	initJWTWithKey(t)
	// Valid base64 but wrong length (not 64 bytes)
	shortSig := "AQID" // 3 bytes
	_, err := VerifyJWT("header.claims." + shortSig)
	if err == nil {
		t.Fatal("expected error for wrong signature length")
	}
}

func TestOpenbaoLogin_UnexpectedStatus(t *testing.T) {
	// Server returns 503 (not 400/401/403) → triggers "unexpected status" path
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service unavailable"))
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoLogin("testuser", "testpass")
	if err == nil {
		t.Fatal("expected error for unexpected status")
	}
}

func TestOpenbaoMFAValidate_UnexpectedStatus(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service unavailable"))
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	_, err := openbaoMFAValidate("req-1", "method-1", "123456")
	if err == nil {
		t.Fatal("expected error for unexpected status")
	}
}

func TestOpenbaoRevokeSelf_ErrorStatus(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bao.Close()
	os.Setenv("OPENBAO_ADDR", bao.URL)
	defer os.Unsetenv("OPENBAO_ADDR")

	err := openbaoRevokeSelf("tok")
	if err == nil {
		t.Fatal("expected error for error status")
	}
}

func TestRolesToApplets_VariousPolicies(t *testing.T) {
	tests := []struct {
		policies []string
		contains string
	}{
		{[]string{"agent-manage"}, "amgr"},
		{[]string{"pb-read"}, "pb"},
		{[]string{"ide-write"}, "ide"},
		{[]string{"access-admin"}, "ac"},
		{[]string{"repo-push"}, "repo"},
		{[]string{"support-ticket"}, "sup"},
	}
	for _, tt := range tests {
		applets := rolesToApplets(tt.policies)
		found := false
		for _, a := range applets {
			if a == tt.contains {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("policies %v should include applet %q, got %v", tt.policies, tt.contains, applets)
		}
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

func TestHandleTOTPEnroll_Success(t *testing.T) {
	methodID := "test-method-id"

	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/identity/mfa/method/totp/admin-generate" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"url":     "otpauth://totp/convocate:testuser?secret=JBSWY3DPEHPK3PXP",
					"barcode": "base64encodedqr",
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer bao.Close()

	os.Setenv("OPENBAO_ADDR", bao.URL)
	os.Setenv("OPENBAO_MFA_METHOD_ID", methodID)
	os.Setenv("OPENBAO_TOKEN", "test-service-token")
	t.Cleanup(func() {
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_MFA_METHOD_ID")
		os.Unsetenv("OPENBAO_TOKEN")
	})

	principal := &httputil.Principal{
		ID:       "ent-001",
		Username: "testuser",
	}
	r := httptest.NewRequest("POST", "/api/v1/auth/totp/enroll", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	handleTOTPEnroll(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var result map[string]string
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["url"] == "" {
		t.Error("expected url in response")
	}
	if result["barcode"] == "" {
		t.Error("expected barcode in response")
	}
}

func TestHandleTOTPEnroll_NoMethodID(t *testing.T) {
	os.Unsetenv("OPENBAO_MFA_METHOD_ID")

	principal := &httputil.Principal{ID: "ent-001", Username: "testuser"}
	r := httptest.NewRequest("POST", "/api/v1/auth/totp/enroll", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	handleTOTPEnroll(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleTOTPEnroll_NoPrincipal(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/v1/auth/totp/enroll", nil)
	w := httptest.NewRecorder()

	handleTOTPEnroll(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleTOTPEnroll_EnrollmentFails(t *testing.T) {
	methodID := "test-method-id"

	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("enrollment error"))
	}))
	defer bao.Close()

	os.Setenv("OPENBAO_ADDR", bao.URL)
	os.Setenv("OPENBAO_MFA_METHOD_ID", methodID)
	os.Setenv("OPENBAO_TOKEN", "test-service-token")
	t.Cleanup(func() {
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_MFA_METHOD_ID")
		os.Unsetenv("OPENBAO_TOKEN")
	})

	principal := &httputil.Principal{ID: "ent-001", Username: "testuser"}
	r := httptest.NewRequest("POST", "/api/v1/auth/totp/enroll", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	handleTOTPEnroll(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandleTOTPStatus_Enrolled(t *testing.T) {
	methodID := "test-method-id"
	entityID := "ent-001"

	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/identity/entity/id/"+entityID {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":          entityID,
					"name":        "testuser",
					"mfa_secrets": map[string]any{methodID: map[string]any{"type": "totp"}},
				},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer bao.Close()

	os.Setenv("OPENBAO_ADDR", bao.URL)
	os.Setenv("OPENBAO_MFA_METHOD_ID", methodID)
	os.Setenv("OPENBAO_TOKEN", "test-service-token")
	t.Cleanup(func() {
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_MFA_METHOD_ID")
		os.Unsetenv("OPENBAO_TOKEN")
	})

	principal := &httputil.Principal{ID: entityID, Username: "testuser"}
	r := httptest.NewRequest("GET", "/api/v1/auth/totp/status", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	handleTOTPStatus(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var result map[string]bool
	json.Unmarshal(w.Body.Bytes(), &result)
	if !result["enrolled"] {
		t.Error("expected enrolled = true")
	}
}

func TestHandleTOTPStatus_NotEnrolled(t *testing.T) {
	methodID := "test-method-id"
	entityID := "ent-001"

	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/identity/entity/id/"+entityID {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":          entityID,
					"name":        "testuser",
					"mfa_secrets": map[string]any{},
				},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer bao.Close()

	os.Setenv("OPENBAO_ADDR", bao.URL)
	os.Setenv("OPENBAO_MFA_METHOD_ID", methodID)
	os.Setenv("OPENBAO_TOKEN", "test-service-token")
	t.Cleanup(func() {
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_MFA_METHOD_ID")
		os.Unsetenv("OPENBAO_TOKEN")
	})

	principal := &httputil.Principal{ID: entityID, Username: "testuser"}
	r := httptest.NewRequest("GET", "/api/v1/auth/totp/status", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	handleTOTPStatus(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var result map[string]bool
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["enrolled"] {
		t.Error("expected enrolled = false")
	}
}

func TestHandleTOTPStatus_NoMethodID(t *testing.T) {
	os.Unsetenv("OPENBAO_MFA_METHOD_ID")

	principal := &httputil.Principal{ID: "ent-001", Username: "testuser"}
	r := httptest.NewRequest("GET", "/api/v1/auth/totp/status", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	handleTOTPStatus(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var result map[string]bool
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["enrolled"] {
		t.Error("expected enrolled = false when method ID not set")
	}
}

func TestHandleTOTPStatus_NoPrincipal(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/auth/totp/status", nil)
	w := httptest.NewRecorder()

	handleTOTPStatus(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleTOTPStatus_LookupFails(t *testing.T) {
	methodID := "test-method-id"

	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bao.Close()

	os.Setenv("OPENBAO_ADDR", bao.URL)
	os.Setenv("OPENBAO_MFA_METHOD_ID", methodID)
	os.Setenv("OPENBAO_TOKEN", "test-service-token")
	t.Cleanup(func() {
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_MFA_METHOD_ID")
		os.Unsetenv("OPENBAO_TOKEN")
	})

	principal := &httputil.Principal{ID: "ent-001", Username: "testuser"}
	r := httptest.NewRequest("GET", "/api/v1/auth/totp/status", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	handleTOTPStatus(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandleTOTPDestroy_Success(t *testing.T) {
	methodID := "test-method-id"

	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/identity/mfa/method/totp/admin-destroy" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer bao.Close()

	os.Setenv("OPENBAO_ADDR", bao.URL)
	os.Setenv("OPENBAO_MFA_METHOD_ID", methodID)
	os.Setenv("OPENBAO_TOKEN", "test-service-token")
	t.Cleanup(func() {
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_MFA_METHOD_ID")
		os.Unsetenv("OPENBAO_TOKEN")
	})

	principal := &httputil.Principal{ID: "ent-001", Username: "testuser"}
	r := httptest.NewRequest("DELETE", "/api/v1/auth/totp", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	handleTOTPDestroy(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusNoContent, w.Body.String())
	}
}

func TestHandleTOTPDestroy_NoMethodID(t *testing.T) {
	os.Unsetenv("OPENBAO_MFA_METHOD_ID")

	principal := &httputil.Principal{ID: "ent-001", Username: "testuser"}
	r := httptest.NewRequest("DELETE", "/api/v1/auth/totp", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	handleTOTPDestroy(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleTOTPDestroy_NoPrincipal(t *testing.T) {
	r := httptest.NewRequest("DELETE", "/api/v1/auth/totp", nil)
	w := httptest.NewRecorder()

	handleTOTPDestroy(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleTOTPDestroy_DestroyFails(t *testing.T) {
	methodID := "test-method-id"

	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("destroy error"))
	}))
	defer bao.Close()

	os.Setenv("OPENBAO_ADDR", bao.URL)
	os.Setenv("OPENBAO_MFA_METHOD_ID", methodID)
	os.Setenv("OPENBAO_TOKEN", "test-service-token")
	t.Cleanup(func() {
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_MFA_METHOD_ID")
		os.Unsetenv("OPENBAO_TOKEN")
	})

	principal := &httputil.Principal{ID: "ent-001", Username: "testuser"}
	r := httptest.NewRequest("DELETE", "/api/v1/auth/totp", nil)
	ctx := httputil.ContextWithPrincipal(r.Context(), principal)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	handleTOTPDestroy(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandleLogin_SAML_MFARequired(t *testing.T) {
	entityID := "samlmfauser"
	methodID := "test-method-id"

	// Mock SAML agent
	samlXML := buildTestSAMLResponseXML(entityID, "mfa@example.com", []string{"node-ops"})
	samlB64 := base64.StdEncoding.EncodeToString([]byte(samlXML))
	samlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := fmt.Sprintf(`<form><input type="hidden" name="SAMLResponse" value="%s"/></form>`, samlB64)
		w.Write([]byte(html))
	}))
	defer samlServer.Close()

	// Mock OpenBao — entity is enrolled
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/identity/entity/id/"+entityID:
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":          entityID,
					"name":        entityID,
					"mfa_secrets": map[string]any{methodID: map[string]any{}},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer bao.Close()

	os.Setenv("SAML_SCIM_AGENT_URL", samlServer.URL)
	os.Setenv("OPENBAO_ADDR", bao.URL)
	os.Setenv("OPENBAO_MFA_METHOD_ID", methodID)
	os.Setenv("OPENBAO_TOKEN", "test-service-token")
	t.Cleanup(func() {
		os.Unsetenv("SAML_SCIM_AGENT_URL")
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_MFA_METHOD_ID")
		os.Unsetenv("OPENBAO_TOKEN")
	})

	body, _ := json.Marshal(loginRequest{Username: entityID, Password: "secret"})
	r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleLogin(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusUnauthorized, w.Body.String())
	}

	var errResp map[string]string
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp["code"] != "mfa_required" {
		t.Errorf("error code = %q, want %q", errResp["code"], "mfa_required")
	}
}

func TestHandleLogin_SAML_Success(t *testing.T) {
	initJWTWithKey(t)

	samlXML := buildTestSAMLResponseXML("samluser", "saml@example.com", []string{"node-ops"})
	samlB64 := base64.StdEncoding.EncodeToString([]byte(samlXML))
	samlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := fmt.Sprintf(`<form><input type="hidden" name="SAMLResponse" value="%s"/></form>`, samlB64)
		w.Write([]byte(html))
	}))
	defer samlServer.Close()

	os.Setenv("SAML_SCIM_AGENT_URL", samlServer.URL)
	os.Unsetenv("OPENBAO_MFA_METHOD_ID")
	t.Cleanup(func() { os.Unsetenv("SAML_SCIM_AGENT_URL") })

	body, _ := json.Marshal(loginRequest{Username: "samluser", Password: "secret"})
	r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleLogin(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var sess session
	json.Unmarshal(w.Body.Bytes(), &sess)
	if sess.AccessToken == "" {
		t.Error("AccessToken is empty")
	}
	if sess.Principal.Username != "samluser" {
		t.Errorf("Username = %q, want %q", sess.Principal.Username, "samluser")
	}
	if sess.Principal.IDP != "saml" {
		t.Errorf("IDP = %q, want %q", sess.Principal.IDP, "saml")
	}
}

func TestHandleLogin_SAML_AuthFailed(t *testing.T) {
	samlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<form><input type="text" name="username"/></form>`
		w.Write([]byte(html))
	}))
	defer samlServer.Close()

	os.Setenv("SAML_SCIM_AGENT_URL", samlServer.URL)
	os.Unsetenv("OPENBAO_MFA_METHOD_ID")
	t.Cleanup(func() { os.Unsetenv("SAML_SCIM_AGENT_URL") })

	body, _ := json.Marshal(loginRequest{Username: "baduser", Password: "wrong"})
	r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleLogin(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRegister_TOTPRoutes(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	// Verify TOTP routes are registered (should not return 404)
	routes := []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/auth/totp/enroll"},
		{"GET", "/api/v1/auth/totp/status"},
		{"DELETE", "/api/v1/auth/totp"},
	}

	for _, route := range routes {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(route.method, route.path, nil)
		mux.ServeHTTP(w, r)

		// Auth middleware should reject with 401, not 404
		if w.Code == http.StatusNotFound {
			t.Errorf("%s %s route not registered (got 404)", route.method, route.path)
		}
	}
}
