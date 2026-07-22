package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/asymmetric-effort/convocate/src/saml-scim-agent/openbao"
	"github.com/asymmetric-effort/convocate/src/saml-scim-agent/saml"
)

func generateTestKeyPair(t *testing.T) *saml.KeyPair {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	return &saml.KeyPair{
		PrivateKey:  key,
		Certificate: cert,
		CertPEM:     certDER,
	}
}

func TestRunNoToken(t *testing.T) {
	cfg := Config{
		ListenAddr: "localhost:8443",
		OpenBaoAddr: "https://127.0.0.1:8200",
		OpenBaoToken: "",
	}
	_, _, err := run(cfg)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestRunKeysError(t *testing.T) {
	cfg := Config{
		ListenAddr:   "localhost:8443",
		OpenBaoAddr:  "http://127.0.0.1:1", // unreachable
		OpenBaoToken: "test-token",
	}
	_, _, err := run(cfg)
	if err == nil {
		t.Fatal("expected error for unreachable OpenBao")
	}
}

func TestRunSuccess(t *testing.T) {
	// Mock OpenBao that returns 404 for key read (triggers generation) and accepts write
	baoMux := http.NewServeMux()
	baoMux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
		}
	})
	bao := httptest.NewServer(baoMux)
	defer bao.Close()

	cfg := Config{
		ListenAddr:   "localhost:8443",
		OpenBaoAddr:  bao.URL,
		OpenBaoToken: "test-token",
		EntityID:     "https://sso.example.com",
		SSOURL:       "https://sso.example.com/saml/sso",
	}

	mux, client, err := run(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mux == nil {
		t.Fatal("expected non-nil mux")
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestStartNoToken(t *testing.T) {
	cfg := Config{
		OpenBaoToken: "",
	}
	err := start(cfg)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestStartInvalidBaoAddr(t *testing.T) {
	cfg := Config{
		OpenBaoAddr:  "http://127.0.0.1:1",
		OpenBaoToken: "test-token",
		ListenAddr:   "localhost:0",
	}
	err := start(cfg)
	if err == nil {
		t.Fatal("expected error for unreachable OpenBao")
	}
}

func TestStartListenError(t *testing.T) {
	// Mock OpenBao for key generation
	baoMux := http.NewServeMux()
	baoMux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
		}
	})
	bao := httptest.NewServer(baoMux)
	defer bao.Close()

	cfg := Config{
		OpenBaoAddr:  bao.URL,
		OpenBaoToken: "test-token",
		ListenAddr:   "invalid:-1",
		EntityID:     "https://sso.example.com",
		SSOURL:       "https://sso.example.com/saml/sso",
	}
	err := start(cfg)
	if err == nil {
		t.Fatal("expected error for invalid listen address")
	}
}

func TestServeInvalidAddr(t *testing.T) {
	mux := http.NewServeMux()
	cfg := Config{
		ListenAddr: "invalid-addr-that-will-fail:-1",
	}
	err := serve(cfg, mux)
	if err == nil {
		t.Fatal("expected error for invalid listen address")
	}
}

func TestServeTLSInvalidAddr(t *testing.T) {
	mux := http.NewServeMux()
	cfg := Config{
		ListenAddr: "invalid-addr:-1",
		TLSCert:    "/nonexistent/cert.pem",
		TLSKey:     "/nonexistent/key.pem",
	}
	err := serve(cfg, mux)
	if err == nil {
		t.Fatal("expected error for invalid TLS config")
	}
}

func TestBuildMux(t *testing.T) {
	bao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"initialized":true,"sealed":false}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer bao.Close()

	client := openbao.NewClient(bao.URL, "test-token", true)
	keys := generateTestKeyPair(t)
	cfg := Config{
		ListenAddr: "localhost:8443",
		EntityID:   "https://sso.example.com",
		SSOURL:     "https://sso.example.com/saml/sso",
	}

	mux := buildMux(client, keys, cfg)
	if mux == nil {
		t.Fatal("expected non-nil mux")
	}

	// Test health endpoint through the mux
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from /health, got %d", w.Code)
	}

	// Test metadata endpoint
	req = httptest.NewRequest(http.MethodGet, "/saml/metadata", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from /saml/metadata, got %d", w.Code)
	}

	// Test SLO endpoint
	req = httptest.NewRequest(http.MethodGet, "/saml/slo", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from /saml/slo, got %d", w.Code)
	}

	// Test SCIM endpoint (no auth should get 401)
	req = httptest.NewRequest(http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 from /scim/v2/ServiceProviderConfig without auth, got %d", w.Code)
	}

	// Test SCIM with auth
	req = httptest.NewRequest(http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from /scim/v2/ServiceProviderConfig with auth, got %d", w.Code)
	}

	// Test SSO without SAMLRequest
	req = httptest.NewRequest(http.MethodGet, "/saml/sso", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 from /saml/sso without SAMLRequest, got %d", w.Code)
	}

	// Test login without POST
	req = httptest.NewRequest(http.MethodGet, "/saml/login", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 from GET /saml/login, got %d", w.Code)
	}
}
