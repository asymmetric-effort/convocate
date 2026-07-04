package k8s

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func TestComputeCertHash_InvalidPEM(t *testing.T) {
	// Non-PEM data should still produce a hash (fallback to raw bytes)
	result := computeCertHash([]byte("not-pem-data"))
	if result == "" {
		t.Fatal("expected non-empty hash")
	}
	if len(result) < 10 {
		t.Fatal("expected sha256: prefix and hex hash")
	}
	if result[:7] != "sha256:" {
		t.Fatalf("expected sha256: prefix, got %s", result[:7])
	}
}

func TestComputeCertHash_ValidPEM_InvalidCert(t *testing.T) {
	// Valid PEM structure but invalid certificate DER data
	pem := []byte("-----BEGIN CERTIFICATE-----\n" +
		base64.StdEncoding.EncodeToString([]byte("not-a-real-cert")) + "\n" +
		"-----END CERTIFICATE-----\n")
	result := computeCertHash(pem)
	if result == "" {
		t.Fatal("expected non-empty hash")
	}
	if result[:7] != "sha256:" {
		t.Fatalf("expected sha256: prefix, got %s", result[:7])
	}
}

func TestBase64Decode(t *testing.T) {
	input := base64.StdEncoding.EncodeToString([]byte("hello world"))
	result, err := base64Decode(input)
	if err != nil {
		t.Fatalf("base64Decode: %v", err)
	}
	if string(result) != "hello world" {
		t.Fatalf("expected 'hello world', got %s", string(result))
	}
}

func TestBase64Decode_Invalid(t *testing.T) {
	_, err := base64Decode("!!!not-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestBase64Decode_Empty(t *testing.T) {
	result, err := base64Decode("")
	if err != nil {
		t.Fatalf("base64Decode empty: %v", err)
	}
	if len(result) != 0 {
		t.Fatal("expected empty result for empty input")
	}
}

func TestComputeCertHash_ValidSelfSignedCert(t *testing.T) {
	// Generate a real self-signed X.509 certificate for testing
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	result := computeCertHash(certPEM)
	if result == "" {
		t.Fatal("expected non-empty hash")
	}
	if result[:7] != "sha256:" {
		t.Fatalf("expected sha256: prefix, got %s", result[:7])
	}
	// Hash should be deterministic
	result2 := computeCertHash(certPEM)
	if result != result2 {
		t.Fatal("hash should be deterministic")
	}
}
