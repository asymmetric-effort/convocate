package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	// Clear all env vars that LoadConfig reads
	envVars := []string{
		"SAML_SCIM_AGENT_LISTEN_ADDR",
		"SAML_SCIM_AGENT_TLS_CERT",
		"SAML_SCIM_AGENT_TLS_KEY",
		"OPENBAO_ADDR",
		"OPENBAO_TOKEN",
		"OPENBAO_SKIP_VERIFY",
		"SAML_SCIM_AGENT_ENTITY_ID",
		"SAML_SCIM_AGENT_SSO_URL",
		"OPENBAO_TOKEN_FILE",
		"SAML_SCIM_AGENT_KEY_ALGORITHM",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddr != "0.0.0.0:8443" {
		t.Errorf("expected default listen addr 0.0.0.0:8443, got %s", cfg.ListenAddr)
	}
	if cfg.TLSCert != "" {
		t.Errorf("expected empty TLSCert, got %s", cfg.TLSCert)
	}
	if cfg.TLSKey != "" {
		t.Errorf("expected empty TLSKey, got %s", cfg.TLSKey)
	}
	if cfg.OpenBaoAddr != "https://127.0.0.1:8200" {
		t.Errorf("expected default OpenBaoAddr, got %s", cfg.OpenBaoAddr)
	}
	if cfg.OpenBaoToken != "" {
		t.Errorf("expected empty OpenBaoToken, got %s", cfg.OpenBaoToken)
	}
	if cfg.OpenBaoSkipTLS != false {
		t.Errorf("expected OpenBaoSkipTLS false, got %v", cfg.OpenBaoSkipTLS)
	}
	if cfg.EntityID != "https://sso.asymmetric-effort.com" {
		t.Errorf("expected default EntityID, got %s", cfg.EntityID)
	}
	if cfg.SSOURL != "https://sso.asymmetric-effort.com/saml/sso" {
		t.Errorf("expected default SSOURL, got %s", cfg.SSOURL)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	os.Setenv("SAML_SCIM_AGENT_LISTEN_ADDR", "localhost:9999")
	os.Setenv("SAML_SCIM_AGENT_TLS_CERT", "/path/to/cert.pem")
	os.Setenv("SAML_SCIM_AGENT_TLS_KEY", "/path/to/key.pem")
	os.Setenv("OPENBAO_ADDR", "https://vault.example.com:8200")
	os.Setenv("OPENBAO_TOKEN", "s.mytoken123")
	os.Setenv("OPENBAO_SKIP_VERIFY", "true")
	os.Setenv("SAML_SCIM_AGENT_ENTITY_ID", "https://custom.entity.id")
	os.Setenv("SAML_SCIM_AGENT_SSO_URL", "https://custom.sso.url/saml/sso")
	defer func() {
		os.Unsetenv("SAML_SCIM_AGENT_LISTEN_ADDR")
		os.Unsetenv("SAML_SCIM_AGENT_TLS_CERT")
		os.Unsetenv("SAML_SCIM_AGENT_TLS_KEY")
		os.Unsetenv("OPENBAO_ADDR")
		os.Unsetenv("OPENBAO_TOKEN")
		os.Unsetenv("OPENBAO_SKIP_VERIFY")
		os.Unsetenv("SAML_SCIM_AGENT_ENTITY_ID")
		os.Unsetenv("SAML_SCIM_AGENT_SSO_URL")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddr != "localhost:9999" {
		t.Errorf("expected localhost:9999, got %s", cfg.ListenAddr)
	}
	if cfg.TLSCert != "/path/to/cert.pem" {
		t.Errorf("expected /path/to/cert.pem, got %s", cfg.TLSCert)
	}
	if cfg.TLSKey != "/path/to/key.pem" {
		t.Errorf("expected /path/to/key.pem, got %s", cfg.TLSKey)
	}
	if cfg.OpenBaoAddr != "https://vault.example.com:8200" {
		t.Errorf("expected https://vault.example.com:8200, got %s", cfg.OpenBaoAddr)
	}
	if cfg.OpenBaoToken != "s.mytoken123" {
		t.Errorf("expected s.mytoken123, got %s", cfg.OpenBaoToken)
	}
	if cfg.OpenBaoSkipTLS != true {
		t.Errorf("expected OpenBaoSkipTLS true, got %v", cfg.OpenBaoSkipTLS)
	}
	if cfg.EntityID != "https://custom.entity.id" {
		t.Errorf("expected https://custom.entity.id, got %s", cfg.EntityID)
	}
	if cfg.SSOURL != "https://custom.sso.url/saml/sso" {
		t.Errorf("expected https://custom.sso.url/saml/sso, got %s", cfg.SSOURL)
	}
}

func TestLoadConfigTokenFromFile(t *testing.T) {
	// Create a temp token file
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token")
	if err := os.WriteFile(tokenFile, []byte("  s.file-token-123  \n"), 0600); err != nil {
		t.Fatalf("failed to write token file: %v", err)
	}

	os.Unsetenv("OPENBAO_TOKEN")
	os.Setenv("OPENBAO_TOKEN_FILE", tokenFile)
	defer os.Unsetenv("OPENBAO_TOKEN_FILE")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.OpenBaoToken != "s.file-token-123" {
		t.Errorf("expected 's.file-token-123', got '%s'", cfg.OpenBaoToken)
	}
}

func TestLoadConfigTokenDirectOverridesFile(t *testing.T) {
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token")
	os.WriteFile(tokenFile, []byte("s.file-token"), 0600)

	os.Setenv("OPENBAO_TOKEN", "s.direct-token")
	os.Setenv("OPENBAO_TOKEN_FILE", tokenFile)
	defer func() {
		os.Unsetenv("OPENBAO_TOKEN")
		os.Unsetenv("OPENBAO_TOKEN_FILE")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.OpenBaoToken != "s.direct-token" {
		t.Errorf("expected direct token to take precedence, got %s", cfg.OpenBaoToken)
	}
}

func TestLoadConfigTokenFileNotExist(t *testing.T) {
	os.Unsetenv("OPENBAO_TOKEN")
	os.Setenv("OPENBAO_TOKEN_FILE", "/nonexistent/path/token")
	defer os.Unsetenv("OPENBAO_TOKEN_FILE")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.OpenBaoToken != "" {
		t.Errorf("expected empty token when file doesn't exist, got %s", cfg.OpenBaoToken)
	}
}

func TestLoadConfigSkipTLSCaseInsensitive(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"TrUe", true},
		{"false", false},
		{"", false},
		{"anything", false},
	}

	for _, tc := range tests {
		os.Setenv("OPENBAO_SKIP_VERIFY", tc.value)
		cfg, _ := LoadConfig()
		if cfg.OpenBaoSkipTLS != tc.expected {
			t.Errorf("OPENBAO_SKIP_VERIFY=%q: expected %v, got %v", tc.value, tc.expected, cfg.OpenBaoSkipTLS)
		}
	}
	os.Unsetenv("OPENBAO_SKIP_VERIFY")
}

func TestEnvOrDefault(t *testing.T) {
	os.Unsetenv("TEST_ENV_VAR_XYZ")
	result := envOrDefault("TEST_ENV_VAR_XYZ", "default_val")
	if result != "default_val" {
		t.Errorf("expected default_val, got %s", result)
	}

	os.Setenv("TEST_ENV_VAR_XYZ", "custom_val")
	defer os.Unsetenv("TEST_ENV_VAR_XYZ")
	result = envOrDefault("TEST_ENV_VAR_XYZ", "default_val")
	if result != "custom_val" {
		t.Errorf("expected custom_val, got %s", result)
	}
}

func TestLoadConfigTokenFileEmpty(t *testing.T) {
	os.Unsetenv("OPENBAO_TOKEN")
	os.Unsetenv("OPENBAO_TOKEN_FILE")

	cfg, _ := LoadConfig()
	if cfg.OpenBaoToken != "" {
		t.Errorf("expected empty token, got %s", cfg.OpenBaoToken)
	}
}

func TestLoadConfigTokenFileEmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token")
	os.WriteFile(tokenFile, []byte("   \n  "), 0600)

	os.Unsetenv("OPENBAO_TOKEN")
	os.Setenv("OPENBAO_TOKEN_FILE", tokenFile)
	defer os.Unsetenv("OPENBAO_TOKEN_FILE")

	cfg, _ := LoadConfig()
	if cfg.OpenBaoToken != "" {
		t.Errorf("expected empty token from whitespace-only file, got '%s'", cfg.OpenBaoToken)
	}
}

