package mtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"
	"time"
)

func testCA(t *testing.T) *CA {
	t.Helper()
	ca, err := GenerateCA("convocate-test-ca", 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA error: %v", err)
	}
	return ca
}

func parseCert(t *testing.T, certPEM []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert
}

func TestIssueServerCert(t *testing.T) {
	ca := testCA(t)
	pair, err := ca.IssueServerCert(
		"router.convocate.local",
		[]string{"router.convocate.local", "localhost"},
		[]net.IP{net.ParseIP("127.0.0.1")},
		24*time.Hour,
	)
	if err != nil {
		t.Fatalf("IssueServerCert error: %v", err)
	}

	cert := parseCert(t, pair.CertPEM)

	t.Run("not a CA", func(t *testing.T) {
		if cert.IsCA {
			t.Error("server cert should not be a CA")
		}
	})

	t.Run("common name", func(t *testing.T) {
		if cert.Subject.CommonName != "router.convocate.local" {
			t.Errorf("CommonName: got %q, want %q", cert.Subject.CommonName, "router.convocate.local")
		}
	})

	t.Run("DNS SANs", func(t *testing.T) {
		if len(cert.DNSNames) != 2 {
			t.Fatalf("DNSNames: got %d, want 2", len(cert.DNSNames))
		}
		if cert.DNSNames[0] != "router.convocate.local" {
			t.Errorf("DNSNames[0]: got %q", cert.DNSNames[0])
		}
		if cert.DNSNames[1] != "localhost" {
			t.Errorf("DNSNames[1]: got %q", cert.DNSNames[1])
		}
	})

	t.Run("IP SANs", func(t *testing.T) {
		if len(cert.IPAddresses) != 1 {
			t.Fatalf("IPAddresses: got %d, want 1", len(cert.IPAddresses))
		}
		if !cert.IPAddresses[0].Equal(net.ParseIP("127.0.0.1")) {
			t.Errorf("IPAddresses[0]: got %v", cert.IPAddresses[0])
		}
	})

	t.Run("server auth EKU", func(t *testing.T) {
		hasServerAuth := false
		for _, eku := range cert.ExtKeyUsage {
			if eku == x509.ExtKeyUsageServerAuth {
				hasServerAuth = true
			}
		}
		if !hasServerAuth {
			t.Error("missing ExtKeyUsageServerAuth")
		}
	})

	t.Run("ECC P-256", func(t *testing.T) {
		pubKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
		if !ok {
			t.Fatalf("expected *ecdsa.PublicKey, got %T", cert.PublicKey)
		}
		if pubKey.Curve != elliptic.P256() {
			t.Errorf("curve: got %v, want P-256", pubKey.Curve.Params().Name)
		}
	})

	t.Run("signed by CA", func(t *testing.T) {
		err := cert.CheckSignatureFrom(ca.Certificate)
		if err != nil {
			t.Errorf("not signed by CA: %v", err)
		}
	})

	t.Run("verify chain", func(t *testing.T) {
		roots := x509.NewCertPool()
		roots.AddCert(ca.Certificate)
		_, err := cert.Verify(x509.VerifyOptions{
			Roots:     roots,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		})
		if err != nil {
			t.Errorf("verify chain: %v", err)
		}
	})
}

func TestIssueClientCert(t *testing.T) {
	ca := testCA(t)
	pair, err := ca.IssueClientCert("agent-host-1", 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueClientCert error: %v", err)
	}

	cert := parseCert(t, pair.CertPEM)

	t.Run("not a CA", func(t *testing.T) {
		if cert.IsCA {
			t.Error("client cert should not be a CA")
		}
	})

	t.Run("common name is host ID", func(t *testing.T) {
		if cert.Subject.CommonName != "agent-host-1" {
			t.Errorf("CommonName: got %q, want %q", cert.Subject.CommonName, "agent-host-1")
		}
	})

	t.Run("client auth EKU only", func(t *testing.T) {
		if len(cert.ExtKeyUsage) != 1 {
			t.Fatalf("ExtKeyUsage: got %d entries, want 1", len(cert.ExtKeyUsage))
		}
		if cert.ExtKeyUsage[0] != x509.ExtKeyUsageClientAuth {
			t.Error("missing ExtKeyUsageClientAuth")
		}
	})

	t.Run("no DNS SANs", func(t *testing.T) {
		if len(cert.DNSNames) != 0 {
			t.Errorf("DNSNames: got %v, want none", cert.DNSNames)
		}
	})

	t.Run("signed by CA", func(t *testing.T) {
		err := cert.CheckSignatureFrom(ca.Certificate)
		if err != nil {
			t.Errorf("not signed by CA: %v", err)
		}
	})

	t.Run("verify chain", func(t *testing.T) {
		roots := x509.NewCertPool()
		roots.AddCert(ca.Certificate)
		_, err := cert.Verify(x509.VerifyOptions{
			Roots:     roots,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		})
		if err != nil {
			t.Errorf("verify chain: %v", err)
		}
	})
}

