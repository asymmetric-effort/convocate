package tlsutil

import (
	"crypto/x509"
	"strings"
	"testing"
)

func TestGenerateCA_RoundTrip(t *testing.T) {
	ca, err := GenerateCA("claude-shell test CA", 5)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	if !ca.Cert.IsCA {
		t.Error("cert is not marked as CA")
	}
	if ca.Cert.Subject.CommonName != "claude-shell test CA" {
		t.Errorf("CN = %q", ca.Cert.Subject.CommonName)
	}
	// Round-trip via ParseKeyMaterial.
	back, err := ParseKeyMaterial(ca.CertPEM, ca.KeyPEM)
	if err != nil {
		t.Fatalf("ParseKeyMaterial: %v", err)
	}
	if back.Cert.SerialNumber.Cmp(ca.Cert.SerialNumber) != 0 {
		t.Error("round-trip serial mismatch")
	}
}

func TestSignCert_ServerValidatesUnderCA(t *testing.T) {
	ca, err := GenerateCA("claude-shell test CA", 5)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := SignCert(ca, SignOptions{
		CommonName: "shell.example.com",
		DNSNames:   []string{"shell.example.com", "localhost"},
		IsServer:   true,
	})
	if err != nil {
		t.Fatalf("SignCert: %v", err)
	}

	// The leaf must verify under a pool containing just the CA.
	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	if _, err := leaf.Cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSName:   "shell.example.com",
	}); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// SAN wildcard works for the second name too.
	if _, err := leaf.Cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSName:   "localhost",
	}); err != nil {
		t.Errorf("verify localhost: %v", err)
	}
}

func TestSignCert_ClientValidates(t *testing.T) {
	ca, err := GenerateCA("ca", 3)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := SignCert(ca, SignOptions{CommonName: "agent-abc123"})
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	// Client certs use ExtKeyUsageClientAuth.
	if _, err := leaf.Cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if leaf.Cert.Subject.CommonName != "agent-abc123" {
		t.Errorf("CN = %q", leaf.Cert.Subject.CommonName)
	}
}

func TestSignCert_ServerCertFailsAsClient(t *testing.T) {
	ca, _ := GenerateCA("ca", 3)
	leaf, _ := SignCert(ca, SignOptions{CommonName: "s", IsServer: true})
	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	if _, err := leaf.Cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err == nil {
		t.Error("server cert should not validate as client auth")
	}
}

func TestGenerateCA_Errors(t *testing.T) {
	if _, err := GenerateCA("", 1); err == nil {
		t.Error("expected error with empty CN")
	}
}

func TestSignCert_Errors(t *testing.T) {
	if _, err := SignCert(nil, SignOptions{CommonName: "x"}); err == nil {
		t.Error("expected error with nil CA")
	}
	ca, _ := GenerateCA("ca", 1)
	if _, err := SignCert(ca, SignOptions{CommonName: ""}); err == nil {
		t.Error("expected error with empty CN")
	}
}

func TestParseKeyMaterial_Errors(t *testing.T) {
	if _, err := ParseKeyMaterial([]byte("not pem"), []byte("also not")); err == nil {
		t.Error("expected parse error")
	}
	ca, _ := GenerateCA("c", 1)
	// Mix: valid cert PEM but garbage key PEM.
	if _, err := ParseKeyMaterial(ca.CertPEM, []byte("-----BEGIN X-----\nbad\n-----END X-----\n")); err == nil ||
		!strings.Contains(err.Error(), "parse key") && !strings.Contains(err.Error(), "no PEM block") {
		// Error is acceptable either way; just ensure we fail somehow.
		if err == nil {
			t.Error("expected error")
		}
	}
}