func TestKeyAlgorithm_Default(t *testing.T) {
	os.Unsetenv("SAML_SCIM_AGENT_KEY_ALGORITHM")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.KeyAlgorithm != "ed25519" {
		t.Errorf("expected default KeyAlgorithm ed25519, got %s", cfg.KeyAlgorithm)
	}
}

func TestKeyAlgorithm_RSA(t *testing.T) {
	os.Setenv("SAML_SCIM_AGENT_KEY_ALGORITHM", "rsa")
	defer os.Unsetenv("SAML_SCIM_AGENT_KEY_ALGORITHM")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.KeyAlgorithm != "rsa" {
		t.Errorf("expected KeyAlgorithm rsa, got %s", cfg.KeyAlgorithm)
	}
}

func TestKeyAlgorithm_Ed25519(t *testing.T) {
	os.Setenv("SAML_SCIM_AGENT_KEY_ALGORITHM", "ed25519")
	defer os.Unsetenv("SAML_SCIM_AGENT_KEY_ALGORITHM")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.KeyAlgorithm != "ed25519" {
		t.Errorf("expected KeyAlgorithm ed25519, got %s", cfg.KeyAlgorithm)
	}
}

func TestKeyAlgorithm_CaseInsensitive(t *testing.T) {
	os.Setenv("SAML_SCIM_AGENT_KEY_ALGORITHM", "Ed25519")
	defer os.Unsetenv("SAML_SCIM_AGENT_KEY_ALGORITHM")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.KeyAlgorithm != "ed25519" {
		t.Errorf("expected KeyAlgorithm ed25519 (lowercased), got %s", cfg.KeyAlgorithm)
	}
}

func TestKeyAlgorithm_Invalid(t *testing.T) {
	os.Setenv("SAML_SCIM_AGENT_KEY_ALGORITHM", "dsa")
	defer os.Unsetenv("SAML_SCIM_AGENT_KEY_ALGORITHM")
	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid key algorithm")
	}
	if !strings.Contains(err.Error(), "invalid SAML_SCIM_AGENT_KEY_ALGORITHM") {
		t.Errorf("expected error to contain 'invalid SAML_SCIM_AGENT_KEY_ALGORITHM', got: %v", err)
	}
}