func TestIssueCombinedCert(t *testing.T) {
	ca := testCA(t)
	pair, err := ca.IssueCombinedCert(
		"router.convocate.local",
		[]string{"router.convocate.local"},
		[]net.IP{net.ParseIP("127.0.0.1")},
		24*time.Hour,
	)
	if err != nil {
		t.Fatalf("IssueCombinedCert error: %v", err)
	}

	cert := parseCert(t, pair.CertPEM)

	t.Run("has both server and client auth EKU", func(t *testing.T) {
		hasServer := false
		hasClient := false
		for _, eku := range cert.ExtKeyUsage {
			if eku == x509.ExtKeyUsageServerAuth {
				hasServer = true
			}
			if eku == x509.ExtKeyUsageClientAuth {
				hasClient = true
			}
		}
		if !hasServer {
			t.Error("missing ExtKeyUsageServerAuth")
		}
		if !hasClient {
			t.Error("missing ExtKeyUsageClientAuth")
		}
	})

	t.Run("verify as server", func(t *testing.T) {
		roots := x509.NewCertPool()
		roots.AddCert(ca.Certificate)
		_, err := cert.Verify(x509.VerifyOptions{
			Roots:     roots,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		})
		if err != nil {
			t.Errorf("verify as server: %v", err)
		}
	})

	t.Run("verify as client", func(t *testing.T) {
		roots := x509.NewCertPool()
		roots.AddCert(ca.Certificate)
		_, err := cert.Verify(x509.VerifyOptions{
			Roots:     roots,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		})
		if err != nil {
			t.Errorf("verify as client: %v", err)
		}
	})
}

func TestCertUniqueness(t *testing.T) {
	ca := testCA(t)
	pair1, err := ca.IssueClientCert("host-1", 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueClientCert error: %v", err)
	}
	pair2, err := ca.IssueClientCert("host-2", 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueClientCert error: %v", err)
	}

	cert1 := parseCert(t, pair1.CertPEM)
	cert2 := parseCert(t, pair2.CertPEM)

	if cert1.SerialNumber.Cmp(cert2.SerialNumber) == 0 {
		t.Error("two certificates have the same serial number")
	}
}

func TestCrossCAVerificationFails(t *testing.T) {
	ca1 := testCA(t)
	ca2 := testCA(t)

	pair, err := ca1.IssueClientCert("host-1", 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueClientCert error: %v", err)
	}
	cert := parseCert(t, pair.CertPEM)

	// Verify with the wrong CA should fail.
	roots := x509.NewCertPool()
	roots.AddCert(ca2.Certificate)
	_, err = cert.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	if err == nil {
		t.Error("cross-CA verification should fail, got nil")
	}
}

func TestKeyPairPEMValid(t *testing.T) {
	ca := testCA(t)
	pair, err := ca.IssueClientCert("host-1", 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueClientCert error: %v", err)
	}

	certBlock, _ := pem.Decode(pair.CertPEM)
	if certBlock == nil || certBlock.Type != pemCertificate {
		t.Error("CertPEM is not a valid CERTIFICATE PEM block")
	}

	keyBlock, _ := pem.Decode(pair.KeyPEM)
	if keyBlock == nil || keyBlock.Type != pemECKey {
		t.Error("KeyPEM is not a valid EC PRIVATE KEY PEM block")
	}
}
