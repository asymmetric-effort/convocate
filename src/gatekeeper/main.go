package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/asymmetric-effort/convocate/src/gatekeeper/openbao"
	"github.com/asymmetric-effort/convocate/src/gatekeeper/saml"
	"github.com/asymmetric-effort/convocate/src/gatekeeper/scim"
)

// buildMux creates the HTTP mux with all routes configured.
func buildMux(client *openbao.Client, keys *saml.KeyPair, cfg Config) *http.ServeMux {
	samlHandler := &saml.Handler{
		Client:   client,
		Keys:     keys,
		EntityID: cfg.EntityID,
		SSOURL:   cfg.SSOURL,
	}

	scimHandler := &scim.Handler{
		Client:  client,
		BaseURL: "https://" + cfg.ListenAddr,
	}

	healthHandler := &HealthHandler{Client: client}

	mux := http.NewServeMux()

	// Health
	mux.Handle("/health", healthHandler)

	// SAML endpoints
	mux.HandleFunc("/saml/metadata", samlHandler.ServeMetadata)
	mux.HandleFunc("/saml/sso", samlHandler.ServeSSO)
	mux.HandleFunc("/saml/login", samlHandler.ServeLogin)
	mux.HandleFunc("/saml/slo", samlHandler.ServeSLO)

	// SCIM endpoints
	mux.Handle("/scim/v2/", scimHandler)

	return mux
}

// run initializes and starts the gatekeeper server. Returns an error if setup fails.
func run(cfg Config) (*http.ServeMux, *openbao.Client, error) {
	if cfg.OpenBaoToken == "" {
		return nil, nil, fmt.Errorf("OPENBAO_TOKEN or OPENBAO_TOKEN_FILE must be set")
	}

	client := openbao.NewClient(cfg.OpenBaoAddr, cfg.OpenBaoToken, cfg.OpenBaoSkipTLS)

	keys, err := saml.LoadOrGenerateKeys(client)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize SAML keys: %w", err)
	}
	log.Printf("SAML signing keys loaded (cert subject: %s)", keys.Certificate.Subject.CommonName)

	mux := buildMux(client, keys, cfg)
	return mux, client, nil
}

// serve starts the HTTP(S) server and blocks until it returns an error.
func serve(cfg Config, mux *http.ServeMux) error {
	log.Printf("Gatekeeper starting on %s", cfg.ListenAddr)
	log.Printf("Entity ID: %s", cfg.EntityID)
	log.Printf("SSO URL: %s", cfg.SSOURL)
	log.Printf("OpenBao: %s", cfg.OpenBaoAddr)

	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		log.Printf("TLS enabled (cert: %s)", cfg.TLSCert)
		return http.ListenAndServeTLS(cfg.ListenAddr, cfg.TLSCert, cfg.TLSKey, mux)
	}
	log.Printf("WARNING: TLS disabled, running in plaintext mode")
	return http.ListenAndServe(cfg.ListenAddr, mux)
}

func main() {
	cfg := LoadConfig()
	if err := start(cfg); err != nil {
		log.Fatal(err)
	}
}

// start initializes the server and begins listening. Extracted for testability.
func start(cfg Config) error {
	mux, _, err := run(cfg)
	if err != nil {
		return err
	}
	return serve(cfg, mux)
}
