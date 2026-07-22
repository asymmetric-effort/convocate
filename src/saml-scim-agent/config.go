package main

import (
	"fmt"
	"os"
	"strings"
)

// Config holds all configuration for the SAML/SCIM Agent service.
type Config struct {
	ListenAddr     string
	TLSCert        string
	TLSKey         string
	OpenBaoAddr    string
	OpenBaoToken   string
	OpenBaoSkipTLS bool
	EntityID       string
	SSOURL         string
	KeyAlgorithm   string
}

// LoadConfig reads configuration from environment variables.
// Returns an error if any configuration value is invalid.
func LoadConfig() (Config, error) {
	cfg := Config{
		ListenAddr:     envOrDefault("SAML_SCIM_AGENT_LISTEN_ADDR", "0.0.0.0:8443"),
		TLSCert:        os.Getenv("SAML_SCIM_AGENT_TLS_CERT"),
		TLSKey:         os.Getenv("SAML_SCIM_AGENT_TLS_KEY"),
		OpenBaoAddr:    envOrDefault("OPENBAO_ADDR", "https://127.0.0.1:8200"),
		OpenBaoToken:   os.Getenv("OPENBAO_TOKEN"),
		OpenBaoSkipTLS: strings.EqualFold(os.Getenv("OPENBAO_SKIP_VERIFY"), "true"),
		EntityID:       envOrDefault("SAML_SCIM_AGENT_ENTITY_ID", "https://sso.asymmetric-effort.com"),
		SSOURL:         envOrDefault("SAML_SCIM_AGENT_SSO_URL", "https://sso.asymmetric-effort.com/saml/sso"),
	}

	// If token not set directly, try reading from file
	if cfg.OpenBaoToken == "" {
		tokenFile := os.Getenv("OPENBAO_TOKEN_FILE")
		if tokenFile != "" {
			data, err := os.ReadFile(tokenFile)
			if err == nil {
				cfg.OpenBaoToken = strings.TrimSpace(string(data))
			}
		}
	}

	// Validate and normalize key algorithm
	algo := strings.ToLower(envOrDefault("SAML_SCIM_AGENT_KEY_ALGORITHM", "ed25519"))
	switch algo {
	case "ed25519", "rsa":
		cfg.KeyAlgorithm = algo
	default:
		return Config{}, fmt.Errorf("invalid SAML_SCIM_AGENT_KEY_ALGORITHM %q: must be \"ed25519\" or \"rsa\"", algo)
	}

	return cfg, nil
}

func envOrDefault(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}
