package saml

import (
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asymmetric-effort/convocate/src/saml-scim-agent/openbao"
)

func TestGenerateKeyPair_Ed25519(t *testing.T) {
	kp, err := generateKeyPair("ed25519")
	if err != nil {
		t.Fatalf("generateKeyPair(ed25519) failed: %v", err)
	}
	if _, ok := kp.PrivateKey.(ed25519.PrivateKey); !ok {
		t.Fatalf("expected ed25519.PrivateKey, got %T", kp.PrivateKey)
	}
	if kp.Algorithm != "ed25519" {
		t.Errorf("Algorithm = %q, want %q", kp.Algorithm, "ed25519")
	}
	if kp.Certificate == nil {
		t.Fatal("Certificate is nil")
	}
	if len(kp.CertPEM) == 0 {
		t.Fatal("CertPEM is empty")
	}

	// Verify certificate subject
	if kp.Certificate.Subject.CommonName != "SAML-SCIM-Agent SAML Signing" {
		t.Errorf("expected CN 'SAML-SCIM-Agent SAML Signing', got %s", kp.Certificate.Subject.CommonName)
	}
	if len(kp.Certificate.Subject.Organization) != 1 || kp.Certificate.Subject.Organization[0] != "Asymmetric Effort" {
		t.Errorf("unexpected organization: %v", kp.Certificate.Subject.Organization)
	}

	// Verify certificate public key matches
	certPub, ok := kp.Certificate.PublicKey.(ed25519.PublicKey)
	if !ok {
		t.Fatalf("certificate public key type = %T, want ed25519.PublicKey", kp.Certificate.PublicKey)
	}
	privKey := kp.PrivateKey.(ed25519.PrivateKey)
	if !certPub.Equal(privKey.Public()) {
		t.Error("certificate public key does not match private key")
	}
}

func TestGenerateKeyPair_RSA(t *testing.T) {
	kp, err := generateKeyPair("rsa")
	if err != nil {
		t.Fatalf("generateKeyPair(rsa) failed: %v", err)
	}
	if _, ok := kp.PrivateKey.(*rsa.PrivateKey); !ok {
		t.Fatalf("expected *rsa.PrivateKey, got %T", kp.PrivateKey)
	}
	if kp.Algorithm != "rsa" {
		t.Errorf("Algorithm = %q, want %q", kp.Algorithm, "rsa")
	}
	if kp.Certificate == nil {
		t.Fatal("Certificate is nil")
	}
	if len(kp.CertPEM) == 0 {
		t.Fatal("CertPEM is empty")
	}

	rsaKey := kp.PrivateKey.(*rsa.PrivateKey)
	if rsaKey.N.BitLen() != 2048 {
		t.Errorf("expected 2048 bit key, got %d", rsaKey.N.BitLen())
	}
}

func TestGenerateKeyPair_InvalidAlgorithm(t *testing.T) {
	_, err := generateKeyPair("dsa")
	if err == nil {
		t.Fatal("expected error for unsupported algorithm")
	}
}

func TestDecodeKeyPair_Ed25519_RoundTrip(t *testing.T) {
	kp, err := generateKeyPair("ed25519")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// Encode private key as PKCS8
	pkcs8, err := x509.MarshalPKCS8PrivateKey(kp.PrivateKey)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
	// Decode
	decoded, err := decodeKeyPair(keyPEM, kp.CertPEM)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := decoded.PrivateKey.(ed25519.PrivateKey); !ok {
		t.Fatalf("decoded key type = %T, want ed25519.PrivateKey", decoded.PrivateKey)
	}
	if decoded.Algorithm != "ed25519" {
		t.Errorf("decoded Algorithm = %q, want %q", decoded.Algorithm, "ed25519")
	}
	if decoded.Certificate == nil {
		t.Fatal("decoded certificate is nil")
	}
}

func TestDecodeKeyPair_RSA_RoundTrip(t *testing.T) {
	kp, err := generateKeyPair("rsa")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	rsaKey := kp.PrivateKey.(*rsa.PrivateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	})
	decoded, err := decodeKeyPair(keyPEM, kp.CertPEM)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := decoded.PrivateKey.(*rsa.PrivateKey); !ok {
		t.Fatalf("decoded key type = %T, want *rsa.PrivateKey", decoded.PrivateKey)
	}
	if decoded.Algorithm != "rsa" {
		t.Errorf("decoded Algorithm = %q, want %q", decoded.Algorithm, "rsa")
	}
}

func TestDecodeKeyPair_RSA_PKCS8_RoundTrip(t *testing.T) {
	kp, err := generateKeyPair("rsa")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// Encode as PKCS8 (PRIVATE KEY) instead of PKCS1 (RSA PRIVATE KEY)
	pkcs8, err := x509.MarshalPKCS8PrivateKey(kp.PrivateKey)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
	decoded, err := decodeKeyPair(keyPEM, kp.CertPEM)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := decoded.PrivateKey.(*rsa.PrivateKey); !ok {
		t.Fatalf("decoded key type = %T, want *rsa.PrivateKey", decoded.PrivateKey)
	}
	if decoded.Algorithm != "rsa" {
		t.Errorf("decoded Algorithm = %q, want %q", decoded.Algorithm, "rsa")
	}
}

func TestDecodeKeyPair_InvalidPEM(t *testing.T) {
	_, err := decodeKeyPair([]byte("not-pem"), []byte("not-pem"))
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestDecodeKeyPair_InvalidKeyBytes(t *testing.T) {
	kp, _ := generateKeyPair("rsa")
	badKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: []byte("not a real key"),
	})
	_, err := decodeKeyPair(badKeyPEM, kp.CertPEM)
	if err == nil {
		t.Fatal("expected error for invalid key bytes")
	}
}

func TestDecodeKeyPair_InvalidCertPEM(t *testing.T) {
	kp, _ := generateKeyPair("rsa")
	rsaKey := kp.PrivateKey.(*rsa.PrivateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	})
	_, err := decodeKeyPair(keyPEM, []byte("not a pem"))
	if err == nil {
		t.Fatal("expected error for invalid cert PEM")
	}
}

func TestDecodeKeyPair_InvalidCertBytes(t *testing.T) {
	kp, _ := generateKeyPair("rsa")
	rsaKey := kp.PrivateKey.(*rsa.PrivateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	})
	badCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: []byte("not a real cert"),
	})
	_, err := decodeKeyPair(keyPEM, badCertPEM)
	if err == nil {
		t.Fatal("expected error for invalid cert bytes")
	}
}

func TestLoadOrGenerateKeysNew_Ed25519(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	kp, err := LoadOrGenerateKeys(client, "ed25519")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kp == nil {
		t.Fatal("expected key pair")
	}
	if _, ok := kp.PrivateKey.(ed25519.PrivateKey); !ok {
		t.Fatalf("expected ed25519.PrivateKey, got %T", kp.PrivateKey)
	}
	if kp.Algorithm != "ed25519" {
		t.Errorf("Algorithm = %q, want %q", kp.Algorithm, "ed25519")
	}
}

func TestLoadOrGenerateKeysNew_RSA(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	kp, err := LoadOrGenerateKeys(client, "rsa")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kp == nil {
		t.Fatal("expected key pair")
	}
	if _, ok := kp.PrivateKey.(*rsa.PrivateKey); !ok {
		t.Fatalf("expected *rsa.PrivateKey, got %T", kp.PrivateKey)
	}
	if kp.Algorithm != "rsa" {
		t.Errorf("Algorithm = %q, want %q", kp.Algorithm, "rsa")
	}
}

func TestLoadOrGenerateKeysExisting_Ed25519(t *testing.T) {
	kp, _ := generateKeyPair("ed25519")
	pkcs8, err := x509.MarshalPKCS8PrivateKey(kp.PrivateKey)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"data": map[string]interface{}{
					"private_key": string(keyPEM),
					"certificate": string(kp.CertPEM),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	loaded, err := LoadOrGenerateKeys(client, "ed25519")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected key pair")
	}
	if _, ok := loaded.PrivateKey.(ed25519.PrivateKey); !ok {
		t.Fatalf("loaded key type = %T, want ed25519.PrivateKey", loaded.PrivateKey)
	}
	if loaded.Algorithm != "ed25519" {
		t.Errorf("loaded Algorithm = %q, want %q", loaded.Algorithm, "ed25519")
	}
}

func TestLoadOrGenerateKeysReadError(t *testing.T) {
	client := openbao.NewClient("http://127.0.0.1:1", "test-token", true)
	_, err := LoadOrGenerateKeys(client, "ed25519")
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
}

func TestLoadOrGenerateKeysWriteError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	_, err := LoadOrGenerateKeys(client, "ed25519")
	if err == nil {
		t.Fatal("expected error when write fails")
	}
}

func TestLoadOrGenerateKeysInvalidStoredKey(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"private_key": "invalid-pem",
						"certificate": "invalid-pem",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	kp, err := LoadOrGenerateKeys(client, "ed25519")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kp == nil {
		t.Fatal("expected newly generated key pair")
	}
}

func TestLoadOrGenerateKeysMissingFields(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"other_field": "something",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	kp, err := LoadOrGenerateKeys(client, "ed25519")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kp == nil {
		t.Fatal("expected newly generated key pair")
	}
}

func TestLoadOrGenerateKeysNonStringValues(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/saml-scim-agent/saml-signing-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"private_key": 12345,
						"certificate": true,
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := openbao.NewClient(ts.URL, "test-token", true)
	kp, err := LoadOrGenerateKeys(client, "ed25519")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kp == nil {
		t.Fatal("expected newly generated key pair")
	}
}
